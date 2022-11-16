// Package prw provides PluggableResponseWriter, which
// is a ResponseWriter and Hijacker (for websockets) that provides reusability and
// resiliency, optimized for handler chains where multiple middlewares
// may want to modify the response. It also can Marshal/Unmarshal the core response parts
// (body, status, headers) for use with caching operations.
package prw

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"net"
	"net/http"
	"sync"

	"go.uber.org/atomic"
)

var (
	// We create a pool of bytes.Buffer to optimize memory CRUD
	bodyPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

// PluggableResponseWriter is a ResponseWriter that provides
// reusability and resiliency, optimized for handler chains where multiple
// middlewares may want to modify the response
type PluggableResponseWriter struct {
	Body       *bytes.Buffer
	status     int
	headers    http.Header
	orig       http.ResponseWriter
	flushFunc  func(http.ResponseWriter, *PluggableResponseWriter)
	flush      atomic.Bool
	rmHeaders  []string
	addHeaders map[string]string
	hijacked   bool
	closeLock  sync.Mutex
}

// simpleResponse is a struct to assist with encoding/decoding the minimum needed to
// preserve a response for caching
type simpleResponse struct {
	Body    []byte
	Status  int
	Headers http.Header
}

// toSimpleResponse returns a simplified representation of the PRW as a simpleResponse
func (w *PluggableResponseWriter) toSimpleResponse() *simpleResponse {
	return &simpleResponse{
		Body:    w.Body.Bytes(),
		Status:  w.status,
		Headers: w.headers,
	}
}

// fromSimpleResponse replaces parts of the PRW with the values from the simpleResponse
func (w *PluggableResponseWriter) fromSimpleResponse(s *simpleResponse) {
	w.closeLock.Lock()
	defer w.closeLock.Unlock()

	// We need to recycle the existing body before replacing it. PRW.Close() will
	// recycle the new one eventually.
	b := bodyPool.Get().(*bytes.Buffer)
	b.Reset()
	b.Write(s.Body)
	bodyPool.Put(w.Body)

	w.Body = b
	w.status = s.Status
	w.headers = s.Headers
}

// NewPluggableResponseWriterIfNot returns a pointer to an initialized PluggableResponseWriter and true,
// if the provided ResponseWriter is not a PluggableResponseWriter, otherwise returns the provided
// ResponseWriter casted as a PluggableResponseWriter and false. This makes simple create-and-clean stanzas
// trivial.
//
// Where "w" is the original ResponseWriter passed
// rw, firstRw := NewPluggableResponseWriterIfNot(w)
// defer rw.FlushToIf(w, firstRw)
func NewPluggableResponseWriterIfNot(rw http.ResponseWriter) (*PluggableResponseWriter, bool) {
	switch rw := rw.(type) {
	case *PluggableResponseWriter:
		// is not first prw, reuse!
		return rw, false
	default:
		// is first prw, create!
		w := NewPluggableResponseWriter()
		w.orig = rw
		return w, true
	}
}

// NewPluggableResponseWriterFromOld returns a pointer to an initialized PluggableResponseWriter, with the original
// stored away for Flush()
func NewPluggableResponseWriterFromOld(rw http.ResponseWriter) *PluggableResponseWriter {
	w := NewPluggableResponseWriter()
	w.orig = rw
	return w
}

// NewPluggableResponseWriter returns a pointer to an initialized PluggableResponseWriter
func NewPluggableResponseWriter() *PluggableResponseWriter {
	w := PluggableResponseWriter{}
	// Empty body, get a buffer
	w.Body = bodyPool.Get().(*bytes.Buffer)
	w.Body.Reset() // we don't trust it's clean
	w.headers = make(map[string][]string)
	w.rmHeaders = make([]string, 0)
	w.addHeaders = make(map[string]string)
	return &w
}

// SetHeadersToRemove sets a list of headers to remove before flushing/writing headers to the response
func (w *PluggableResponseWriter) SetHeadersToRemove(headers []string) {
	w.rmHeaders = headers
}

// SetHeadersToAdd sets a map of headers to add before flushing/writing headers to the response
func (w *PluggableResponseWriter) SetHeadersToAdd(headers map[string]string) {
	w.addHeaders = headers
}

// AddFlushFunc adds a function to run if any of the Flush methods are called, to customize that activity
func (w *PluggableResponseWriter) AddFlushFunc(f func(http.ResponseWriter, *PluggableResponseWriter)) {
	w.flushFunc = f
}

// Length returns the byte length of the response body
func (w *PluggableResponseWriter) Length() int {
	return w.Body.Len()
}

// Code returns the HTTP status code
func (w *PluggableResponseWriter) Code() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}

// Header returns the current http.Header
func (w *PluggableResponseWriter) Header() http.Header {
	return w.headers
}

// SetHeader takes an http.Header to replace the current with
func (w *PluggableResponseWriter) SetHeader(h http.Header) {
	w.headers = h
}

// WriteHeader sends an HTTP response header with the provided
// status code.
func (w *PluggableResponseWriter) WriteHeader(status int) {
	w.status = status
}

// Write writes the data to the connection as part of an HTTP reply.
// Additionally, it sets the status if that hasn't been set yet,
// and determines the Content-Type if that hasn't been determined yet.
func (w *PluggableResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		// If Write before WriteHeader,
		// set the status to OK
		w.status = 200
	}

	wlen, err := w.Body.Write(b)
	if err != nil {
		return 0, err
	}

	if ct := w.Header().Get("Content-Type"); ct == "" {
		// Content-Type hasn't been set, so let's set it.
		w.Header().Set("Content-Type", http.DetectContentType(b))
	}

	if w.flush.Load() {
		w.orig.Write(b)
	}

	return wlen, err
}

// Close should only be called if the PluggableResponseWriter will no longer be used.
func (w *PluggableResponseWriter) Close() {
	w.closeLock.Lock()
	defer w.closeLock.Unlock()

	if w.Body != nil {
		bodyPool.Put(w.Body)
		w.Body = nil
	}
}

// FlushToIf takes a ResponseWriter and boolean, and calls FlushTo if the boolean is true.
// The PluggableResponseWriter should not be used after calling FlushToIf.
// This makes simple create-and-clean stanzas trivial.
//
// Where "w" is the original ResponseWriter passed
// rw, firstRw := NewPluggableResponseWriterIfNot(w)
// defer rw.FlushToIf(w, firstRw)
func (w *PluggableResponseWriter) FlushToIf(to http.ResponseWriter, first bool) (int, error) {
	var (
		l   int
		err error
	)

	if first {
		l, err = w.FlushTo(to)
		w.Close()
	}

	return l, err
}

// FlushTo writes to the provided ResponseWriter with our headers, status code, and body.
// The PluggableResponseWriter should not be used after calling FlushToIf.
func (w *PluggableResponseWriter) FlushTo(to http.ResponseWriter) (int, error) {
	if w.flushFunc != nil {
		w.flushFunc(to, w)
		return 0, nil
	}

	w.syncHeaders(w.Header())
	for k, v := range w.Header() {
		to.Header()[k] = v
	}

	to.WriteHeader(w.Code())
	s, err := to.Write(w.Body.Bytes())

	if flusher, ok := to.(http.Flusher); ok {
		// to is a Flusher, so Flush
		flusher.Flush()
	}

	return s, err
}

// Flush satisfies http.Flusher. If NewPluggableResponseWriterFromOld or NewPluggableResponseWriterIfNot is used,
// then the first time Flush() is called, if the original ResponseWriter is an http.Flusher, all headers and the
// body thus far are written to it, and then Flush() is called on it too. **ALSO** further Write() calls are also
// written to the original. Subsequent calls to Flush will call Flush() on the original.
func (w *PluggableResponseWriter) Flush() {
	if w.orig == nil {
		// We have no orig, don't bother
		return
	}

	if w.hijacked {
		// We've been hijacked. Noop the flush
		return
	}

	if w.flushFunc != nil {
		// We have a custom flushFunc set
		w.flushFunc(w.orig, w)
	} else if f, ok := w.orig.(http.Flusher); ok {
		// orig is a Flusher
		defer f.Flush()

		// We have an atomic Swap happening here, ensuring there is no race
		if !w.flush.Swap(true) {
			w.syncHeaders(w.Header())
			for k, v := range w.Header() {
				w.orig.Header()[k] = v
			}

			w.orig.WriteHeader(w.Code())
			w.orig.Write(w.Body.Bytes())
		}

	}
}

// Hijack implements http.Hijacker
func (w *PluggableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.orig.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("original ResponseWriter is not a Hijacker")
	}
	w.hijacked = true
	return hj.Hijack()
}

// MarshalBinary is used by encoding/gob to create a representation for encoding.
func (w *PluggableResponseWriter) MarshalBinary() ([]byte, error) {
	// we don't use the bodyPool here because we have to return the
	// .Bytes and that creates a defer race
	var b bytes.Buffer
	s := w.toSimpleResponse()
	enc := gob.NewEncoder(&b)
	err := enc.Encode(s)
	if err != nil {
		return []byte{}, err
	}
	return b.Bytes(), nil
}

// UnmarshalBinary is used by encoding/gob to reconstitute a previously-encoded instance.
func (w *PluggableResponseWriter) UnmarshalBinary(data []byte) error {
	var (
		s simpleResponse
		b = bodyPool.Get().(*bytes.Buffer)
	)
	b.Reset()
	defer bodyPool.Put(b)
	if _, err := b.Write(data); err != nil {
		return err
	}

	dec := gob.NewDecoder(b)
	err := dec.Decode(&s)
	if err != nil {
		return err
	}
	w.fromSimpleResponse(&s)
	return nil
}

// syncHeaders is a helper to call trimHeaders and setHeaders
func (w *PluggableResponseWriter) syncHeaders(from http.Header) {
	w.trimHeaders(from)
	w.setHeaders(from)
}

// trimHeaders is used to remove headers listed in SetHeadersToRemove()
func (w *PluggableResponseWriter) trimHeaders(from http.Header) {
	for _, header := range w.rmHeaders {
		from.Del(header)
	}
}

// setHeaders is used to set headers listed in SetHeadersToAdd()
func (w *PluggableResponseWriter) setHeaders(from http.Header) {
	for k, v := range w.addHeaders {
		from.Set(k, v)
	}
}

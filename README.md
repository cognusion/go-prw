

# prw
`import "github.com/cognusion/go-prw"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package prw provides PluggableResponseWriter, which
is a ResponseWriter and Hijacker (for websockets) that provides reusability and
resiliency, optimized for handler chains where multiple middlewares
may want to modify the response. It also can Marshal/Unmarshal the core response parts
(body, status, headers) for use with caching operations.




## <a name="pkg-index">Index</a>
* [type PluggableResponseWriter](#PluggableResponseWriter)
  * [func NewPluggableResponseWriter() *PluggableResponseWriter](#NewPluggableResponseWriter)
  * [func NewPluggableResponseWriterFromOld(rw http.ResponseWriter) *PluggableResponseWriter](#NewPluggableResponseWriterFromOld)
  * [func NewPluggableResponseWriterIfNot(rw http.ResponseWriter) (*PluggableResponseWriter, bool)](#NewPluggableResponseWriterIfNot)
  * [func (w *PluggableResponseWriter) AddFlushFunc(f func(http.ResponseWriter, *PluggableResponseWriter))](#PluggableResponseWriter.AddFlushFunc)
  * [func (w *PluggableResponseWriter) Close()](#PluggableResponseWriter.Close)
  * [func (w *PluggableResponseWriter) Code() int](#PluggableResponseWriter.Code)
  * [func (w *PluggableResponseWriter) Flush()](#PluggableResponseWriter.Flush)
  * [func (w *PluggableResponseWriter) FlushTo(to http.ResponseWriter) (int, error)](#PluggableResponseWriter.FlushTo)
  * [func (w *PluggableResponseWriter) FlushToIf(to http.ResponseWriter, first bool) (int, error)](#PluggableResponseWriter.FlushToIf)
  * [func (w *PluggableResponseWriter) Header() http.Header](#PluggableResponseWriter.Header)
  * [func (w *PluggableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error)](#PluggableResponseWriter.Hijack)
  * [func (w *PluggableResponseWriter) Length() int](#PluggableResponseWriter.Length)
  * [func (w *PluggableResponseWriter) MarshalBinary() ([]byte, error)](#PluggableResponseWriter.MarshalBinary)
  * [func (w *PluggableResponseWriter) SetHeader(h http.Header)](#PluggableResponseWriter.SetHeader)
  * [func (w *PluggableResponseWriter) SetHeadersToAdd(headers map[string]string)](#PluggableResponseWriter.SetHeadersToAdd)
  * [func (w *PluggableResponseWriter) SetHeadersToRemove(headers []string)](#PluggableResponseWriter.SetHeadersToRemove)
  * [func (w *PluggableResponseWriter) UnmarshalBinary(data []byte) error](#PluggableResponseWriter.UnmarshalBinary)
  * [func (w *PluggableResponseWriter) Write(b []byte) (int, error)](#PluggableResponseWriter.Write)
  * [func (w *PluggableResponseWriter) WriteHeader(status int)](#PluggableResponseWriter.WriteHeader)


#### <a name="pkg-files">Package files</a>
[prw.go](https://github.com/cognusion/go-prw/tree/master/prw.go)






## <a name="PluggableResponseWriter">type</a> [PluggableResponseWriter](https://github.com/cognusion/go-prw/tree/master/prw.go?s=818:1134#L32)
``` go
type PluggableResponseWriter struct {
    Body *bytes.Buffer
    // contains filtered or unexported fields
}

```
PluggableResponseWriter is a ResponseWriter that provides
reusability and resiliency, optimized for handler chains where multiple
middlewares may want to modify the response







### <a name="NewPluggableResponseWriter">func</a> [NewPluggableResponseWriter](https://github.com/cognusion/go-prw/tree/master/prw.go?s=3232:3290#L109)
``` go
func NewPluggableResponseWriter() *PluggableResponseWriter
```
NewPluggableResponseWriter returns a pointer to an initialized PluggableResponseWriter


### <a name="NewPluggableResponseWriterFromOld">func</a> [NewPluggableResponseWriterFromOld](https://github.com/cognusion/go-prw/tree/master/prw.go?s=2991:3078#L102)
``` go
func NewPluggableResponseWriterFromOld(rw http.ResponseWriter) *PluggableResponseWriter
```
NewPluggableResponseWriterFromOld returns a pointer to an initialized PluggableResponseWriter, with the original
stored away for Flush()


### <a name="NewPluggableResponseWriterIfNot">func</a> [NewPluggableResponseWriterIfNot](https://github.com/cognusion/go-prw/tree/master/prw.go?s=2535:2628#L87)
``` go
func NewPluggableResponseWriterIfNot(rw http.ResponseWriter) (*PluggableResponseWriter, bool)
```
NewPluggableResponseWriterIfNot returns a pointer to an initialized PluggableResponseWriter and true,
if the provided ResponseWriter is not a PluggableResponseWriter, otherwise returns the provided
ResponseWriter casted as a PluggableResponseWriter and false. This makes simple create-and-clean stanzas
trivial.

Where "w" is the original ResponseWriter passed
rw, firstRw := NewPluggableResponseWriterIfNot(w)
defer rw.FlushToIf(w, firstRw)





### <a name="PluggableResponseWriter.AddFlushFunc">func</a> (\*PluggableResponseWriter) [AddFlushFunc](https://github.com/cognusion/go-prw/tree/master/prw.go?s=4076:4177#L131)
``` go
func (w *PluggableResponseWriter) AddFlushFunc(f func(http.ResponseWriter, *PluggableResponseWriter))
```
AddFlushFunc adds a function to run if any of the Flush methods are called, to customize that activity




### <a name="PluggableResponseWriter.Close">func</a> (\*PluggableResponseWriter) [Close](https://github.com/cognusion/go-prw/tree/master/prw.go?s=5647:5688#L192)
``` go
func (w *PluggableResponseWriter) Close()
```
Close should only be called if the PluggableResponseWriter will no longer be used.




### <a name="PluggableResponseWriter.Code">func</a> (\*PluggableResponseWriter) [Code](https://github.com/cognusion/go-prw/tree/master/prw.go?s=4365:4409#L141)
``` go
func (w *PluggableResponseWriter) Code() int
```
Code returns the HTTP status code




### <a name="PluggableResponseWriter.Flush">func</a> (\*PluggableResponseWriter) [Flush](https://github.com/cognusion/go-prw/tree/master/prw.go?s=7368:7409#L251)
``` go
func (w *PluggableResponseWriter) Flush()
```
Flush satisfies http.Flusher. If NewPluggableResponseWriterFromOld or NewPluggableResponseWriterIfNot is used,
then the first time Flush() is called, if the original ResponseWriter is an http.Flusher, all headers and the
body thus far are written to it, and then Flush() is called on it too. **ALSO** further Write() calls are also
written to the original. Subsequent calls to Flush will call Flush() on the original.




### <a name="PluggableResponseWriter.FlushTo">func</a> (\*PluggableResponseWriter) [FlushTo](https://github.com/cognusion/go-prw/tree/master/prw.go?s=6530:6608#L225)
``` go
func (w *PluggableResponseWriter) FlushTo(to http.ResponseWriter) (int, error)
```
FlushTo writes to the provided ResponseWriter with our headers, status code, and body.
The PluggableResponseWriter should not be used after calling FlushToIf.




### <a name="PluggableResponseWriter.FlushToIf">func</a> (\*PluggableResponseWriter) [FlushToIf](https://github.com/cognusion/go-prw/tree/master/prw.go?s=6166:6258#L209)
``` go
func (w *PluggableResponseWriter) FlushToIf(to http.ResponseWriter, first bool) (int, error)
```
FlushToIf takes a ResponseWriter and boolean, and calls FlushTo if the boolean is true.
The PluggableResponseWriter should not be used after calling FlushToIf.
This makes simple create-and-clean stanzas trivial.

Where "w" is the original ResponseWriter passed
rw, firstRw := NewPluggableResponseWriterIfNot(w)
defer rw.FlushToIf(w, firstRw)




### <a name="PluggableResponseWriter.Header">func</a> (\*PluggableResponseWriter) [Header](https://github.com/cognusion/go-prw/tree/master/prw.go?s=4510:4564#L149)
``` go
func (w *PluggableResponseWriter) Header() http.Header
```
Header returns the current http.Header




### <a name="PluggableResponseWriter.Hijack">func</a> (\*PluggableResponseWriter) [Hijack](https://github.com/cognusion/go-prw/tree/master/prw.go?s=8030:8109#L284)
``` go
func (w *PluggableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error)
```
Hijack implements http.Hijacker




### <a name="PluggableResponseWriter.Length">func</a> (\*PluggableResponseWriter) [Length](https://github.com/cognusion/go-prw/tree/master/prw.go?s=4255:4301#L136)
``` go
func (w *PluggableResponseWriter) Length() int
```
Length returns the byte length of the response body




### <a name="PluggableResponseWriter.MarshalBinary">func</a> (\*PluggableResponseWriter) [MarshalBinary](https://github.com/cognusion/go-prw/tree/master/prw.go?s=8358:8423#L294)
``` go
func (w *PluggableResponseWriter) MarshalBinary() ([]byte, error)
```
MarshalBinary is used by encoding/gob to create a representation for encoding.




### <a name="PluggableResponseWriter.SetHeader">func</a> (\*PluggableResponseWriter) [SetHeader](https://github.com/cognusion/go-prw/tree/master/prw.go?s=4650:4708#L154)
``` go
func (w *PluggableResponseWriter) SetHeader(h http.Header)
```
SetHeader takes an http.Header to replace the current with




### <a name="PluggableResponseWriter.SetHeadersToAdd">func</a> (\*PluggableResponseWriter) [SetHeadersToAdd](https://github.com/cognusion/go-prw/tree/master/prw.go?s=3864:3940#L126)
``` go
func (w *PluggableResponseWriter) SetHeadersToAdd(headers map[string]string)
```
SetHeadersToAdd sets a map of headers to add before flushing/writing headers to the response




### <a name="PluggableResponseWriter.SetHeadersToRemove">func</a> (\*PluggableResponseWriter) [SetHeadersToRemove](https://github.com/cognusion/go-prw/tree/master/prw.go?s=3669:3739#L121)
``` go
func (w *PluggableResponseWriter) SetHeadersToRemove(headers []string)
```
SetHeadersToRemove sets a list of headers to remove before flushing/writing headers to the response




### <a name="PluggableResponseWriter.UnmarshalBinary">func</a> (\*PluggableResponseWriter) [UnmarshalBinary](https://github.com/cognusion/go-prw/tree/master/prw.go?s=8787:8855#L308)
``` go
func (w *PluggableResponseWriter) UnmarshalBinary(data []byte) error
```
UnmarshalBinary is used by encoding/gob to reconstitute a previously-encoded instance.




### <a name="PluggableResponseWriter.Write">func</a> (\*PluggableResponseWriter) [Write](https://github.com/cognusion/go-prw/tree/master/prw.go?s=5095:5157#L167)
``` go
func (w *PluggableResponseWriter) Write(b []byte) (int, error)
```
Write writes the data to the connection as part of an HTTP reply.
Additionally, it sets the status if that hasn't been set yet,
and determines the Content-Type if that hasn't been determined yet.




### <a name="PluggableResponseWriter.WriteHeader">func</a> (\*PluggableResponseWriter) [WriteHeader](https://github.com/cognusion/go-prw/tree/master/prw.go?s=4808:4865#L160)
``` go
func (w *PluggableResponseWriter) WriteHeader(status int)
```
WriteHeader sends an HTTP response header with the provided
status code.








- - -
Generated by [godoc2md](http://godoc.org/github.com/cognusion/godoc2md)

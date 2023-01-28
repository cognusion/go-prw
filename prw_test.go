package prw

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/atomic"
)

func Test_NewPRW(t *testing.T) {

	Convey("When we create a PRW, it is correct", t, func() {

		p := NewPluggableResponseWriter()
		defer p.Close()
		So(p, ShouldNotBeNil)
		So(p.Length(), ShouldEqual, 0)
		So(p.Code(), ShouldEqual, http.StatusOK)

		Convey("... and it is an http.ResponseWriter", func() {
			a := func(x http.ResponseWriter) bool { return true }
			So(a(p), ShouldBeTrue)
		})

		Convey("... and when we pass a PRW to NewPluggableResponseWriterIfNot it returns the existing PRW", func() {
			n, created := NewPluggableResponseWriterIfNot(p)
			So(created, ShouldBeFalse)
			So(n, ShouldPointTo, p)
		})

		Convey("... and when we pass a PRW to NewPluggableResponseWriterFromOld it returns a new PRW with .orig sets to the old PRW", func() {
			n := NewPluggableResponseWriterFromOld(p)
			defer n.Close()
			So(n, ShouldNotPointTo, p)
			So(n.orig, ShouldPointTo, p)
		})
	})
}

func Test_WriteHeader(t *testing.T) {

	Convey("Writing headers works as expected", t, func() {
		p := NewPluggableResponseWriter()
		defer p.Close()

		p.WriteHeader(http.StatusMultipleChoices)
		So(p.Code(), ShouldEqual, http.StatusMultipleChoices)
	})
}

func Test_Write(t *testing.T) {

	Convey("Writing to the body works as expected", t, func() {
		p := NewPluggableResponseWriter()
		defer p.Close()

		n, err := p.Write([]byte("hola"))
		So(err, ShouldBeNil)
		So(n, ShouldEqual, 4)
		So(p.Length(), ShouldEqual, 4)
		So(p.Code(), ShouldEqual, http.StatusOK)
		So(p.Body.String(), ShouldEqual, "hola")

		n, err = p.Write([]byte(" adios"))
		So(err, ShouldBeNil)
		So(n, ShouldEqual, 6)
		So(p.Length(), ShouldEqual, 10)
		So(p.Code(), ShouldEqual, http.StatusOK)
		So(p.Body.String(), ShouldEqual, "hola adios")
	})
}

func Test_SimpleResponse(t *testing.T) {
	p := NewPluggableResponseWriter()
	defer p.Close()

	Convey("Writing works are expected", t, func() {
		p.WriteHeader(http.StatusOK)
		So(p.Code(), ShouldEqual, http.StatusOK)

		// Testing roll-up
		n, err := p.Write([]byte("hola adios"))
		So(err, ShouldBeNil)
		So(n, ShouldEqual, 10)
		So(p.Length(), ShouldEqual, 10)
		So(p.Code(), ShouldEqual, http.StatusOK)
		So(p.Body.String(), ShouldEqual, "hola adios")

		// Test the SimpleResponse TOREMOVE
		s := p.toSimpleResponse()
		So(s.Headers, ShouldResemble, p.headers)
		So(s.Status, ShouldEqual, p.status)
		So(s.Body, ShouldResemble, p.Body.Bytes())

		// Test marshalling
		Convey("Marshalling and unmarshalling work as expected", func() {
			mp, err := p.MarshalBinary()
			So(err, ShouldBeNil)
			So(mp, ShouldNotBeEmpty)

			// Make some changes
			n, err := p.Write([]byte(" OMG THIS SHOULDN'T BE HERE"))
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 27)
			p.WriteHeader(http.StatusForbidden)
			So(p.Code(), ShouldEqual, http.StatusForbidden)
			So(p.Body.String(), ShouldEqual, "hola adios OMG THIS SHOULDN'T BE HERE")

			err = p.UnmarshalBinary(mp)
			So(err, ShouldBeNil)
			So(p.Length(), ShouldEqual, 10)
			So(p.Code(), ShouldEqual, http.StatusOK)
			So(p.Body.String(), ShouldEqual, "hola adios")
		})

	})
}

func Test_Flush(t *testing.T) {
	Convey("When a test server writes stuff and FlushToIf is called, it works as expected", t, func(c C) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, isNew := NewPluggableResponseWriterIfNot(w)
			c.So(isNew, ShouldBeTrue)
			defer p.FlushToIf(w, isNew)
			p.WriteHeader(http.StatusInternalServerError)
			p.Write([]byte("Oh this is bad"))

		}))
		defer testServer.Close()

		// should return 500
		resp, err := http.Get(testServer.URL)
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
		b := bodyPool.Get()
		defer b.Close()
		b.ResetFromReader(resp.Body)
		So(b.String(), ShouldEqual, "Oh this is bad")
	})

	Convey("When a test servers writes stuff and Flush is called, it works as expected", t, func() {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := NewPluggableResponseWriterFromOld(w)

			p.WriteHeader(http.StatusInternalServerError)
			p.Write([]byte("Oh this is bad"))
			p.Flush()
		}))
		defer testServer.Close()

		// should return 500
		resp, err := http.Get(testServer.URL)
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
		b := bodyPool.Get()
		defer b.Close()
		b.ResetFromReader(resp.Body)
		So(b.String(), ShouldEqual, "Oh this is bad")
	})
}

func Test_Hijack(t *testing.T) {
	Convey("When a test server wraps a ResponseWriter that doesn't support Hijacking, .Hijack fails properly", t, func() {
		p := NewPluggableResponseWriter()
		_, _, err := p.Hijack()
		So(err, ShouldNotBeNil)
	})

	Convey("When a test servers wraps a ResponseWriter that supports Hijacking, .Hijack works properly", t, func(c C) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := NewPluggableResponseWriterFromOld(w)

			conn, _, err := p.Hijack()
			c.So(err, ShouldBeNil)
			conn.Close()

		}))
		defer testServer.Close()

		// should return 500
		_, err := http.Get(testServer.URL)
		So(err, ShouldNotBeNil)
	})
}

// Introducing a lock on flushing seemed non-performant to me, when all we need is
// the atomic setting of a bool. These benchmarks are here to prove it. ~3x faster
// to do atomic.Bool.Swap instead of a lock/unlock.

func BenchmarkMutex(b *testing.B) {

	var lock sync.Mutex
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		lockUnlock(&lock)
	}
}

func BenchmarkAtomicBool(b *testing.B) {
	var (
		ab atomic.Bool
		v  bool
	)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		v = ab.Swap(true)
	}
	// Satisfy vars
	if v {
		v = true
	}
}

func lockUnlock(l *sync.Mutex) {
	l.Lock()
	defer l.Unlock()
}

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type discard struct {
	header http.Header
}

func (d *discard) Header() http.Header         { return d.header }
func (d *discard) Write(b []byte) (int, error) { return len(b), nil }
func (d *discard) WriteHeader(int)             {}

func BenchmarkCachedSegment(b *testing.B) {
	e := newEnv(&testing.T{})
	_, variant := e.liveVariant(&testing.T{}, "bench")
	segment := "/hls/" + firstURI(variant)
	e.get(&testing.T{}, segment, false)
	handler := e.srv.Config.Handler
	req := httptest.NewRequest(http.MethodGet, segment, nil)
	w := &discard{header: http.Header{}}
	b.ReportAllocs()
	b.SetBytes(int64(e.fake.SegmentBytes))
	for b.Loop() {
		clear(w.header)
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkCachedVariant(b *testing.B) {
	e := newEnv(&testing.T{})
	variantPath, _ := e.liveVariant(&testing.T{}, "bench")
	handler := e.srv.Config.Handler
	req := httptest.NewRequest(http.MethodGet, variantPath, nil)
	w := &discard{header: http.Header{}}
	b.ReportAllocs()
	for b.Loop() {
		clear(w.header)
		handler.ServeHTTP(w, req)
	}
}

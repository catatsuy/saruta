package bench

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type routerAdapter interface {
	Name() string
	BuildStatic(path string) (http.Handler, error)
	BuildParam(path string) (http.Handler, error)
	BuildManyStatic(prefix string, n int) (http.Handler, string, error)
}

var adapters []routerAdapter

func registerAdapter(a routerAdapter) {
	adapters = append(adapters, a)
}

type discardResponseWriter struct {
	header http.Header
}

func (w *discardResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *discardResponseWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *discardResponseWriter) WriteHeader(statusCode int) {}

func BenchmarkStaticLookup(b *testing.B) {
	for _, a := range adapters {
		a := a
		b.Run(a.Name(), func(b *testing.B) {
			h, err := a.BuildStatic("/health")
			if err != nil {
				b.Fatalf("build router: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := &discardResponseWriter{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.ServeHTTP(w, req)
			}
		})
	}
}

func BenchmarkParamLookup(b *testing.B) {
	for _, a := range adapters {
		a := a
		b.Run(a.Name(), func(b *testing.B) {
			h, err := a.BuildParam("/users/{id}")
			if err != nil {
				b.Fatalf("build router: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/users/12345", nil)
			w := &discardResponseWriter{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.ServeHTTP(w, req)
			}
		})
	}
}

func BenchmarkDeepLookup(b *testing.B) {
	const path = "/a/b/c/d/e/f/g"
	for _, a := range adapters {
		a := a
		b.Run(a.Name(), func(b *testing.B) {
			h, err := a.BuildStatic(path)
			if err != nil {
				b.Fatalf("build router: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := &discardResponseWriter{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.ServeHTTP(w, req)
			}
		})
	}
}

func BenchmarkLookupScale(b *testing.B) {
	for _, routeCount := range []int{100, 1000, 10000} {
		routeCount := routeCount
		b.Run(fmt.Sprintf("routes=%d", routeCount), func(b *testing.B) {
			for _, a := range adapters {
				a := a
				b.Run(a.Name(), func(b *testing.B) {
					h, targetPath, err := a.BuildManyStatic("/items", routeCount)
					if err != nil {
						b.Fatalf("build router: %v", err)
					}
					req := httptest.NewRequest(http.MethodGet, targetPath, nil)
					w := &discardResponseWriter{}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						h.ServeHTTP(w, req)
					}
				})
			}
		})
	}
}

func itemPath(prefix string, i int) string {
	return prefix + "/" + strconv.Itoa(i)
}

package saruta

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

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

func BenchmarkRouterStatic(b *testing.B) {
	b.Run("saruta", func(b *testing.B) {
		r := New()
		r.Get("/health", func(w http.ResponseWriter, req *http.Request) {})
		r.MustCompile()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := &discardResponseWriter{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("servemux", func(b *testing.B) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, req *http.Request) {})
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := &discardResponseWriter{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRouterParam(b *testing.B) {
	b.Run("saruta", func(b *testing.B) {
		r := New()
		r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
			_ = req.PathValue("id")
		})
		r.MustCompile()
		req := httptest.NewRequest(http.MethodGet, "/users/12345", nil)
		w := &discardResponseWriter{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("servemux", func(b *testing.B) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, req *http.Request) {
			_ = req.PathValue("id")
		})
		req := httptest.NewRequest(http.MethodGet, "/users/12345", nil)
		w := &discardResponseWriter{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRouterDeepPath(b *testing.B) {
	path := "/a/b/c/d/e/f/g"
	b.Run("saruta", func(b *testing.B) {
		r := New()
		r.Get(path, func(w http.ResponseWriter, req *http.Request) {})
		r.MustCompile()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := &discardResponseWriter{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("servemux", func(b *testing.B) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {})
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := &discardResponseWriter{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRouterLookupScale(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run("saruta/routes="+strconv.Itoa(n), func(b *testing.B) {
			r := New()
			for i := range n {
				r.Get("/items/"+strconv.Itoa(i), func(w http.ResponseWriter, req *http.Request) {})
			}
			r.MustCompile()
			req := httptest.NewRequest(http.MethodGet, "/items/"+strconv.Itoa(n-1), nil)
			w := &discardResponseWriter{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.ServeHTTP(w, req)
			}
		})

		b.Run("servemux/routes="+strconv.Itoa(n), func(b *testing.B) {
			mux := http.NewServeMux()
			for i := range n {
				path := "/items/" + strconv.Itoa(i)
				mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {})
			}
			req := httptest.NewRequest(http.MethodGet, "/items/"+strconv.Itoa(n-1), nil)
			w := &discardResponseWriter{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mux.ServeHTTP(w, req)
			}
		})
	}
}

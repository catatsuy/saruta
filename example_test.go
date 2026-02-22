package saruta

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func ExampleRouter_basic() {
	r := New()
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("user=" + req.PathValue("id")))
	})
	r.Get("/api/{name:[0-9]+}.json", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("name=" + req.PathValue("name")))
	})
	r.MustCompile()

	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	fmt.Println(rec1.Body.String())

	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/123.json", nil))
	fmt.Println(rec2.Body.String())

	// Output:
	// user=42
	// name=123
}

func ExampleRouter_Group() {
	r := New()
	events := make([]string, 0, 4)

	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			events = append(events, "log")
			next.ServeHTTP(w, req)
		})
	}
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			events = append(events, "auth")
			next.ServeHTTP(w, req)
		})
	}

	r.Use(loggingMiddleware)
	r.Group(func(api *Router) {
		api.Use(authMiddleware)
		api.Get("/me", func(w http.ResponseWriter, req *http.Request) {
			events = append(events, "handler")
			w.Write([]byte("ok"))
		})
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me", nil))

	fmt.Println(rec.Body.String())
	fmt.Println(events)

	// Output:
	// ok
	// [log auth handler]
}

func ExampleRouter_Mount() {
	r := New()
	r.Mount("/static", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("mounted:" + req.URL.Path))
	}))
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.js", nil))
	fmt.Println(rec.Body.String())

	// Output:
	// mounted:/static/app.js
}

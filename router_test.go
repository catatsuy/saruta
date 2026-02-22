package saruta

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"
)

func TestRouterStaticAndNotFound(t *testing.T) {
	r := New()
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	r.MustCompile()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	req = httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRouterMethodNotAllowedAndAllowHeader(t *testing.T) {
	r := New()
	r.Get("/users", func(w http.ResponseWriter, req *http.Request) {})
	r.Post("/users", func(w http.ResponseWriter, req *http.Request) {})
	r.MustCompile()

	req := httptest.NewRequest(http.MethodDelete, "/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got, want := rec.Header().Get("Allow"), "GET, POST"; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestRouterPathValueSingleAndMultiple(t *testing.T) {
	r := New()
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.PathValue("id")))
	})
	r.Get("/orgs/{org}/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.PathValue("org") + ":" + req.PathValue("id")))
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if got, want := rec.Body.String(), "42"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orgs/acme/users/7", nil))
	if got, want := rec.Body.String(), "acme:7"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestRouterConstrainedParamWithSuffix(t *testing.T) {
	r := New()
	r.Get(`/api/{name:[0-9]+}.json`, func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.PathValue("name")))
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/123.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), "123"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/abc.json", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/123.txt", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRouterCatchAll(t *testing.T) {
	r := New()
	r.Get("/files/{path...}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.PathValue("path")))
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/a/b/c.txt", nil))
	if got, want := rec.Body.String(), "a/b/c.txt"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/", nil))
	if got, want := rec.Body.String(), ""; got != want {
		t.Fatalf("body = %q, want empty", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouterStaticBeatsParam(t *testing.T) {
	r := New()
	r.Get("/users/me", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("me"))
	})
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("id=" + req.PathValue("id")))
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/me", nil))
	if got, want := rec.Body.String(), "me"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestRouterUseOrderAndPathValueVisibleInMiddleware(t *testing.T) {
	r := New()
	var calls []string

	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if id := req.PathValue("id"); id != "" {
					calls = append(calls, name+":"+id)
				} else {
					calls = append(calls, name)
				}
				next.ServeHTTP(w, req)
			})
		}
	}

	r.Use(mw("A"), mw("B"), mw("C"))
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, "handler:"+req.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	want := []string{"A:42", "B:42", "C:42", "handler:42"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRouterWithAndGroupScope(t *testing.T) {
	r := New()
	var calls []string

	base := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			calls = append(calls, "base")
			next.ServeHTTP(w, req)
		})
	}
	child := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			calls = append(calls, "child")
			next.ServeHTTP(w, req)
		})
	}
	group := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			calls = append(calls, "group")
			next.ServeHTTP(w, req)
		})
	}

	r.Use(base)
	r.Get("/root", func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, "root")
	})

	r2 := r.With(child)
	r2.Get("/child", func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, "child-handler")
	})

	r.Group(func(gr *Router) {
		gr.Use(group)
		gr.Get("/group", func(w http.ResponseWriter, req *http.Request) {
			calls = append(calls, "group-handler")
		})
	})
	r.MustCompile()

	for _, tc := range []struct {
		path string
		want []string
	}{
		{path: "/root", want: []string{"base", "root"}},
		{path: "/child", want: []string{"base", "child", "child-handler"}},
		{path: "/group", want: []string{"base", "group", "group-handler"}},
	} {
		calls = nil
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if !reflect.DeepEqual(calls, tc.want) {
			t.Fatalf("%s calls = %#v, want %#v", tc.path, calls, tc.want)
		}
	}
}

func TestRouterCustomErrorHandlersAndNoUseMiddlewareApplied(t *testing.T) {
	r := New()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Use", "1")
			next.ServeHTTP(w, req)
		})
	})

	r.NotFound(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nf"))
	}))
	r.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("mna"))
	}))
	r.Get("/users", func(w http.ResponseWriter, req *http.Request) {})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("notfound status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if got := rec.Header().Get("X-Use"); got != "" {
		t.Fatalf("notfound X-Use header = %q, want empty", got)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("mna status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if got := rec.Header().Get("X-Use"); got != "" {
		t.Fatalf("mna X-Use header = %q, want empty", got)
	}
	if got, want := rec.Header().Get("Allow"), "GET"; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestRouterMount(t *testing.T) {
	r := New()
	sub := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.URL.Path))
	})
	r.Mount("/api", sub)
	r.Get("/api/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("route:" + req.PathValue("id")))
	})
	r.MustCompile()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/other", nil))
	if got, want := rec.Body.String(), "/api/other"; got != want {
		t.Fatalf("mount body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/9", nil))
	if got, want := rec.Body.String(), "route:9"; got != want {
		t.Fatalf("route body = %q, want %q", got, want)
	}
}

func TestRouterMethodSugars(t *testing.T) {
	r := New()
	type registerFn func(string, http.HandlerFunc)
	methods := map[string]registerFn{
		http.MethodGet:     r.Get,
		http.MethodPost:    r.Post,
		http.MethodPut:     r.Put,
		http.MethodPatch:   r.Patch,
		http.MethodDelete:  r.Delete,
		http.MethodHead:    r.Head,
		http.MethodOptions: r.Options,
	}

	seen := make([]string, 0, len(methods))
	for method, fn := range methods {
		m := method
		path := "/" + stringsLower(m)
		fn(path, func(w http.ResponseWriter, req *http.Request) {
			seen = append(seen, req.Method)
		})
		r.MustCompile()
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", method, rec.Code)
		}
	}
	slices.Sort(seen)
	want := []string{http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut}
	slices.Sort(want)
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("seen = %#v, want %#v", seen, want)
	}
}

func TestRouterMustMethods(t *testing.T) {
	r := New()

	defer func() {
		if v := recover(); v == nil {
			t.Fatalf("expected panic for invalid pattern")
		}
	}()

	r.Get("invalid", func(w http.ResponseWriter, req *http.Request) {})
	r.MustCompile()
}

func TestRouterMustMethodsPanicOnInvalidMatcher(t *testing.T) {
	r := New()

	defer func() {
		if v := recover(); v == nil {
			t.Fatalf("expected panic for invalid matcher")
		}
	}()

	r.Get(`/api/{name:[0-9+}.json`, func(w http.ResponseWriter, req *http.Request) {})
	r.MustCompile()
}

func TestRouterMustMethodsNoPanicOnValidRoute(t *testing.T) {
	r := New()
	r.Get("/ok", func(w http.ResponseWriter, req *http.Request) {})
	r.Get(`/api/{name:[0-9]+}.json`, func(w http.ResponseWriter, req *http.Request) {})
	r.Mount("/api", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
	r.MustCompile()
}

func TestRouterOptionPanicOnRegisterError(t *testing.T) {
	r := New(WithPanicOnCompileError())

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()

	r.Get("invalid", func(w http.ResponseWriter, req *http.Request) {})
	_ = r.Compile()
}

func TestRouterCompileReturnsError(t *testing.T) {
	r := New()
	r.Get("invalid", func(w http.ResponseWriter, req *http.Request) {})
	if err := r.Compile(); err == nil {
		t.Fatalf("expected compile error")
	}
}

func TestRouterServeHTTPPanicsBeforeCompile(t *testing.T) {
	r := New()
	r.Get("/ok", func(w http.ResponseWriter, req *http.Request) {})

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic before compile")
		}
	}()

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
}

func stringsLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		b[i] = c
	}
	return string(b)
}

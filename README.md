# saruta

`saruta` is a small radix-tree-based HTTP router for `net/http` with Go 1.22+ `PathValue()` support.

It is named after Sarutahiko, a guide deity in Japanese mythology associated with roads and directions.

## Features

- `net/http` compatible (`http.Handler`)
- Path params via `req.PathValue(...)`
- Static / param / catch-all routing (runtime radix tree)
- Middleware: `func(http.Handler) http.Handler`
- 404 / 405 (`Allow` header)
- `Mount` for static prefixes (MVP: no path strip)

## Install

```bash
go get github.com/catatsuy/saruta
```

## Quick Start

```go
package main

import (
	"log"
	"net/http"

	"github.com/catatsuy/saruta"
)

func main() {
	r := saruta.New()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			log.Printf("%s %s", req.Method, req.URL.Path)
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("user=" + req.PathValue("id")))
	})

	r.Get("/api/{name:[0-9]+}.json", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("name=" + req.PathValue("name")))
	})

	r.Get("/image/{id:[a-z0-9]+}.{ext:[a-z]+}", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(req.PathValue("id") + "." + req.PathValue("ext")))
	})

	r.Get("/files/{path...}", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("file=" + req.PathValue("path")))
	})

	if err := r.Compile(); err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

## More Examples

### Grouped middleware

```go
r := saruta.New()

loggingMiddleware := func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		log.Printf("%s %s", req.Method, req.URL.Path)
		next.ServeHTTP(w, req)
	})
}

authMiddleware := func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

r.Use(loggingMiddleware)

r.Group(func(api *saruta.Router) {
	api.Use(authMiddleware)

	api.Get("/me", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})
})

r.MustCompile()
```

### Mount another handler

```go
files := http.FileServer(http.Dir("./public"))
r.Mount("/static", files)
r.MustCompile()
```

`Mount` matches a static prefix and forwards the original path (no stripping).

### Custom 404 / 405 handlers

```go
r.NotFound(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "custom not found", http.StatusNotFound)
}))

r.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "custom method not allowed", http.StatusMethodNotAllowed)
}))
```

### Startup panic mode

```go
r := saruta.New(saruta.WithPanicOnCompileError())
r.Get("/users/{id}", usersShow)
r.Get("/users/{name}", usersShow) // conflict

// Panics instead of returning an error.
r.Compile()
```

If you prefer explicit error handling:

```go
if err := r.Compile(); err != nil {
	log.Fatal(err)
}
```

### Graceful shutdown

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/catatsuy/saruta"
)

func main() {
	r := saruta.New()
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})
	r.MustCompile()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
```

## Routing Rules (MVP)

- Pattern must start with `/`
- Trailing slash is significant (`/users` and `/users/` are different)
- Params: `/{id}`
- Constrained params (lightweight matcher, no `regexp`): `/{id:[0-9]+}`
- Prefix/suffix constrained params: `/api/{name:[0-9]+}.json`
- Multiple params in one segment: `/image/{id:[a-z0-9]+}.{ext:[a-z]+}`
- Catch-all (last segment only): `/{path...}`
- Priority: static > param > catch-all
- No automatic path normalization or redirects

## Registration API

- Registration API: `Handle`, `Get`, `Post`, ...
- Finalization API: `Compile() error`, `MustCompile()`
- Optional panic mode for `Compile()`: `New(saruta.WithPanicOnCompileError())`

Routes are validated and compiled when `Compile()` runs.
Invalid patterns/conflicts return an error from `Compile()` (or panic with `MustCompile()` / `WithPanicOnCompileError()`).

### Supported Constraint Expressions (current)

- `[0-9]+`, `[0-9]*`
- `[a-z0-9-]+`
- `\d+`, `\d*`

This is intentionally not full regular expression support for performance reasons.

## Middleware

- Type: `func(http.Handler) http.Handler`
- `Use(A, B, C)` executes as `A -> B -> C -> handler`
- `With(...)` creates a derived router sharing the same routing tree
- `Group(fn)` is a scoped `With(...)`

Matched path params are set before middleware execution, so middleware can call `req.PathValue(...)`.

## Thread Safety

- Concurrent `ServeHTTP` after route registration is safe
- Concurrent route registration and request handling is undefined
- Register routes, then call `Compile()`, then start the server

## Benchmark

Run:

```bash
go test -bench . -benchmem
```

For cross-router comparisons, use the benchmark harness in `bench/`:

```bash
cd bench
go test -run '^$' -bench . -benchmem -tags 'chi httprouter'
```

### Benchmark Snapshot (2026-02-22, Apple M1)

Command:

```bash
cd bench
go test -run '^$' -bench . -benchmem -tags 'chi httprouter'
```

Selected results:

| Benchmark | chi | httprouter | saruta | servemux |
| --- | ---: | ---: | ---: | ---: |
| Static lookup (ns/op) | 374.1 | 17.66 | 60.64 | 65.91 |
| Static lookup (allocs/op) | 2 | 0 | 0 | 0 |
| Param lookup (ns/op) | 259.7 | 39.22 | 87.74 | 133.8 |
| Param lookup (allocs/op) | 4 | 1 | 0 | 1 |
| Deep lookup (ns/op) | 190.7 | 18.03 | 62.91 | 229.2 |
| Deep lookup (allocs/op) | 2 | 0 | 0 | 0 |
| Scale 100 routes (ns/op) | 168.5 | 37.94 | 62.84 | 105.8 |
| Scale 1,000 routes (ns/op) | 202.5 | 35.44 | 77.83 | 105.7 |
| Scale 10,000 routes (ns/op) | 260.2 | 45.17 | 76.96 | 96.39 |

Notes:

- `saruta` now reaches `0 allocs/op` in the benchmarked lookup paths (static/param/deep/scale).
- `saruta` uses a runtime radix tree and remains `0 allocs/op` in the benchmarked lookup paths.
- In this benchmark run, `saruta` outperforms `ServeMux` across the listed static/param/deep/scale cases.
- `httprouter` is still faster in these microbenchmarks, especially on static/deep lookups.
- `httprouter` is significantly faster in these cases, but uses a different API/model.
- Benchmark numbers depend on CPU, Go version, and benchmark flags. Re-run on your target machine for production decisions.

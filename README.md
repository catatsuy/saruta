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
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("user=" + req.PathValue("id")))
	})

	r.Get("/api/{name:[0-9]+}.json", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("name=" + req.PathValue("name")))
	})

	r.Get("/files/{path...}", func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("file=" + req.PathValue("path")))
	})

	// Validate and compile all routes before starting the server.
	r.MustCompile()

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

## Routing Rules (MVP)

- Pattern must start with `/`
- Trailing slash is significant (`/users` and `/users/` are different)
- Params: `/{id}`
- Constrained params (lightweight matcher, no `regexp`): `/{id:[0-9]+}`
- Prefix/suffix constrained params: `/api/{name:[0-9]+}.json`
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
| Static lookup (ns/op) | 125.2 | 13.38 | 44.92 | 59.67 |
| Static lookup (allocs/op) | 2 | 0 | 0 | 0 |
| Param lookup (ns/op) | 233.2 | 32.46 | 67.17 | 108.5 |
| Param lookup (allocs/op) | 4 | 1 | 0 | 1 |
| Deep lookup (ns/op) | 124.5 | 12.99 | 44.73 | 205.3 |
| Deep lookup (allocs/op) | 2 | 0 | 0 | 0 |
| Scale 100 routes (ns/op) | 158.8 | 30.27 | 55.26 | 88.12 |
| Scale 1,000 routes (ns/op) | 182.7 | 34.20 | 69.89 | 92.67 |
| Scale 10,000 routes (ns/op) | 207.0 | 38.56 | 69.19 | 92.60 |

Notes:

- `saruta` now reaches `0 allocs/op` in the benchmarked lookup paths (static/param/deep/scale).
- `saruta` uses a runtime radix tree and now beats `ServeMux` in the benchmarked static/param/deep/scale lookups on this machine.
- `httprouter` is still faster in these microbenchmarks, especially on static/deep lookups.
- `httprouter` is significantly faster in these cases, but uses a different API/model.
- Benchmark numbers depend on CPU, Go version, and benchmark flags. Re-run on your target machine for production decisions.

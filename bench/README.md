# Benchmarks

This directory contains benchmark suites for comparing `saruta` with other routers.

## Why a separate directory?

- Keep benchmark-only dependencies out of the root module
- Allow side-by-side comparisons against other router repos
- Make it easy to add optional adapters (for example `chi`)

## Quick start

```bash
cd bench
go test -bench . -benchmem
```

This runs the default comparison targets:

- `saruta`
- `net/http` `ServeMux`

## What is measured?

- Static route lookup
- Param route lookup
- Deep path lookup
- Lookup scalability as the number of registered handlers grows (`100`, `1k`, `10k`)

The scalability benchmark is meant to show whether lookup time degrades as route count increases.

## Optional comparison targets

Optional adapters included in this directory:

- `chi` (build tag: `chi`)
- `httprouter` (build tag: `httprouter`)

```bash
cd bench
go get github.com/go-chi/chi/v5 github.com/julienschmidt/httprouter
go test -tags 'chi httprouter' -bench . -benchmem
```

## Comparing a local router repo

If you want to compare another local router implementation:

1. Add a new `targets_<name>_test.go` file in this directory
2. Register an adapter in `init()`
3. Add a `replace` directive in `bench/go.mod` pointing to the local checkout

Example `replace`:

```go
replace example.com/your/router => /path/to/router
```

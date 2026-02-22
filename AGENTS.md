# Repository Guidelines

## Project Structure & Module Organization

- Root module (`go.mod`) contains the `saruta` router library.
- Core implementation files:
  - `router.go`: public API, registration, compile/finalize, `ServeHTTP`
  - `radix.go`: routing tree construction and runtime lookup (radix tree)
  - `pattern.go`: pattern parsing and matcher compilation
  - `middleware.go`: middleware chaining
- Tests are in root `*_test.go` files (`router_test.go`, `pattern_test.go`, `radix_test.go`, `bench_test.go`).
- Cross-router benchmarks live in `bench/` as a separate Go module to isolate benchmark dependencies.

## Build, Test, and Development Commands

- `go test ./...`
  - Run all unit tests in the root module.
- `go test -run '^$' -bench . -benchmem`
  - Run root benchmarks (library-local comparisons, alloc checks).
- `cd bench && go test -run '^$' -bench . -benchmem`
  - Run comparison benchmarks (`saruta` vs `ServeMux` by default).
- `cd bench && go test -run '^$' -bench . -benchmem -tags 'chi httprouter'`
  - Run optional third-party router comparisons (requires deps in `bench/go.mod`).

## Coding Style & Naming Conventions

- Language: Go 1.25+.
- Format all changes with `gofmt -w *.go` (and `bench/*.go` when applicable).
- Keep package-level API names concise and Go-idiomatic (`New`, `Get`, `Compile`, `MustCompile`).
- Prefer clear, performance-aware code paths in router lookup logic; avoid unnecessary allocations in hot paths.

## Testing Guidelines

- Use Go’s standard `testing` package.
- Test files follow Go conventions: `*_test.go`, `TestXxx`, `BenchmarkXxx`.
- Add unit tests for behavior changes (routing precedence, params, 404/405, middleware, compile errors).
- For performance changes, include benchmark evidence (`ns/op`, `B/op`, `allocs/op`) from root and/or `bench/`.

## Commit & Pull Request Guidelines

- Current history is minimal (`Initial commit`), so use short imperative commit messages (e.g., `optimize radix lookup`, `add benchmark adapter`).
- PRs should include:
  - What changed and why
  - Behavior impact (API/compatibility)
  - Test results (`go test ./...`)
  - Benchmark results for routing hot-path changes

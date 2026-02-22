package saruta

import (
	"net/http"
	"testing"
)

func TestTrieConflictsParamAndCatchAll(t *testing.T) {
	root := newNode()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	cp, err := compilePattern("/users/{id}")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, "/users/{id}", cp, h); err != nil {
		t.Fatalf("insert first param route: %v", err)
	}

	cp2, err := compilePattern("/users/{name}")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, "/users/{name}", cp2, h); err == nil {
		t.Fatalf("expected param conflict")
	}

	cpRegex, err := compilePattern(`/posts/{slug:[a-z0-9-]+}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, `/posts/{slug:[a-z0-9-]+}`, cpRegex, h); err != nil {
		t.Fatalf("insert regex param route: %v", err)
	}

	cpRegexConflict, err := compilePattern(`/posts/{slug:[0-9]+}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, `/posts/{slug:[0-9]+}`, cpRegexConflict, h); err == nil {
		t.Fatalf("expected regex param conflict")
	}

	cp3, err := compilePattern("/files/{path...}")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, "/files/{path...}", cp3, h); err != nil {
		t.Fatalf("insert first catch-all route: %v", err)
	}

	cp4, err := compilePattern("/files/{rest...}")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, "/files/{rest...}", cp4, h); err == nil {
		t.Fatalf("expected catch-all conflict")
	}
}

func TestTrieDuplicateRoute(t *testing.T) {
	root := newNode()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	cp, err := compilePattern("/users/{id}")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, "/users/{id}", cp, h); err != nil {
		t.Fatal(err)
	}
	if err := root.insertRoute(http.MethodGet, "/users/{id}", cp, h); err == nil {
		t.Fatalf("expected duplicate route error")
	}
}

func TestTriePriorityStaticParamCatchAll(t *testing.T) {
	root := newNode()
	mark := func(s string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}
	mustInsert := func(method, pattern string) {
		t.Helper()
		cp, err := compilePattern(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if err := root.insertRoute(method, pattern, cp, mark(pattern)); err != nil {
			t.Fatal(err)
		}
	}

	mustInsert(http.MethodGet, "/users/me")
	mustInsert(http.MethodGet, "/users/{id}")
	mustInsert(http.MethodGet, "/users/{rest...}")

	rt := buildRadix(root)

	m, ok := rt.matchRoute("/users/me")
	if !ok {
		t.Fatalf("expected match")
	}
	if _, ok := m.leaf.handlers[http.MethodGet]; !ok {
		t.Fatalf("expected GET handler")
	}
	if m.paramCount != 0 {
		t.Fatalf("params count = %d, want none", m.paramCount)
	}

	m, ok = rt.matchRoute("/users/42")
	if !ok {
		t.Fatalf("expected match for param")
	}
	if m.paramCount != 1 || m.params[0].name != "id" || m.params[0].value != "42" {
		t.Fatalf("params = %#v", m.params[:m.paramCount])
	}

	m, ok = rt.matchRoute("/users/a/b")
	if !ok {
		t.Fatalf("expected catch-all match")
	}
	if m.paramCount != 1 || m.params[0].name != "rest" || m.params[0].value != "a/b" {
		t.Fatalf("params = %#v", m.params[:m.paramCount])
	}
}

//go:build chi

package bench

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type chiAdapter struct{}

func (chiAdapter) Name() string { return "chi" }

func (chiAdapter) BuildStatic(path string) (http.Handler, error) {
	r := chi.NewRouter()
	r.Get(path, func(w http.ResponseWriter, req *http.Request) {})
	return r, nil
}

func (chiAdapter) BuildParam(path string) (http.Handler, error) {
	r := chi.NewRouter()
	r.Get(path, func(w http.ResponseWriter, req *http.Request) {
		_ = chi.URLParam(req, "id")
	})
	return r, nil
}

func (chiAdapter) BuildManyStatic(prefix string, n int) (http.Handler, string, error) {
	r := chi.NewRouter()
	for i := 0; i < n; i++ {
		path := itemPath(prefix, i)
		r.Get(path, func(w http.ResponseWriter, req *http.Request) {})
	}
	return r, itemPath(prefix, n-1), nil
}

func init() {
	registerAdapter(chiAdapter{})
}

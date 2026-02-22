package bench

import (
	"net/http"

	"github.com/catatsuy/saruta"
)

type sarutaAdapter struct{}

func (sarutaAdapter) Name() string { return "saruta" }

func (sarutaAdapter) BuildStatic(path string) (http.Handler, error) {
	r := saruta.New()
	r.Get(path, func(w http.ResponseWriter, req *http.Request) {})
	if err := r.Compile(); err != nil {
		return nil, err
	}
	return r, nil
}

func (sarutaAdapter) BuildParam(path string) (http.Handler, error) {
	r := saruta.New()
	r.Get(path, func(w http.ResponseWriter, req *http.Request) {
		_ = req.PathValue("id")
	})
	if err := r.Compile(); err != nil {
		return nil, err
	}
	return r, nil
}

func (sarutaAdapter) BuildManyStatic(prefix string, n int) (http.Handler, string, error) {
	r := saruta.New()
	for i := 0; i < n; i++ {
		path := itemPath(prefix, i)
		r.Get(path, func(w http.ResponseWriter, req *http.Request) {})
	}
	if err := r.Compile(); err != nil {
		return nil, "", err
	}
	return r, itemPath(prefix, n-1), nil
}

func init() {
	registerAdapter(sarutaAdapter{})
}

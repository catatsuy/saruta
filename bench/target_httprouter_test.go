//go:build httprouter

package bench

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type httprouterAdapter struct{}

func (httprouterAdapter) Name() string { return "httprouter" }

func (httprouterAdapter) BuildStatic(path string) (http.Handler, error) {
	r := httprouter.New()
	r.GET(path, func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {})
	return r, nil
}

func (httprouterAdapter) BuildParam(_ string) (http.Handler, error) {
	r := httprouter.New()
	r.GET("/users/:id", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		_ = ps.ByName("id")
	})
	return r, nil
}

func (httprouterAdapter) BuildManyStatic(prefix string, n int) (http.Handler, string, error) {
	r := httprouter.New()
	for i := 0; i < n; i++ {
		path := itemPath(prefix, i)
		r.GET(path, func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {})
	}
	return r, itemPath(prefix, n-1), nil
}

func init() {
	registerAdapter(httprouterAdapter{})
}

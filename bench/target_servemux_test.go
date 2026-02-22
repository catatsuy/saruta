package bench

import "net/http"

type serveMuxAdapter struct{}

func (serveMuxAdapter) Name() string { return "servemux" }

func (serveMuxAdapter) BuildStatic(path string) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {})
	return mux, nil
}

func (serveMuxAdapter) BuildParam(path string) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {
		_ = req.PathValue("id")
	})
	return mux, nil
}

func (serveMuxAdapter) BuildManyStatic(prefix string, n int) (http.Handler, string, error) {
	mux := http.NewServeMux()
	for i := 0; i < n; i++ {
		path := itemPath(prefix, i)
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {})
	}
	return mux, itemPath(prefix, n-1), nil
}

func init() {
	registerAdapter(serveMuxAdapter{})
}

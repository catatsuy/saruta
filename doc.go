// Package saruta provides a small radix-tree-based HTTP router for net/http.
//
// Routes are registered first and validated/compiled by calling Compile or
// MustCompile before serving requests.
//
// Example:
//
//	r := saruta.New()
//	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
//		w.Write([]byte(req.PathValue("id")))
//	})
//	r.MustCompile()
//	http.ListenAndServe(":8080", r)
package saruta

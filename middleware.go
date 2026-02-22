package saruta

import "net/http"

// Middleware wraps an http.Handler.
//
// Middleware is applied in registration order, so Use(A, B) executes as:
// A -> B -> handler.
type Middleware func(http.Handler) http.Handler

func chainMiddlewares(h http.Handler, mws []Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

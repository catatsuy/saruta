package saruta

import (
	"fmt"
	"net/http"
)

type Router struct {
	state      *routerState
	middleware []Middleware
}

type routerState struct {
	root             *radixNode
	notFound         http.Handler
	methodNotAllowed http.Handler

	routes []registeredRoute
	mounts []registeredMount

	compiled          bool
	panicOnCompileErr bool
}

type registeredRoute struct {
	method     string
	pattern    string
	handler    http.Handler
	middleware []Middleware
}

type registeredMount struct {
	prefix  string
	handler http.Handler
}

type Option func(*Router)

func WithPanicOnCompileError() Option {
	return func(r *Router) {
		r.state.panicOnCompileErr = true
	}
}

func New(opts ...Option) *Router {
	r := &Router{
		state: &routerState{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *Router) Handle(method, pattern string, h http.Handler) {
	r.state.routes = append(r.state.routes, registeredRoute{
		method:     method,
		pattern:    pattern,
		handler:    h,
		middleware: append([]Middleware(nil), r.middleware...),
	})
	r.state.compiled = false
}

func (r *Router) HandleFunc(method, pattern string, h http.HandlerFunc) {
	r.Handle(method, pattern, h)
}

func (r *Router) Get(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodGet, pattern, h)
}

func (r *Router) Post(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodPost, pattern, h)
}

func (r *Router) Put(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodPut, pattern, h)
}

func (r *Router) Patch(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodPatch, pattern, h)
}

func (r *Router) Delete(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodDelete, pattern, h)
}

func (r *Router) Head(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodHead, pattern, h)
}

func (r *Router) Options(pattern string, h http.HandlerFunc) {
	r.HandleFunc(http.MethodOptions, pattern, h)
}

func (r *Router) Use(mw ...Middleware) {
	r.middleware = append(r.middleware, mw...)
}

func (r *Router) With(mw ...Middleware) *Router {
	combined := make([]Middleware, 0, len(r.middleware)+len(mw))
	combined = append(combined, r.middleware...)
	combined = append(combined, mw...)
	return &Router{
		state:      r.state,
		middleware: combined,
	}
}

func (r *Router) Group(fn func(r *Router)) {
	if fn == nil {
		return
	}
	fn(r.With())
}

func (r *Router) Mount(prefix string, h http.Handler) {
	r.state.mounts = append(r.state.mounts, registeredMount{
		prefix:  prefix,
		handler: h,
	})
	r.state.compiled = false
}

func (r *Router) Compile() error {
	root := newNode()

	for _, rt := range r.state.routes {
		if rt.method == "" {
			return r.compileError(fmt.Errorf("invalid method: empty"))
		}
		if rt.handler == nil {
			return r.compileError(fmt.Errorf("invalid handler: nil"))
		}
		cp, err := compilePattern(rt.pattern)
		if err != nil {
			return r.compileError(err)
		}
		h := chainMiddlewares(rt.handler, rt.middleware)
		if err := root.insertRoute(rt.method, rt.pattern, cp, h); err != nil {
			return r.compileError(err)
		}
	}

	for _, mt := range r.state.mounts {
		if mt.handler == nil {
			return r.compileError(fmt.Errorf("invalid handler: nil"))
		}
		cp, err := compilePattern(mt.prefix)
		if err != nil {
			return r.compileError(err)
		}
		for _, seg := range cp.segments {
			if seg.kind != segmentStatic {
				return r.compileError(fmt.Errorf("invalid mount prefix %q: prefix must be a static path", mt.prefix))
			}
		}
		if err := root.insertMount(mt.prefix, cp, mt.handler); err != nil {
			return r.compileError(err)
		}
	}

	r.state.root = buildRadix(root)
	r.state.compiled = true
	return nil
}

func (r *Router) MustCompile() {
	if err := r.Compile(); err != nil {
		panic(err)
	}
}

func (r *Router) NotFound(h http.Handler) {
	r.state.notFound = h
}

func (r *Router) MethodNotAllowed(h http.Handler) {
	r.state.methodNotAllowed = h
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !r.state.compiled || r.state.root == nil {
		panic("saruta: router is not compiled; call Compile or MustCompile before serving")
	}
	if req == nil || req.URL == nil {
		http.NotFound(w, req)
		return
	}
	path := req.URL.Path
	if path == "" || path[0] != '/' {
		r.serveNotFound(w, req)
		return
	}

	if matched, ok := r.state.root.matchRoute(path); ok {
		if h, ok := matched.leaf.handlers[req.Method]; ok {
			for i := 0; i < matched.paramCount; i++ {
				p := matched.params[i]
				req.SetPathValue(p.name, p.value)
			}
			h.ServeHTTP(w, req)
			return
		}
		if len(matched.leaf.handlers) > 0 {
			allow := allowHeaderValue(matched.leaf.handlers)
			if allow != "" {
				w.Header().Set("Allow", allow)
			}
			r.serveMethodNotAllowed(w, req)
			return
		}
	}

	if h := r.state.root.findMount(path); h != nil {
		h.ServeHTTP(w, req)
		return
	}

	r.serveNotFound(w, req)
}

func (r *Router) serveNotFound(w http.ResponseWriter, req *http.Request) {
	if r.state.notFound != nil {
		r.state.notFound.ServeHTTP(w, req)
		return
	}
	http.NotFound(w, req)
}

func (r *Router) serveMethodNotAllowed(w http.ResponseWriter, req *http.Request) {
	if r.state.methodNotAllowed != nil {
		r.state.methodNotAllowed.ServeHTTP(w, req)
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (r *Router) compileError(err error) error {
	if err == nil {
		return nil
	}
	if r.state.panicOnCompileErr {
		panic(err)
	}
	return err
}

package web

import (
	"net/http"
	"net/url"
)

// Router registers routes on an http.ServeMux and remembers each one, so
// Lookup can tell a real page from the catch-all for a path without a request.
// Registration happens once at startup, before serving, so it needs no locking.
type Router struct {
	mux    *http.ServeMux
	routes map[string]*Route // keyed by the exact pattern the mux stores
}

func NewRouter(mux *http.ServeMux) *Router {
	return &Router{mux: mux, routes: make(map[string]*Route)}
}

func (rt *Router) Get(pattern string, h http.HandlerFunc) *Route {
	return rt.Handle(http.MethodGet, pattern, h)
}

func (rt *Router) Post(pattern string, h http.HandlerFunc) *Route {
	return rt.Handle(http.MethodPost, pattern, h)
}

// Handle registers h and returns the route. An empty method registers the pattern
// without one (any method matches).
func (rt *Router) Handle(method, pattern string, h http.Handler) *Route {
	full := pattern
	if method != "" {
		full = method + " " + pattern
	}
	// A duplicate registration panics inside ServeMux; let it.
	rt.mux.Handle(full, h)
	route := &Route{method: method, pattern: pattern}
	rt.routes[full] = route
	return route
}

// Lookup resolves a path to the route that would serve it, nil when nothing
// matches or it was not registered through the Router.
func (rt *Router) Lookup(method, p string) *Route {
	req := &http.Request{Method: method, URL: &url.URL{Path: p}}
	_, pattern := rt.mux.Handler(req)
	if pattern == "" {
		return nil
	}
	return rt.routes[pattern]
}

// Routes lists every route registered through the Router, in no set order.
func (rt *Router) Routes() []*Route {
	out := make([]*Route, 0, len(rt.routes))
	for _, route := range rt.routes {
		out = append(out, route)
	}
	return out
}

// Route is a route registered through the Router.
type Route struct {
	method  string
	pattern string
}

// Method is the HTTP method the route answers, "" for any.
func (rt *Route) Method() string { return rt.method }

// Pattern is the path with its wildcards intact ("/servers/{id}/settings").
func (rt *Route) Pattern() string { return rt.pattern }

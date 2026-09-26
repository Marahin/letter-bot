package web

import (
	"net/http"
	"net/url"
	"strings"
)

// Router registers routes on an http.ServeMux and remembers each one, so a URL can
// be rebuilt from the route itself instead of string-editing a rendered path.
// Registration happens once at startup, before serving, so it needs no locking.
type Router struct {
	mux    *http.ServeMux
	routes map[string]*Route // keyed by the exact pattern the mux stores in r.Pattern
}

// NewRouter wraps mux; routes registered through it stay reachable by name.
func NewRouter(mux *http.ServeMux) *Router {
	return &Router{mux: mux, routes: make(map[string]*Route)}
}

// Mux is the escape hatch for registrations that must bypass the named-route layer.
func (rt *Router) Mux() *http.ServeMux { return rt.mux }

func (rt *Router) Get(pattern string, h http.HandlerFunc) *Route {
	return rt.Handle(http.MethodGet, pattern, h)
}

func (rt *Router) Post(pattern string, h http.HandlerFunc) *Route {
	return rt.Handle(http.MethodPost, pattern, h)
}

func (rt *Router) Delete(pattern string, h http.HandlerFunc) *Route {
	return rt.Handle(http.MethodDelete, pattern, h)
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
	route := &Route{method: method, pattern: pattern, params: parseParams(pattern)}
	rt.routes[full] = route
	return route
}

// Matched returns the route that r was routed to, nil when nothing matched or the
// route was registered elsewhere. ServeMux sets r.Pattern to the exact registered
// pattern string, which is the key used here.
func (rt *Router) Matched(r *http.Request) *Route {
	if r.Pattern == "" {
		return nil
	}
	return rt.routes[r.Pattern]
}

// Lookup resolves a path to the GET route that would serve it, nil when nothing
// matches or it was not registered through the Router.
func (rt *Router) Lookup(method, p string) *Route {
	req := &http.Request{Method: method, URL: &url.URL{Path: p}}
	// The mux answers with the pattern it matched, which is the key used here.
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

// Route is a registered route and the wildcards its pattern declares.
type Route struct {
	method  string
	pattern string
	params  []string
}

func (rt *Route) Method() string { return rt.method }

// Pattern is the path with its wildcards intact ("/servers/{id}/settings").
func (rt *Route) Pattern() string { return rt.pattern }

// Params are the wildcard names in declaration order.
func (rt *Route) Params() []string { return rt.params }

// Path fills the wildcards from values. ok is false when any wildcard is unset, and
// nothing partial is returned: a caller building an href can fall back cleanly.
// Values naming no wildcard are ignored.
func (rt *Route) Path(values map[string]string) (string, bool) {
	var b strings.Builder
	rest := rt.pattern
	for {
		before, after, found := strings.Cut(rest, "{")
		b.WriteString(before)
		if !found {
			return b.String(), true
		}
		name, tail, closed := strings.Cut(after, "}")
		if !closed {
			return "", false // not a legal ServeMux pattern
		}
		rest = tail
		if name == "$" {
			continue // an anchor, not part of the URL: "/{$}" builds "/"
		}
		v, ok := values[strings.TrimSuffix(name, "...")]
		if !ok || v == "" {
			return "", false
		}
		b.WriteString(v)
	}
}

// PathOf is positional Path: one value per wildcard in declaration order.
func (rt *Route) PathOf(values ...string) (string, bool) {
	if len(values) != len(rt.params) {
		return "", false
	}
	byName := make(map[string]string, len(values))
	for i, name := range rt.params {
		byName[name] = values[i]
	}
	return rt.Path(byName)
}

// parseParams reads the wildcard names out of a ServeMux pattern: "{name}" and the
// trailing "{name...}" are wildcards, "{$}" is an end-of-path anchor and is not.
func parseParams(pattern string) []string {
	var params []string
	rest := pattern
	for {
		_, after, found := strings.Cut(rest, "{")
		if !found {
			return params
		}
		name, tail, closed := strings.Cut(after, "}")
		if !closed {
			return params
		}
		rest = tail
		if name == "$" {
			continue
		}
		params = append(params, strings.TrimSuffix(name, "..."))
	}
}

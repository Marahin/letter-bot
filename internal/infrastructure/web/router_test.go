package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteParams_ParsedFromPattern(t *testing.T) {
	// given routes covering each pattern shape
	rt := NewRouter(http.NewServeMux())
	plain := rt.Get("/dashboard", okHandler)
	one := rt.Get("/servers/{id}/settings", okHandler)
	two := rt.Post("/servers/{id}/sessions/{sessid}/stats", okHandler)
	trailing := rt.Get("/builds/assets/{path...}", okHandler)
	anchored := rt.Get("/{$}", okHandler)

	// then only real wildcards count; {$} is an anchor, {name...} keeps its bare name
	assert.Empty(t, plain.Params())
	assert.Equal(t, []string{"id"}, one.Params())
	assert.Equal(t, []string{"id", "sessid"}, two.Params())
	assert.Equal(t, []string{"path"}, trailing.Params())
	assert.Empty(t, anchored.Params())

	// and the route remembers how it was declared
	assert.Equal(t, http.MethodPost, two.Method())
	assert.Equal(t, "/servers/{id}/sessions/{sessid}/stats", two.Pattern())
}

func TestRoutePath_BuildsAndRefuses(t *testing.T) {
	rt := NewRouter(http.NewServeMux())
	settings := rt.Get("/servers/{id}/settings", okHandler)
	stats := rt.Get("/servers/{id}/sessions/{sessid}/stats", okHandler)
	plain := rt.Get("/dashboard", okHandler)
	trailing := rt.Get("/builds/assets/{path...}", okHandler)
	anchored := rt.Get("/{$}", okHandler)

	// given every wildcard has a value
	got, ok := settings.Path(map[string]string{"id": "42"})
	assert.True(t, ok)
	assert.Equal(t, "/servers/42/settings", got)

	// given extra entries naming no wildcard: ignored
	got, ok = settings.Path(map[string]string{"id": "42", "sessid": "9", "junk": "x"})
	assert.True(t, ok)
	assert.Equal(t, "/servers/42/settings", got)

	// given a missing value: no path at all, not a half-built one
	got, ok = stats.Path(map[string]string{"id": "42"})
	assert.False(t, ok)
	assert.Empty(t, got)

	// given an empty value: treated as missing
	got, ok = settings.Path(map[string]string{"id": ""})
	assert.False(t, ok)
	assert.Empty(t, got)

	// given a wildcard-free pattern: the pattern itself
	got, ok = plain.Path(nil)
	assert.True(t, ok)
	assert.Equal(t, "/dashboard", got)

	// given a trailing wildcard: the value fills the suffix
	got, ok = trailing.Path(map[string]string{"path": "icons/rgoz.png"})
	assert.True(t, ok)
	assert.Equal(t, "/builds/assets/icons/rgoz.png", got)

	// given an anchored pattern: the anchor is not part of the URL
	got, ok = anchored.Path(nil)
	assert.True(t, ok)
	assert.Equal(t, "/", got)
}

func TestRoutePathOf_PositionalArity(t *testing.T) {
	// given a two-wildcard route
	rt := NewRouter(http.NewServeMux())
	stats := rt.Get("/servers/{id}/sessions/{sessid}/stats", okHandler)

	// when one value per wildcard is passed in declaration order
	got, ok := stats.PathOf("42", "7")

	// then the path is built
	assert.True(t, ok)
	assert.Equal(t, "/servers/42/sessions/7/stats", got)

	// when the arity is wrong in either direction
	for _, values := range [][]string{{}, {"42"}, {"42", "7", "extra"}} {
		got, ok := stats.PathOf(values...)
		assert.Falsef(t, ok, "%v should not satisfy two wildcards", values)
		assert.Empty(t, got)
	}
}

func TestRouterMatched_FindsRouteForRoutedRequest(t *testing.T) {
	// given a route whose handler asks the router what matched
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	var matched *Route
	var pattern string
	settings := rt.Get("/servers/{id}/settings", func(w http.ResponseWriter, r *http.Request) {
		matched, pattern = rt.Matched(r), r.Pattern
		okHandler(w, r)
	})

	// when the mux routes a real request to it
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/servers/42/settings", nil))

	// then the pattern the mux records is the key Matched looks up
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "GET /servers/{id}/settings", pattern, "ServeMux stores the method-prefixed pattern")
	require.Same(t, settings, matched)

	// and a route registered straight on the mux stays unknown to the router
	var bypass *Route
	mux.HandleFunc("GET /bypass", func(_ http.ResponseWriter, r *http.Request) { bypass = rt.Matched(r) })
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bypass", nil))
	assert.Nil(t, bypass)
}

func TestRouterLookup_ResolvesPathWithoutARequest(t *testing.T) {
	// given a router with a plain page, a guild-scoped route and no catch-all
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	planner := rt.Get("/battleplanner", okHandler)
	settings := rt.Get("/servers/{id}/settings", okHandler)

	// then a registered GET path resolves to its route, wildcards filled in or not
	assert.Same(t, planner, rt.Lookup(http.MethodGet, "/battleplanner"))
	assert.Same(t, settings, rt.Lookup(http.MethodGet, "/servers/42/settings"))

	// and an unclaimed path resolves to nothing
	assert.Nil(t, rt.Lookup(http.MethodGet, "/nope"))

	// and a route registered straight on the mux stays unknown to the router
	mux.HandleFunc("GET /bypass", okHandler)
	assert.Nil(t, rt.Lookup(http.MethodGet, "/bypass"))
}

func TestRouterLookup_UnclaimedPathFindsTheCatchAll(t *testing.T) {
	// given a router whose catch-all brands the 404
	rt := NewRouter(http.NewServeMux())
	catchAll := rt.Get("/", okHandler)
	rt.Get("/dashboard", okHandler)

	// then an unclaimed path is the catch-all, which the caller must tell apart
	// from a real page by its pattern
	assert.Same(t, catchAll, rt.Lookup(http.MethodGet, "/afssaf"))
	assert.Equal(t, "/", catchAll.Pattern())
}

func TestRouter_MuxAndDelete(t *testing.T) {
	// given a router over a mux
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	del := rt.Delete("/servers/{id}/squads/{squadID}", okHandler)

	// then Mux is the same mux, and DELETE is registered under its method
	assert.Same(t, mux, rt.Mux())
	assert.Equal(t, http.MethodDelete, del.Method())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/servers/1/squads/2", nil))
	assert.Equal(t, sentinel, rec.Body.String())
}

// An unclosed "{" is not a legal ServeMux pattern (registering one panics in the
// mux), so the parser just stops rather than inventing a wildcard.
func TestRoute_MalformedPatternYieldsNothing(t *testing.T) {
	assert.Empty(t, parseParams("/servers/{id"))
	broken := &Route{pattern: "/servers/{id"}
	got, ok := broken.Path(map[string]string{"id": "1"})
	assert.False(t, ok)
	assert.Empty(t, got)
}

func TestRouterMatched_NilWithoutAPattern(t *testing.T) {
	// given a request that no route matched (empty r.Pattern)
	rt := NewRouter(http.NewServeMux())
	rt.Get("/dashboard", okHandler)

	// then
	assert.Nil(t, rt.Matched(httptest.NewRequest(http.MethodGet, "/nope", nil)))
}

func TestRouterHandle_RegistersNonHandlerFuncAndMethodless(t *testing.T) {
	// given the two shell registrations that are not HandlerFuncs, plus a
	// method-less route
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	assets := rt.Handle(http.MethodGet, "/assets/", http.StripPrefix("/assets/", http.NotFoundHandler()))
	favicon := rt.Handle(http.MethodGet, "/favicon.ico", http.RedirectHandler("/assets/favicon.ico", http.StatusMovedPermanently))
	anyMethod := rt.Handle("", "/webhook", http.HandlerFunc(okHandler))

	// then the patterns survive and the method-less one is unconstrained
	assert.Equal(t, "/assets/", assets.Pattern())
	assert.Empty(t, anyMethod.Method())

	// and the mux serves them
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	assert.Equal(t, http.StatusMovedPermanently, rec.Code)
	assert.Equal(t, "/assets/favicon.ico", rec.Header().Get("Location"))
	assert.Equal(t, "GET /favicon.ico", favicon.Method()+" "+favicon.Pattern())

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/webhook", nil))
	assert.Equal(t, sentinel, rec.Body.String(), "a method-less route accepts any method")
}

// TestHandler_FaviconProbeKeepsTheBareTarget asserts the real route the stand-in above
// only imitates. Its Location must stay unstamped: a 301 lives in the browser's redirect
// cache (and the edge's) past the release that issued it, so a stamped target would pin a
// returning visitor to an old build. layout.templ's <link rel="icon"> tags carry the stamp.
// The clan switcher rebases the matched route onto another clan. A route whose only
// wildcard is the clan id can travel; one naming a record of the clan being left
// cannot, and that is now arity rather than a string scan.
func TestRoute_GuildRebase(t *testing.T) {
	rt := NewRouter(http.NewServeMux())
	settings := rt.Get("/servers/{id}/settings", okHandler)
	sessionStats := rt.Get("/servers/{id}/sessions/{sessid}/stats", okHandler)

	// when rebasing onto another clan id
	got, ok := settings.PathOf("999")
	assert.True(t, ok)
	assert.Equal(t, "/servers/999/settings", got)

	// then a record-scoped route refuses: a clan id alone cannot fill it
	got, ok = sessionStats.PathOf("999")
	assert.False(t, ok)
	assert.Empty(t, got)
	got, ok = sessionStats.Path(map[string]string{guildIDPathValue: "999"})
	assert.False(t, ok)
	assert.Empty(t, got)

	// and a clan id that collides with a path literal is substituted by name, never
	// by search-and-replace over the rendered path
	got, ok = settings.PathOf("settings")
	assert.True(t, ok)
	assert.Equal(t, "/servers/settings/settings", got)
}

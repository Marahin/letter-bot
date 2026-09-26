package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouterLookup_ResolvesPathWithoutARequest(t *testing.T) {
	// given a router with a plain page, a guild-scoped route and no catch-all
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	planner := rt.Get("/battleplanner", okHandler)
	settings := rt.Get("/servers/{id}/settings", okHandler)

	// then a registered GET path resolves to its route, wildcards filled in or not
	assert.Same(t, planner, rt.Lookup(http.MethodGet, "/battleplanner"))
	assert.Same(t, settings, rt.Lookup(http.MethodGet, "/servers/42/settings"))
	assert.Equal(t, "/servers/{id}/settings", settings.Pattern())

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

func TestRouterHandle_RegistersNonHandlerFuncAndMethodless(t *testing.T) {
	// given the two shell registrations that are not HandlerFuncs, plus a
	// method-less route
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	assets := rt.Handle(http.MethodGet, "/assets/", http.StripPrefix("/assets/", http.NotFoundHandler()))
	rt.Handle(http.MethodGet, "/favicon.ico", http.RedirectHandler("/assets/favicon.ico", http.StatusMovedPermanently))
	rt.Handle("", "/webhook", http.HandlerFunc(okHandler))
	rt.Post("/form", okHandler)

	// when
	favicon := httptest.NewRecorder()
	mux.ServeHTTP(favicon, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	webhook := httptest.NewRecorder()
	mux.ServeHTTP(webhook, httptest.NewRequest(http.MethodPost, "/webhook", nil))
	form := httptest.NewRecorder()
	mux.ServeHTTP(form, httptest.NewRequest(http.MethodPost, "/form", nil))

	// then
	assert.Equal(t, "/assets/", assets.Pattern())
	assert.Equal(t, http.StatusMovedPermanently, favicon.Code)
	assert.Equal(t, "/assets/favicon.ico", favicon.Header().Get("Location"))
	assert.Equal(t, sentinel, webhook.Body.String(), "a method-less route accepts any method")
	assert.Equal(t, sentinel, form.Body.String())
}

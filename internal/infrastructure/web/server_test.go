package web

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := Config{BaseURL: "http://localhost:8080", Discord: DiscordConfig{ClientID: "4242"}}
	return newServerWithSessions(cfg, zap.NewNop().Sugar(), NewSessionManager(cfg, nil)).WithLandingTool(templ.NopComponent)
}

func serveReq(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func htmlGet(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept", "text/html")
	return req
}

func languagePost(form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/language", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}

func TestHandler_LandingRendersInEnglishByDefault(t *testing.T) {
	// given
	h := newTestServer(t).Handler()

	// when
	rec := serveReq(t, h, htmlGet("/"))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `<html lang="en"`)
	assert.Contains(t, body, "Reserve respawns on Discord.")
	assert.Contains(t, body, "client_id=4242")
	assert.Contains(t, body, "permissions="+InviteBotPermissions)
	assert.Contains(t, body, `href="/login"`)
	assert.Contains(t, body, `href="/tools/loot-calculator"`)
	assert.Contains(t, rec.Header().Values("Vary"), "Accept-Language")
}

func TestHandler_LocaleResolutionOrder(t *testing.T) {
	h := newTestServer(t).Handler()
	for name, tc := range map[string]struct {
		query, cookie, accept string
		want                  string
	}{
		"accept-language":            {accept: "pl-PL,pl;q=0.9", want: "pl"},
		"cookie beats header":        {cookie: "en", accept: "pl", want: "en"},
		"query beats cookie":         {query: "?lang=pl", cookie: "en", want: "pl"},
		"unsupported query ignored":  {query: "?lang=ru", cookie: "pl", want: "pl"},
		"unsupported cookie ignored": {cookie: "xx", accept: "pl", want: "pl"},
		"nothing is english":         {want: "en"},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			req := htmlGet("/" + tc.query)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: langCookieName, Value: tc.cookie})
			}
			if tc.accept != "" {
				req.Header.Set("Accept-Language", tc.accept)
			}

			// when
			rec := serveReq(t, h, req)

			// then
			assert.Contains(t, rec.Body.String(), `<html lang="`+tc.want+`"`)
		})
	}
}

func TestHandler_LandingInPolish(t *testing.T) {
	// when
	rec := serveReq(t, newTestServer(t).Handler(), htmlGet("/?lang=pl"))

	// then
	body := rec.Body.String()
	assert.Contains(t, body, "Rezerwuj respy na Discordzie.")
	assert.Contains(t, body, "/assets/l10n/pl.js")
	assert.Contains(t, body, "Pomóż nam ulepszyć to tłumaczenie")
}

func TestHandler_HtmxRequestFollowsThePageLanguage(t *testing.T) {
	// given an htmx request from a ?lang=pl page
	req := htmlGet("/nope")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Current-URL", "http://localhost:8080/?lang=pl")

	// when
	rec := serveReq(t, newTestServer(t).Handler(), req)

	// then the plain-text 404 is Polish
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "Nie znaleziono.\n", rec.Body.String())
}

func TestHandler_UnknownPathIsBrandedNotFound(t *testing.T) {
	// when
	rec := serveReq(t, newTestServer(t).Handler(), htmlGet("/nope"))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<title>Page not found - TibiaLoot.com</title>")
	assert.Contains(t, body, ">404<")
	assert.Contains(t, body, "Back to home")
}

func TestHandler_UnknownPathIsPlainForNonHTMLClients(t *testing.T) {
	// when
	rec := serveReq(t, newTestServer(t).Handler(), httptest.NewRequest(http.MethodGet, "/nope", nil))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "Not found.\n", rec.Body.String())
}

func TestHandler_FaviconProbeKeepsTheBareTarget(t *testing.T) {
	// when a browser probes the site root for a favicon
	rec := serveReq(t, newTestServer(t).Handler(), httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))

	// then it is redirected to the bare asset path
	assert.Equal(t, http.StatusMovedPermanently, rec.Code)
	assert.Equal(t, "/assets/favicon-32.png", rec.Header().Get("Location"))
}

func TestHandler_ServesAssetsAndStampedManifest(t *testing.T) {
	h := newTestServer(t).Handler()

	// when
	svg := serveReq(t, h, httptest.NewRequest(http.MethodGet, "/assets/favicon.svg", nil))
	manifest := serveReq(t, h, httptest.NewRequest(http.MethodGet, "/assets/site.webmanifest", nil))

	// then
	assert.Equal(t, http.StatusOK, svg.Code)
	assert.Contains(t, svg.Body.String(), "#F97316")
	assert.Equal(t, http.StatusOK, manifest.Code)
	assert.Equal(t, "application/manifest+json", manifest.Header().Get("Content-Type"))
	assert.Contains(t, manifest.Body.String(), `"name": "TibiaLoot.com"`)
	assert.Contains(t, manifest.Body.String(), "/assets/favicon-192.png"+AssetQuery())
}

func TestHandler_MountRegistersFeatureRoutes(t *testing.T) {
	// given a feature that registers a page
	s := newTestServer(t)
	var got *Deps
	s.Mount(func(rt *Router, d *Deps) {
		got = d
		rt.Get("/feature", okHandler)
	})

	// when
	rec := serveReq(t, s.Handler(), htmlGet("/feature"))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), sentinel)
	require.NotNil(t, got)
	assert.NotNil(t, got.Routes)
}

func TestSetLanguage_SetsCookieAndReturnsToThePage(t *testing.T) {
	// given
	h := newTestServer(t).Handler()

	// when
	rec := serveReq(t, h, languagePost(url.Values{"lang": {"pl"}, "to": {"/?lang=en&tab=2"}}))

	// then ?lang= is dropped, so it cannot override the pick
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?tab=2", rec.Header().Get("Location"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, langCookieName, cookies[0].Name)
	assert.Equal(t, "pl", cookies[0].Value)
}

func TestSetLanguage_UnsupportedCodeIsEnglish(t *testing.T) {
	// when
	rec := serveReq(t, newTestServer(t).Handler(), languagePost(url.Values{"lang": {"klingon"}, "to": {"/"}}))

	// then
	require.Len(t, rec.Result().Cookies(), 1)
	assert.Equal(t, "en", rec.Result().Cookies()[0].Value)
}

func TestSetLanguage_RefusesUnsafeReturnTargets(t *testing.T) {
	h := newTestServer(t).Handler()
	for _, to := range []string{"", "https://evil.example", "//evil.example", `/\evil.example`, "/\t/evil.example", "/nope", "/../x", "javascript:alert(1)"} {
		t.Run(to, func(t *testing.T) {
			// when
			rec := serveReq(t, h, languagePost(url.Values{"lang": {"pl"}, "to": {to}}))

			// then
			assert.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, "/", rec.Header().Get("Location"))
		})
	}
}

func TestSetLanguage_RefusesCrossOrigin(t *testing.T) {
	// given a forged form post
	req := languagePost(url.Values{"lang": {"pl"}})
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	// when
	rec := serveReq(t, newTestServer(t).Handler(), req)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Result().Cookies())
}

func TestSetLanguage_MalformedFormIs400(t *testing.T) {
	// given a body the form parser refuses
	req := httptest.NewRequest(http.MethodPost, "/language", strings.NewReader("%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// when
	rec := serveReq(t, newTestServer(t).Handler(), req)

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNewSessionManager_CookieFlags(t *testing.T) {
	// when
	plain := NewSessionManager(Config{BaseURL: "http://localhost:8080"}, nil)
	secure := NewSessionManager(Config{BaseURL: "HTTPS://letter.example.com"}, nil)

	// then
	assert.Equal(t, "letter_session", plain.Cookie.Name)
	assert.True(t, plain.Cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, plain.Cookie.SameSite)
	assert.False(t, plain.Cookie.Secure)
	assert.True(t, secure.Cookie.Secure)
	assert.Equal(t, sessionLifetime, plain.Lifetime)
}

func TestShutdown_WithoutListenIsANoop(t *testing.T) {
	assert.NoError(t, newTestServer(t).Shutdown(context.Background()))
}

func TestListen_ServesUntilShutdown(t *testing.T) {
	// given
	s := newTestServer(t)
	s.cfg.Addr = "127.0.0.1:0"
	serve, err := s.Listen()
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- serve() }()

	// when
	resp, err := http.Get("http://" + s.Addr() + "/")
	require.NoError(t, err)
	_ = resp.Body.Close()
	shutdownErr := s.Shutdown(context.Background())

	// then
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, shutdownErr)
	assert.NoError(t, <-served)
}

func TestListen_FailsOnABusyPort(t *testing.T) {
	// given
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	s := newTestServer(t)
	s.cfg.Addr = listener.Addr().String()

	// when
	_, err = s.Listen()

	// then
	assert.Error(t, err)
}

func TestLanding_RendersTheLandingTool(t *testing.T) {
	// given
	h := newTestServer(t).WithLandingTool(templ.Raw(`<form id="loot-main"></form>`)).Handler()

	// when
	rec := serveReq(t, h, htmlGet("/"))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `<form id="loot-main"></form>`)
	assert.Contains(t, rec.Body.String(), "<title>TibiaLoot.com: Tibia loot calculator for your party hunts</title>")
}

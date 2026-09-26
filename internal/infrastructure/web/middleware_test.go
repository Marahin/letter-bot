package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// sentinel marks that a wrapped handler ran.
const sentinel = "HANDLER_RAN"

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sentinel))
}

// newTestDeps is a Deps over an in-memory session store and a no-op logger.
func newTestDeps(t *testing.T) *Deps {
	t.Helper()
	cfg := Config{BaseURL: "http://localhost:8080"}
	return &Deps{Cfg: cfg, Log: zap.NewNop().Sugar(), Sessions: NewSessionManager(cfg, nil)}
}

// loadedReq is a GET request whose context carries a loaded session.
func loadedReq(t *testing.T, sessions *scs.SessionManager, target string) *http.Request {
	t.Helper()
	ctx, err := sessions.Load(context.Background(), "")
	require.NoError(t, err)
	return httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
}

func TestLogMiddleware_PassesThroughStatusAndLogsPages(t *testing.T) {
	// given a handler that writes a non-200 status
	d := newTestDeps(t)
	core, logs := observer.New(zapcore.InfoLevel)
	d.Log = zap.New(core).Sugar()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	rec := httptest.NewRecorder()

	// when
	d.LogMiddleware(next).ServeHTTP(rec, loadedReq(t, d.Sessions, "/x"))

	// then the wrapped status reaches the client and the log
	assert.Equal(t, http.StatusTeapot, rec.Code)
	entries := logs.FilterMessage("request").All()
	require.Len(t, entries, 1)
	assert.Equal(t, int64(http.StatusTeapot), entries[0].ContextMap()["status"])
}

func TestLogMiddleware_SkipsAssets(t *testing.T) {
	// given
	d := newTestDeps(t)
	core, logs := observer.New(zapcore.InfoLevel)
	d.Log = zap.New(core).Sugar()

	// when
	d.LogMiddleware(http.HandlerFunc(okHandler)).ServeHTTP(httptest.NewRecorder(), loadedReq(t, d.Sessions, "/assets/app.css"))

	// then
	assert.Zero(t, logs.Len())
}

func TestStatusRecorder_Unwraps(t *testing.T) {
	// given
	inner := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: inner}

	// then
	assert.Same(t, inner, rec.Unwrap())
}

func TestRecoverMiddleware_TurnsPanicInto500(t *testing.T) {
	// given a handler that panics
	d := newTestDeps(t)
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept", "text/html")

	// when: it must not propagate the panic
	assert.NotPanics(t, func() {
		d.RecoverMiddleware(next).ServeHTTP(rec, req)
	})

	// then the branded page answers
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Something went wrong")
}

// csrfDeps returns a CSRF middleware over okHandler plus the warn log it writes.
func csrfDeps(t *testing.T, baseURL string) (http.Handler, *observer.ObservedLogs) {
	t.Helper()
	d := newTestDeps(t)
	d.Cfg.BaseURL = baseURL
	core, logs := observer.New(zapcore.WarnLevel)
	d.Log = zap.New(core).Sugar()
	return d.CSRFMiddleware(http.HandlerFunc(okHandler)), logs
}

// csrfPost returns a state-changing request carrying the given fetch metadata and
// Origin; an empty string omits the header.
func csrfPost(target, fetchSite, origin string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, nil)
	if fetchSite != "" {
		r.Header.Set("Sec-Fetch-Site", fetchSite)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

func TestCSRFMiddleware_SkipsSafeMethods(t *testing.T) {
	// given a hostile cross-site read: safe methods change nothing, so they pass
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			h, _ := csrfDeps(t, "http://localhost:8080")
			req := httptest.NewRequest(method, "/dashboard", nil)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			req.Header.Set("Origin", "https://evil.example")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), sentinel)
		})
	}
}

func TestCSRFMiddleware_AllowsSameOriginFetchMetadata(t *testing.T) {
	// "none" is a direct navigation (a bookmarked form post): no initiator to distrust
	for _, site := range []string{"same-origin", "Same-Origin", "none"} {
		t.Run(site, func(t *testing.T) {
			h, _ := csrfDeps(t, "http://localhost:8080")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, csrfPost("/logout", site, "http://localhost:8080"))

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), sentinel)
		})
	}
}

func TestCSRFMiddleware_RefusesCrossOrigin(t *testing.T) {
	cases := []struct {
		name      string
		fetchSite string
		origin    string
	}{
		// same-site, not same-origin: a sibling subdomain is refused too, which is
		// exactly what SameSite=Lax on the cookie would have let through.
		{"same-site sibling subdomain", "same-site", "http://evil.localhost:8080"},
		{"cross-site", "cross-site", "https://evil.example"},
		{"no fetch metadata, foreign origin", "", "https://evil.example"},
		{"no fetch metadata, subdomain origin", "", "http://evil.localhost:8080"},
		{"no fetch metadata, opaque origin", "", "null"},
		{"no fetch metadata, right host wrong scheme", "", "https://localhost:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, logs := csrfDeps(t, "http://localhost:8080")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, csrfPost("/servers/1/spots", tc.fetchSite, tc.origin))

			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.NotContains(t, rec.Body.String(), sentinel)
			entries := logs.FilterMessage("refused cross-origin request").All()
			require.Len(t, entries, 1)
			fields := entries[0].ContextMap()
			assert.Equal(t, http.MethodPost, fields["method"])
			assert.Equal(t, "/servers/1/spots", fields["path"])
			assert.Equal(t, tc.origin, fields["origin"])
		})
	}
}

// TestCSRFMiddleware_RefusesHeaderlessRequest pins the deliberate decision: a
// non-GET carrying neither Sec-Fetch-Site nor Origin is refused. Every browser that
// can hold a session sends at least one; the bot and worker never call the web app.
func TestCSRFMiddleware_RefusesHeaderlessRequest(t *testing.T) {
	h, logs := csrfDeps(t, "http://localhost:8080")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, csrfPost("/logout", "", ""))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.NotContains(t, rec.Body.String(), sentinel)
	assert.Equal(t, 1, logs.FilterMessage("refused cross-origin request").Len())
}

func TestCSRFMiddleware_OriginFallbackMatches(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		origin  string
	}{
		{"exact match", "http://localhost:8080", "http://localhost:8080"},
		{"explicit default port", "https://letter.example.com", "https://letter.example.com:443"},
		{"configured default port", "http://letter.example.com:80", "http://letter.example.com"},
		{"case-insensitive host", "https://letter.example.com", "https://LETTER.Example.COM"},
		{"trailing path on the base URL", "https://letter.example.com/", "https://letter.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := csrfDeps(t, tc.baseURL)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, csrfPost("/logout", "", tc.origin))

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), sentinel)
		})
	}
}

// TestCSRFMiddleware_UnusableBaseURL covers the misconfigured deployment: with no
// comparable origin the fallback can only refuse, but a real browser still passes on
// its fetch metadata, so the app does not lock itself out.
func TestCSRFMiddleware_UnusableBaseURL(t *testing.T) {
	for _, baseURL := range []string{"", "localhost:8080"} {
		t.Run("base URL "+strconv.Quote(baseURL), func(t *testing.T) {
			h, _ := csrfDeps(t, baseURL)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, csrfPost("/logout", "same-origin", ""))
			assert.Equal(t, http.StatusOK, rec.Code, "fetch metadata still vouches for the request")

			rec = httptest.NewRecorder()
			h.ServeHTTP(rec, csrfPost("/logout", "", "http://localhost:8080"))
			assert.Equal(t, http.StatusForbidden, rec.Code, "nothing to compare the Origin against")
		})
	}
}

// TestCSRFMiddleware_WarnsOnBaseURLMismatch covers the deployment gotcha: the
// browser vouches for a same-origin request while the Origin it sent disagrees with
// WEB_BASE_URL, so the fallback path is quietly broken behind the proxy. The request
// still passes; only a distinct config warning is written.
func TestCSRFMiddleware_WarnsOnBaseURLMismatch(t *testing.T) {
	h, logs := csrfDeps(t, "http://web:8080")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, csrfPost("/logout", "same-origin", "https://letter.example.com"))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), sentinel)
	assert.Zero(t, logs.FilterMessage("refused cross-origin request").Len(), "a config smell is not a refusal")
	entries := logs.FilterMessageSnippet("check WEB_BASE_URL").All()
	require.Len(t, entries, 1)
	assert.Equal(t, "https://letter.example.com", entries[0].ContextMap()["origin"])
	assert.Equal(t, "http://web:8080", entries[0].ContextMap()["expected_origin"])
}

func TestCSRFMiddleware_QuietWhenOriginAgrees(t *testing.T) {
	h, logs := csrfDeps(t, "https://letter.example.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, csrfPost("/logout", "same-origin", "https://letter.example.com:443"))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Zero(t, logs.Len(), "a matching origin must not warn")
}

func TestOriginOf(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"http://localhost:8080", "http://localhost:8080"},
		{"HTTP://LocalHost:8080", "http://localhost:8080"},
		{"https://letter.example.com/", "https://letter.example.com"},
		{"https://letter.example.com:443", "https://letter.example.com"},
		{"http://letter.example.com:80", "http://letter.example.com"},
		{"  http://letter.example.com  ", "http://letter.example.com"},
		// the port is only dropped when it is that scheme's default
		{"https://letter.example.com:80", "https://letter.example.com:80"},
		{"http://[::1]:80", "http://[::1]"},
		{"http://[::1]:8080", "http://[::1]:8080"},
		{"", ""},
		{"null", ""},
		{"localhost:8080", ""},
		{"://nope", ""},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, originOf(tc.raw), "originOf(%q)", tc.raw)
	}
}

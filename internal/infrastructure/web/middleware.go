package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ctxKey namespaces request-context values set by the web middleware.
type ctxKey int

const (
	ctxRangeSelection ctxKey = iota // RangeSelection resolved by WithRangeSelection
	ctxUser                         // *webuser.User loaded by RequireAuth
	ctxCurrentAccess                // access.GuildAccess resolved by the guild guards
)

// guildIDPathValue is the wildcard every guild-scoped route carries the guild id in
// ("GET /servers/{id}/...").
const guildIDPathValue = "id"

// statusRecorder captures the response status for access logging.
type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the optional interfaces (Flusher, ...)
// of the wrapped writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// LogMiddleware logs each request with method, path, status and duration. Static
// asset requests are skipped: a single page pulls in a dozen scripts and icons,
// which drowns the log without adding signal.
func (d *Deps) LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if isAssetPath(r.URL.Path) {
			return
		}
		d.Log.Infow("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

func isAssetPath(path string) bool {
	return strings.HasPrefix(path, assetsPrefix)
}

// RecoverMiddleware turns a handler panic into a logged 500 rather than a crash.
func (d *Deps) RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				d.Log.Errorw("panic recovered", "error", rec, "path", r.URL.Path)
				d.serverErrorPage(w, r)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CSRFMiddleware refuses a state-changing request that is not provably same-origin.
// Sec-Fetch-Site carries the weight: it is a forbidden header name, so only the
// browser can set it, and "same-site" is refused too because the cookie's
// SameSite=Lax already admits a sibling subdomain. The Origin comparison is reached
// only without fetch metadata, so a wrong WEB_BASE_URL breaks that fallback alone.
// Neither header present is a deliberate 403.
func (d *Deps) CSRFMiddleware(next http.Handler) http.Handler {
	expected := originOf(d.Cfg.BaseURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		site, origin := r.Header.Get("Sec-Fetch-Site"), r.Header.Get("Origin")
		if !sameOriginRequest(site, origin, expected) {
			d.Log.Warnw("refused cross-origin request",
				"method", r.Method, "path", r.URL.Path,
				"origin", origin, "fetch_site", site, "expected_origin", expected)
			// Plain text: outside LoadAndSave a rendered page would show a signed-out
			// shell, and a forged request needs no page.
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		d.warnOriginMismatch(site, origin, expected)
		next.ServeHTTP(w, r)
	})
}

// warnOriginMismatch surfaces what the fetch-metadata pass would otherwise hide: the
// browser vouches for a same-origin request whose Origin disagrees with the
// configured base URL, so WEB_BASE_URL is not the public origin. Refuses nothing.
func (d *Deps) warnOriginMismatch(fetchSite, origin, expected string) {
	if !strings.EqualFold(fetchSite, "same-origin") || origin == "" || expected == "" {
		return
	}
	if got := originOf(origin); got != expected {
		d.Log.Warnw("base URL does not match the request origin, check WEB_BASE_URL behind the proxy",
			"origin", got, "expected_origin", expected)
	}
}

// sameOriginRequest decides a state-changing request. Fetch metadata wins when the
// browser sent it ("none" is a direct navigation, no initiator to distrust);
// without it the Origin header must equal our own origin exactly, so a subdomain
// is refused along with everything else.
func sameOriginRequest(fetchSite, origin, expected string) bool {
	switch strings.ToLower(fetchSite) {
	case "same-origin", "none":
		return true
	case "same-site", "cross-site":
		return false
	}
	if origin == "" || expected == "" {
		return false
	}
	return originOf(origin) == expected
}

// originOf reduces a URL to its comparable scheme://host[:port], lowercased with a
// default port dropped, so an Origin header and a configured base URL that spell
// the same origin differently still match. Empty when there is no usable
// scheme+host (an unset or scheme-less WEB_BASE_URL, or a literal "null" Origin).
func originOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	scheme, host := strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	// TrimSuffix, not u.Hostname(), so an IPv6 literal keeps its brackets.
	switch scheme {
	case "http":
		host = strings.TrimSuffix(host, ":80")
	case "https":
		host = strings.TrimSuffix(host, ":443")
	}
	return scheme + "://" + host
}

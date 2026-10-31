package web

import (
	"net/http"
	"net/url"
	"path"

	"spot-assistant/internal/infrastructure/i18n"
)

// WithLocale resolves the request's language into the context, where views read it
// via i18n.From. Mounted outermost: RecoverMiddleware renders its 500 from the
// request it was handed, so a locale added further in would not reach that page.
func (d *Deps) WithLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAssetPath(r.URL.Path) {
			// Add, not Set: scs adds "Cookie" further in.
			w.Header().Add("Vary", "Accept-Language")
		}
		next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), d.resolveLocale(r))))
	})
}

// resolveLocale applies ?lang= > cookie > Accept-Language > English. ?lang= is not
// persisted: a GET must not write.
func (d *Deps) resolveLocale(r *http.Request) i18n.Locale {
	if l, ok := i18n.Lookup(langOverride(r)); ok {
		return l
	}
	if l, ok := i18n.Lookup(d.LanguageCookie(r)); ok {
		return l
	}
	return i18n.Match(r.Header.Get("Accept-Language"))
}

// langOverride reads ?lang=, falling back to the query of the page an htmx request
// came from: hx-post URLs carry no query of their own, so without this a fragment
// answers a ?lang=pl page in the cookie's language. The value only ever selects
// from the supported set, so a forged header picks a language and nothing else.
func langOverride(r *http.Request) string {
	if v := r.URL.Query().Get("lang"); v != "" {
		return v
	}
	if cur, err := url.Parse(r.Header.Get("HX-Current-URL")); err == nil {
		return cur.Query().Get("lang")
	}
	return ""
}

// SetLanguage records an explicit switch in the language cookie.
func (d *Deps) SetLanguage(w http.ResponseWriter, code string) i18n.Locale {
	loc := i18n.Normalize(code)
	d.SetLanguageCookie(w, loc.Code())
	return loc
}

// safeReturnTo is where a language switch sends the visitor back to. `to` comes
// from a form field, so it is an open-redirect surface: anything that is not a
// registered GET page of our own is refused rather than repaired. The query string
// survives when the path validates, minus ?lang=, which would override the pick.
func (d *Deps) safeReturnTo(to string) string {
	const home = "/"
	if d.Routes == nil || !isLocalURL(to) {
		return home
	}
	u, err := url.Parse(to)
	if err != nil {
		return home
	}
	if !d.isPage(u.Path) {
		return home
	}
	q := u.Query()
	q.Del("lang")
	u.RawQuery = q.Encode()
	return u.RequestURI()
}

// isPage reports whether p is a registered GET page of our own, as written. The
// catch-all "GET /" matches any unclaimed path, so a route match alone would wave
// through anything same-origin. "/" itself resolves to the landing route ("/{$}"),
// so home stays reachable.
func (d *Deps) isPage(p string) bool {
	if path.Clean(p) != p {
		return false
	}
	route := d.Routes.Lookup(http.MethodGet, p)
	return route != nil && route.Pattern() != "/"
}

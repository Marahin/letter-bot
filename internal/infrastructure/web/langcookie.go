package web

import (
	"net/http"
	"strings"
)

// langCookieName holds the visitor's chosen interface language. Unlike the
// per-server letter_range_<id>, it is one long-lived cookie for every server: the
// choice must survive closing the window and signing out, which is the point of it
// (an anonymous visitor has nowhere else to keep it). HttpOnly because only the
// server reads it.
const langCookieName = "letter_lang"

// langCookieMaxAge is one year: a language is a standing preference, not session state.
const langCookieMaxAge = 31536000

// LanguageCookie returns the stored language code, or "" when none is set. The
// value is untrusted, so callers resolve it through i18n.Lookup rather than
// believing it.
func (d *Deps) LanguageCookie(r *http.Request) string {
	c, err := r.Cookie(langCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

// SetLanguageCookie persists code as the visitor's language for a year. A blank
// code clears it, so the next request falls back to Accept-Language.
func (d *Deps) SetLanguageCookie(w http.ResponseWriter, code string) {
	c := &http.Cookie{
		Name:     langCookieName,
		Value:    code,
		Path:     "/",
		MaxAge:   langCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(d.Cfg.BaseURL),
	}
	if code == "" {
		c.MaxAge = -1 // delete
	}
	http.SetCookie(w, c)
}

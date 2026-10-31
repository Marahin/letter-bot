package web

import (
	"net/http"
	"strings"
)

// guildCookieName scopes a selection cookie to one server by name, not by path.
func guildCookieName(name, guildID string) string { return name + "_" + guildID }

// guildCookie reads the request's server selection cookie, trimmed, or "" when
// none is set.
func guildCookie(r *http.Request, name string) string {
	c, err := r.Cookie(guildCookieName(name, r.PathValue(guildIDPathValue)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

// setGuildCookie stores a selection for guildID, so the server's views share it
// without leaking across servers. It carries no
// Expires/Max-Age, so it is a session cookie: the choice clears when the browser
// window closes. A blank value clears it now.
func (d *Deps) setGuildCookie(w http.ResponseWriter, name, guildID, value string) {
	c := &http.Cookie{
		Name:     guildCookieName(name, guildID),
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(d.Cfg.BaseURL),
	}
	if value == "" {
		c.MaxAge = -1 // delete
	}
	http.SetCookie(w, c)
}

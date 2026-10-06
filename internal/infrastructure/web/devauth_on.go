//go:build devauth

package web

import (
	"html"
	"net/http"
	"net/url"
	"strings"

	"spot-assistant/internal/infrastructure/devauth"
)

// registerDevAuth wires the dev-only mock-user sign-in, so a developer and the
// e2e suite reach the signed-in pages without Discord. Besides the build tag,
// each request needs WEB_DEV_AUTH and a localhost base URL; webapp wires the
// mock OAuth port, which accepts the dev codes, only under the same conditions.
func registerDevAuth(s *Server, router *Router) {
	router.Get("/dev/login", s.handleDevLoginList)
	router.Get("/dev/login/{userID}", s.handleDevLogin)
}

// handleDevLoginList is a plain list of the mock users, English only.
func (s *Server) handleDevLoginList(w http.ResponseWriter, r *http.Request) {
	if !s.devAuthAllowed(w) {
		return
	}
	suffix := ""
	if to := r.URL.Query().Get("to"); to != "" {
		suffix = "?to=" + url.QueryEscape(to)
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Dev login</title></head><body>`)
	b.WriteString(`<h1>Dev login</h1>`)
	b.WriteString(`<p style="color:#b00"><strong>Development mock authentication.</strong> `)
	b.WriteString(`This bypass exists only in dev and test builds and must never be reachable in production.</p><ul>`)
	for _, u := range devauth.Users {
		b.WriteString(`<li><a data-dev-user="` + html.EscapeString(u.ID) + `" href="/dev/login/` + html.EscapeString(u.ID) + html.EscapeString(suffix) + `">`)
		b.WriteString(html.EscapeString(u.Label) + " (" + html.EscapeString(u.Username) + ")</a></li>")
	}
	b.WriteString(`</ul></body></html>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// handleDevLogin signs in as the chosen mock user through the same Auth.Complete
// and session tail as the OAuth callback.
func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.devAuthAllowed(w) {
		return
	}
	d := s.deps()
	ctx := r.Context()
	mock, ok := devauth.ByID(r.PathValue("userID"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, err := d.Auth.Complete(ctx, devauth.DevCodePrefix+mock.ID)
	if err != nil {
		d.ServerError(w, r, "dev login", err)
		return
	}
	if err := s.sessions.RenewToken(ctx); err != nil {
		d.ServerError(w, r, "renew session", err)
		return
	}
	s.sessions.Put(ctx, SessionUserKey, user.DiscordUserID)
	http.Redirect(w, r, loginDestination(d.safeReturnTo(r.URL.Query().Get("to"))), http.StatusSeeOther)
}

// devAuthAllowed answers 404 and false unless WEB_DEV_AUTH is on and the base URL
// is local, so a tagged binary pointed at a real origin still refuses.
func (s *Server) devAuthAllowed(w http.ResponseWriter) bool {
	if !s.cfg.DevAuth || !devauth.IsDevBaseURL(s.cfg.BaseURL) {
		http.Error(w, "not found", http.StatusNotFound)
		return false
	}
	return true
}

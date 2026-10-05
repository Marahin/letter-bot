package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"

	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/ports"
)

func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	d.Render(w, r, Landing(s.cfg.BaseURL, d.InviteURLGeneric(), d.MarketingNav(r), s.landingTool))
}

// handleNotFound serves the branded 404 for any path no route claimed. Registered
// as the "GET /" catch-all; more specific routes still win.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.deps().NotFound(w, r)
}

// handleSetLanguage records the language picked in the shell's picker. Not behind
// sign-in: for an anonymous visitor the cookie is the only carrier there is.
func (s *Server) handleSetLanguage(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	if !ParseForm(w, r) {
		return
	}
	d.SetLanguage(w, r.PostFormValue("lang"))
	http.Redirect(w, r, d.safeReturnTo(r.PostFormValue("to")), http.StatusSeeOther)
}

const (
	sessionStateKey = "oauth_state"
	// sessionLoginReturnKey holds the page /login?to= named, for the callback to land on.
	sessionLoginReturnKey = "login_return_to"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	to := d.safeReturnTo(r.URL.Query().Get("to"))
	if d.SessionUserID(r.Context()) != "" {
		http.Redirect(w, r, loginDestination(to), http.StatusSeeOther)
		return
	}
	state, err := RandomState()
	if err != nil {
		d.ServerError(w, r, "generate state", err)
		return
	}
	s.sessions.Put(r.Context(), sessionStateKey, state)
	// A stale return path from an abandoned attempt must not be honoured.
	if to != "/" {
		s.sessions.Put(r.Context(), sessionLoginReturnKey, to)
	} else {
		s.sessions.Remove(r.Context(), sessionLoginReturnKey)
	}
	http.Redirect(w, r, d.Auth.LoginURL(state), http.StatusSeeOther)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	ctx := r.Context()
	wantState := s.sessions.PopString(ctx, sessionStateKey)
	gotState := r.URL.Query().Get("state")
	if wantState == "" || subtle.ConstantTimeCompare([]byte(gotState), []byte(wantState)) != 1 {
		d.BadRequest(w, r, i18n.T(ctx, "auth.error.state"))
		return
	}
	// The user pressed Cancel on Discord's consent screen.
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		d.BadRequest(w, r, i18n.T(ctx, "auth.error.state"))
		return
	}
	user, err := d.Auth.Complete(ctx, code)
	switch {
	case errors.Is(err, ports.ErrUnauthorized):
		d.Log.Infow("discord refused the oauth code", "error", err)
		d.BadRequest(w, r, i18n.T(ctx, "auth.error.state"))
		return
	case errors.Is(err, ports.ErrUpstreamUnavailable):
		d.Unavailable(w, r, "complete oauth", err)
		return
	case err != nil:
		d.ServerError(w, r, "complete oauth", err)
		return
	}
	// A fresh token stops a session id planted before sign-in from riding along.
	if err := s.sessions.RenewToken(ctx); err != nil {
		d.ServerError(w, r, "renew session", err)
		return
	}
	s.sessions.Put(ctx, SessionUserKey, user.DiscordUserID)
	d.Log.Infow("user signed in", "user_id", user.DiscordUserID)
	http.Redirect(w, r, loginDestination(s.sessions.PopString(ctx, sessionLoginReturnKey)), http.StatusSeeOther)
}

// loginDestination is where a sign-in lands: the validated return path, else the dashboard.
func loginDestination(to string) string {
	if to == "" || to == "/" || !isLocalURL(to) {
		return "/dashboard"
	}
	return to
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	if userID := d.SessionUserID(r.Context()); userID != "" {
		if err := d.Auth.Logout(r.Context(), userID); err != nil {
			d.Log.Warnw("clear oauth token on logout", "error", err)
		}
	}
	if err := s.sessions.Destroy(r.Context()); err != nil {
		d.ServerError(w, r, "destroy session", err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	user, err := d.CurrentUser(r.Context())
	if err != nil {
		d.ServerError(w, r, "load current user", err)
		return
	}
	list, err := d.Access.AccessibleGuilds(r.Context(), user.DiscordUserID)
	if err != nil {
		d.accessError(w, r, err)
		return
	}
	nav := d.NavFromAccess(user, list, selectedGuildID(user, list))
	nav.ReturnTo = r.URL.RequestURI()
	d.Render(w, r, Dashboard(s.cfg.BaseURL, user, list, d.InviteURLGeneric(), nav))
}

// handleGuildRoot sends /servers/{id} to Reservations. The guard shows the lock
// page here for a server without premium.
func (s *Server) handleGuildRoot(w http.ResponseWriter, r *http.Request) {
	current, _ := CurrentAccessFrom(r.Context())
	http.Redirect(w, r, GuildPath(current.Config.GuildID, "/reservations"), http.StatusSeeOther)
}

// RandomState mints the CSRF state of an OAuth round trip.
func RandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

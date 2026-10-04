package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/ports"
)

// RequireAuth sends an anonymous visitor to /login, back to this page after
// sign-in when it is a GET. A session whose user row is gone is destroyed.
func (d *Deps) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := d.SessionUserID(r.Context())
		if userID == "" {
			d.redirectToLogin(w, r)
			return
		}
		user, err := d.Auth.User(r.Context(), userID)
		if errors.Is(err, ports.ErrNotFound) {
			d.signOutAndLogin(w, r)
			return
		}
		if err != nil {
			d.ServerError(w, r, "load current user", err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxUser, user)))
	}
}

func (d *Deps) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	target := "/login"
	if r.Method == http.MethodGet && !IsHTMX(r) {
		target += "?to=" + url.QueryEscape(r.URL.RequestURI())
	}
	if IsHTMX(r) {
		// htmx would swap the login redirect into a fragment; ask for a full navigation.
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// signOutAndLogin drops a session whose user or Discord token is no longer valid.
func (d *Deps) signOutAndLogin(w http.ResponseWriter, r *http.Request) {
	if err := d.Sessions.Destroy(r.Context()); err != nil {
		d.Log.Errorw("destroy session", "error", err)
	}
	d.redirectToLogin(w, r)
}

// RequireSiteAdmin answers 404, not 403, so the admin routes are never disclosed.
// It assumes RequireAuth ran first.
func (d *Deps) RequireSiteAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Access.IsSiteAdmin(d.SessionUserID(r.Context())) {
			d.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// RequireView guards a read-only server page.
func (d *Deps) RequireView(next http.HandlerFunc) http.HandlerFunc {
	return d.requireGuildAccess(access.TierView, next)
}

// RequireReserve guards the reservation mutations of a member with a reserve rank.
func (d *Deps) RequireReserve(next http.HandlerFunc) http.HandlerFunc {
	return d.requireGuildAccess(access.TierReserve, next)
}

// RequireManage guards respawn management and changes to other members' reservations.
func (d *Deps) RequireManage(next http.HandlerFunc) http.HandlerFunc {
	return d.requireGuildAccess(access.TierManage, next)
}

// RequireAdmin guards Settings and Channels (owner or Administrator).
func (d *Deps) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return d.requireGuildAccess(access.TierAdmin, next)
}

// requireGuildAccess resolves the {id} server once. A server the user may not
// view answers 404 (not 403), so nobody can probe which servers exist; a tier the
// user does not reach answers 403. It assumes RequireAuth ran first.
func (d *Deps) requireGuildAccess(tier access.Tier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := d.SessionUserID(r.Context())
		guildID := r.PathValue(guildIDPathValue)
		current, err := d.Access.Access(r.Context(), userID, guildID)
		if errors.Is(err, ports.ErrNotFound) {
			d.NotFound(w, r)
			return
		}
		if err != nil {
			d.accessError(w, r, err)
			return
		}
		if !current.Allows(tier) {
			d.Forbidden(w, r)
			return
		}
		d.rememberGuild(r.Context(), guildID)
		next(w, r.WithContext(context.WithValue(r.Context(), ctxCurrentAccess, *current)))
	}
}

// rememberGuild makes the routed server the user's default, so the next visit opens
// on it. It writes only on a change, and a failed write keeps the old default
// rather than failing the page.
func (d *Deps) rememberGuild(ctx context.Context, guildID string) {
	user, ok := ctx.Value(ctxUser).(*webuser.User)
	if !ok || user.DefaultGuildID == guildID {
		return
	}
	if err := d.Auth.SetDefaultGuild(ctx, user.DiscordUserID, guildID); err != nil {
		d.Log.Warnw("remember default guild", "error", err)
	}
}

// RequirePremium shows the "premium required" page on a server that is not
// premium. Site admins pass. It must run inside one of the guild guards.
func (d *Deps) RequirePremium(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, ok := d.MustAccess(w, r)
		if !ok {
			return
		}
		if current.Config.IsPremium() || d.Access.IsSiteAdmin(d.SessionUserID(r.Context())) {
			next(w, r)
			return
		}
		d.premiumRequired(w, r, current)
	}
}

func (d *Deps) premiumRequired(w http.ResponseWriter, r *http.Request, current access.GuildAccess) {
	if !wantsHTMLErrorPage(r) {
		http.Error(w, i18n.T(r.Context(), "premium.required.plain"), http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	if err := PremiumRequired(d.Cfg.BaseURL, current, d.Nav(r, current.Config.GuildID)).Render(r.Context(), w); err != nil {
		d.Log.Errorw("render premium required", "path", r.URL.Path, "error", err)
	}
}

// accessError maps a failed access resolution: a refused Discord token signs the
// user out, an unreachable Discord is a 503 to retry, anything else a 500.
func (d *Deps) accessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ports.ErrUnauthorized):
		d.Log.Infow("discord refused the stored token, signing out", "path", r.URL.Path)
		d.signOutAndLogin(w, r)
	case errors.Is(err, ports.ErrUpstreamUnavailable):
		d.Unavailable(w, r, "resolve access", err)
	default:
		d.ServerError(w, r, "resolve access", err)
	}
}

// CurrentAccessFrom returns the access the guild guard resolved for the {id} server.
func CurrentAccessFrom(ctx context.Context) (access.GuildAccess, bool) {
	v, ok := ctx.Value(ctxCurrentAccess).(access.GuildAccess)
	return v, ok
}

// errNoGuildGuard means a guild route was registered without its guard.
var errNoGuildGuard = errors.New("no guild access in context")

// MustAccess returns the access the guild guard resolved. Without one it answers
// 500 and returns false.
func (d *Deps) MustAccess(w http.ResponseWriter, r *http.Request) (access.GuildAccess, bool) {
	current, ok := CurrentAccessFrom(r.Context())
	if !ok {
		d.ServerError(w, r, "guild route without a guild guard", errNoGuildGuard)
	}
	return current, ok
}

// IsHTMX reports whether htmx sent the request.
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// WithCurrentAccess stores a resolved access in the context. Feature tests use it
// to call a handler without the guard.
func WithCurrentAccess(ctx context.Context, a access.GuildAccess) context.Context {
	return context.WithValue(ctx, ctxCurrentAccess, a)
}

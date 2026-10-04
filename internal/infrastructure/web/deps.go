package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/ports"
)

// SessionUserKey holds the signed-in Discord user id.
const SessionUserKey = "user_id"

// InviteBotPermissions is the bitfield requested in the bot invite link: View
// Channel, Send Messages, Manage Messages, Embed Links, Attach Files, Read Message
// History, Manage Channels and Manage Roles.
const InviteBotPermissions = "268561424"

// Deps carries the cross-cutting dependencies every feature handler needs. The
// server builds one and hands it to each feature package.
type Deps struct {
	Cfg          Config
	Log          *zap.SugaredLogger
	Sessions     *scs.SessionManager
	Auth         ports.AuthService
	Access       ports.GuildAccessService
	Premium      ports.PremiumService
	Settings     ports.GuildSettingsService
	Spots        ports.SpotService
	Reservations ports.ReservationService
	Stats        ports.StatsService
	Characters   ports.CharacterProfileService
	// Routes is nil in a Deps built without a server.
	Routes *Router
}

// PathInt64 parses a path value as an int64, writing a 400 and returning ok=false
// when it is not a valid integer.
func (d *Deps) PathInt64(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		d.BadRequest(w, r, "invalid "+name)
		return 0, false
	}
	return v, true
}

// ParseForm parses the request form, answering a plain 400 and reporting false when
// it is malformed. Plain text, not the branded error page: a malformed body is a
// broken client, not a page a user navigated to.
func ParseForm(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return false
	}
	return true
}

// Render writes a templ component as the HTML response.
func (d *Deps) Render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		d.Log.Errorw("render failed", "path", r.URL.Path, "error", err)
	}
}

// ServerError logs the cause and returns a branded 500 to the client.
func (d *Deps) ServerError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	d.Log.Errorw(msg, "path", r.URL.Path, "error", err)
	d.serverErrorPage(w, r)
}

// serverErrorPage is shared by ServerError and the panic recovery so the two
// cannot say different things.
func (d *Deps) serverErrorPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	d.RenderError(w, r, http.StatusInternalServerError,
		i18n.T(ctx, "error.server.title"), i18n.T(ctx, "error.server.message"), i18n.T(ctx, "error.server.plain"))
}

// Unavailable logs at warn and returns a 503 with a Retry-After hint, for when a
// driven adapter could not reach an external service. A transient upstream failure
// is not a bug in ours, so it is not logged as an error.
func (d *Deps) Unavailable(w http.ResponseWriter, r *http.Request, msg string, err error) {
	d.Log.Warnw(msg, "path", r.URL.Path, "error", err)
	w.Header().Set("Retry-After", "5")
	ctx := r.Context()
	d.RenderError(w, r, http.StatusServiceUnavailable,
		i18n.T(ctx, "error.unavailable.title"), i18n.T(ctx, "error.unavailable.message"), i18n.T(ctx, "error.unavailable.plain"))
}

// BadRequest returns a branded 400. message doubles as the plain-text body for
// non-HTML and htmx callers, and is the caller's to localize.
func (d *Deps) BadRequest(w http.ResponseWriter, r *http.Request, message string) {
	d.RenderError(w, r, http.StatusBadRequest, i18n.T(r.Context(), "error.bad_request.title"), message, message)
}

// Forbidden writes the branded 403.
func (d *Deps) Forbidden(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	msg := i18n.T(ctx, "error.forbidden.message")
	d.RenderError(w, r, http.StatusForbidden, i18n.T(ctx, "error.denied.title"), msg, msg)
}

// NotFound writes the branded 404. A guild outside the user's list answers this
// too, so a 403 never leaks which guilds exist.
func (d *Deps) NotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	d.RenderError(w, r, http.StatusNotFound,
		i18n.T(ctx, "error.not_found.title"), i18n.T(ctx, "error.not_found.message"), i18n.T(ctx, "error.not_found.plain"))
}

// RenderError writes the branded full-page ErrorPage for a plain (non-htmx) HTML
// navigation, and a plain-text plainMsg for everything else (htmx swaps, API
// clients, asset fetches). It never reveals which resource failed.
func (d *Deps) RenderError(w http.ResponseWriter, r *http.Request, status int, title, message, plainMsg string) {
	if !wantsHTMLErrorPage(r) {
		http.Error(w, plainMsg, status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := ErrorPage(d.Cfg.BaseURL, status, title, message, d.Nav(r, "")).Render(r.Context(), w); err != nil {
		d.Log.Errorw("render error page", "path", r.URL.Path, "status", status, "error", err)
	}
}

func wantsHTMLErrorPage(r *http.Request) bool {
	if IsHTMX(r) {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// sessionString reads a session value, "" when the request carries no loaded
// session. RecoverMiddleware sits outside LoadAndSave and renders its error page
// through Nav, and scs panics on a context without session data.
func (d *Deps) sessionString(ctx context.Context, key string) (v string) {
	if d.Sessions == nil {
		return ""
	}
	defer func() {
		if recover() != nil {
			v = ""
		}
	}()
	return d.Sessions.GetString(ctx, key)
}

// SessionUserID is the signed-in Discord user id, "" for an anonymous visitor.
func (d *Deps) SessionUserID(ctx context.Context) string {
	return d.sessionString(ctx, SessionUserKey)
}

// CurrentUser returns the signed-in user: the one RequireAuth loaded, or a fresh
// lookup. It returns ports.ErrNotFound for an anonymous visitor.
func (d *Deps) CurrentUser(ctx context.Context) (*webuser.User, error) {
	if u, ok := ctx.Value(ctxUser).(*webuser.User); ok {
		return u, nil
	}
	id := d.SessionUserID(ctx)
	if id == "" || d.Auth == nil {
		return nil, ports.ErrNotFound
	}
	return d.Auth.User(ctx, id)
}

// Nav builds the shell navigation state for the current request. An anonymous
// visitor gets the signed-out Nav. A failed server list degrades to a switcher
// without servers rather than failing the page.
func (d *Deps) Nav(r *http.Request, currentGuildID string) Nav {
	ctx := r.Context()
	signedOut := Nav{CurrentGuildID: currentGuildID, ReturnTo: r.URL.RequestURI()}
	user, err := d.CurrentUser(ctx)
	if err != nil {
		if !errors.Is(err, ports.ErrNotFound) {
			d.Log.Warnw("nav: load current user", "error", err)
		}
		return signedOut
	}
	var list []access.GuildAccess
	if d.Access != nil {
		if list, err = d.Access.AccessibleGuilds(ctx, user.DiscordUserID); err != nil {
			d.Log.Warnw("nav: list accessible guilds", "error", err)
			list = nil
		}
	}
	if currentGuildID == "" {
		currentGuildID = selectedGuildID(user, list)
	}
	n := d.NavFromAccess(user, list, currentGuildID)
	if current, ok := CurrentAccessFrom(ctx); ok && current.Config.GuildID == currentGuildID {
		n.applyAccess(current)
	}
	n.ReturnTo = signedOut.ReturnTo
	return n
}

// NavFromAccess assembles a signed-in Nav from an already-resolved access list.
func (d *Deps) NavFromAccess(user *webuser.User, list []access.GuildAccess, currentGuildID string) Nav {
	n := Nav{
		Authenticated:  true,
		Username:       user.DisplayName(),
		SiteAdmin:      d.Access != nil && d.Access.IsSiteAdmin(user.DiscordUserID),
		CurrentGuildID: currentGuildID,
	}
	for _, a := range list {
		n.Servers = append(n.Servers, NavServer{ID: a.Config.GuildID, Name: a.Config.Name, Icon: a.Config.Icon})
		if a.Config.GuildID == currentGuildID {
			n.applyAccess(a)
		}
	}
	return n
}

// selectedGuildID is the server a guild-less page shows as selected: the remembered
// default while still accessible, else the first premium server, else the first.
func selectedGuildID(user *webuser.User, list []access.GuildAccess) string {
	if user != nil && user.DefaultGuildID != "" {
		for _, a := range list {
			if a.Config.GuildID == user.DefaultGuildID {
				return a.Config.GuildID
			}
		}
	}
	for _, a := range list {
		if a.Config.IsPremium() {
			return a.Config.GuildID
		}
	}
	if len(list) > 0 {
		return list[0].Config.GuildID
	}
	return ""
}

// MarketingNav is a Nav for the marketing landing page: the shell renders the top
// bar (not the app sidebar) even for a signed-in visitor.
func (d *Deps) MarketingNav(r *http.Request) Nav {
	n := d.Nav(r, "")
	n.Marketing = true
	n.Wide = true
	return n
}

// InviteURL builds the Discord bot invite link for a specific guild.
func (d *Deps) InviteURL(guildID string) string {
	q := d.inviteQuery()
	q.Set("guild_id", guildID)
	q.Set("disable_guild_select", "true")
	return "https://discord.com/oauth2/authorize?" + q.Encode()
}

// InviteURLGeneric builds a bot invite without a fixed guild, so Discord shows the
// server picker.
func (d *Deps) InviteURLGeneric() string {
	return "https://discord.com/oauth2/authorize?" + d.inviteQuery().Encode()
}

func (d *Deps) inviteQuery() url.Values {
	q := url.Values{}
	q.Set("client_id", d.Cfg.Discord.ClientID)
	q.Set("scope", "bot applications.commands")
	q.Set("permissions", InviteBotPermissions)
	return q
}

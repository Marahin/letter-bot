package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/i18n"
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
	Cfg      Config
	Log      *zap.SugaredLogger
	Sessions *scs.SessionManager
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
	if r.Header.Get("HX-Request") == "true" {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// Nav builds the shell navigation state for the current request. Sign-in arrives in
// a later phase; until then every visitor gets the signed-out Nav.
func (d *Deps) Nav(r *http.Request, currentGuildID string) Nav {
	return Nav{CurrentGuildID: currentGuildID, ReturnTo: r.URL.RequestURI()}
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

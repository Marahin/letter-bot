package adminhttp

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/premium"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/ports"
)

type Handlers struct {
	D *web.Deps
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d} }

func (h *Handlers) HandleGuilds(w http.ResponseWriter, r *http.Request) {
	guilds, err := h.D.Premium.List(r.Context())
	if err != nil {
		h.D.ServerError(w, r, "list guilds", err)
		return
	}
	sortGuilds(guilds)
	nav := h.D.Nav(r, "")
	nav.Active = "admin-guilds"
	h.D.Render(w, r, Guilds(h.D.Cfg.BaseURL, guilds, nav))
}

// HandleSetPremium turns premium on or off (form field premium=true|false). An
// htmx request gets the updated table row, a plain form post a redirect back.
func (h *Handlers) HandleSetPremium(w http.ResponseWriter, r *http.Request) {
	if !web.ParseForm(w, r) {
		return
	}
	// These refusals stay English: the UI never submits such a value.
	var on bool
	switch r.PostFormValue("premium") {
	case "true":
		on = true
	case "false":
	default:
		http.Error(w, "invalid premium value", http.StatusBadRequest)
		return
	}
	guildID := r.PathValue("id")
	err := h.D.Premium.SetPremium(r.Context(), guildID, on)
	switch {
	case errors.Is(err, ports.ErrNotFound):
		h.D.NotFound(w, r)
		return
	case errors.Is(err, premium.ErrPremiumForever):
		msg := i18n.T(r.Context(), "admin.guilds.error.forever")
		h.D.RenderError(w, r, http.StatusConflict, i18n.T(r.Context(), "error.bad_request.title"), msg, msg)
		return
	case err != nil:
		h.D.ServerError(w, r, "set premium", err)
		return
	}
	h.D.Log.Infow("admin action", "action", "set-premium", "actor", h.D.SessionUserID(r.Context()), "guild_id", guildID, "premium", on)

	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/admin/guilds", http.StatusSeeOther)
		return
	}
	guilds, err := h.D.Premium.List(r.Context())
	if err != nil {
		h.D.ServerError(w, r, "list guilds", err)
		return
	}
	for _, g := range guilds {
		if g.GuildID == guildID {
			h.D.Render(w, r, guildRow(g))
			return
		}
	}
	h.D.NotFound(w, r)
}

// sortGuilds puts bot-present servers first, then orders by name.
func sortGuilds(guilds []*guildconfig.Config) {
	sort.SliceStable(guilds, func(i, j int) bool {
		if guilds[i].BotPresent != guilds[j].BotPresent {
			return guilds[i].BotPresent
		}
		return strings.ToLower(guilds[i].Name) < strings.ToLower(guilds[j].Name)
	})
}

func premiumFormValue(g *guildconfig.Config) string {
	if g.Premium {
		return "false"
	}
	return "true"
}

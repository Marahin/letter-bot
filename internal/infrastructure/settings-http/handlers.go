package settingshttp

import (
	"errors"
	"net/http"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/infrastructure/guildsettings"
	"spot-assistant/internal/infrastructure/web"
)

type Handlers struct {
	D *web.Deps
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d} }

func (h *Handlers) HandleSettings(w http.ResponseWriter, r *http.Request) {
	current, ok := web.CurrentAccessFrom(r.Context())
	if !ok {
		h.D.ServerError(w, r, "settings without a guild guard", errors.New("no guild access in context"))
		return
	}
	guildID := current.Config.GuildID
	roles, err := h.D.Settings.Roles(r.Context(), guildID)
	if err != nil {
		h.D.ServerError(w, r, "list roles", err)
		return
	}
	world, err := h.D.Settings.World(r.Context(), guildID)
	if err != nil {
		h.D.ServerError(w, r, "load world", err)
		return
	}
	nav := h.D.Nav(r, guildID)
	nav.Active = "settings"
	h.D.Render(w, r, Settings(h.D.Cfg.BaseURL, current.Config, roles, world, h.D.InviteURL(guildID), nav))
}

// HandleSetRanks stores one rank list. Unchecked boxes are absent from the form,
// so no role_ids at all clears the list. Every refusal answers 200 with a status
// fragment: htmx does not swap a 4xx body.
func (h *Handlers) HandleSetRanks(w http.ResponseWriter, r *http.Request) {
	kind := guildconfig.RoleKind(r.PathValue("kind"))
	if !kind.Valid() {
		h.D.NotFound(w, r)
		return
	}
	if !web.ParseForm(w, r) {
		return
	}
	err := h.D.Settings.SetRoleIDs(r.Context(), r.PathValue("id"), kind, r.PostForm["role_ids"])
	switch {
	case errors.Is(err, guildsettings.ErrUnknownRole):
		h.D.Render(w, r, StaleStatus())
	case err != nil:
		h.D.ServerError(w, r, "set "+string(kind)+" ranks", err)
	default:
		h.D.Render(w, r, RanksStatus(kind))
	}
}

func (h *Handlers) HandleSetWorld(w http.ResponseWriter, r *http.Request) {
	if !web.ParseForm(w, r) {
		return
	}
	err := h.D.Settings.SetWorld(r.Context(), r.PathValue("id"), r.PostFormValue("world"))
	switch {
	case errors.Is(err, guildsettings.ErrUnknownWorld):
		h.D.Render(w, r, WorldInvalidStatus())
	case err != nil:
		h.D.ServerError(w, r, "set world", err)
	default:
		h.D.Render(w, r, WorldStatus())
	}
}

// HandleRefresh backs the "Refresh server data" button: it asks the bot to sync
// the channels and roles again, or reports the cooldown.
func (h *Handlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	retryAfter, err := h.D.Settings.RequestResync(r.Context(), r.PathValue("id"))
	if err != nil {
		h.D.ServerError(w, r, "request resync", err)
		return
	}
	if retryAfter > 0 {
		h.D.Render(w, r, RefreshCooldown(retryAfter))
		return
	}
	h.D.Render(w, r, RefreshStatus())
}

package channelshttp

import (
	"errors"
	"net/http"
	"strings"

	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/ports"
)

type Handlers struct {
	D *web.Deps
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d} }

func (h *Handlers) HandleChannels(w http.ResponseWriter, r *http.Request) {
	current, ok := h.D.MustAccess(w, r)
	if !ok {
		return
	}
	channels, err := h.D.Settings.Channels(r.Context(), current.Config.GuildID)
	if err != nil {
		h.D.ServerError(w, r, "list channels", err)
		return
	}
	nav := h.D.Nav(r, current.Config.GuildID)
	nav.Active = "channels"
	h.D.Render(w, r, Channels(h.D.Cfg.BaseURL, current.Config, channels, nav))
}

// HandleSetChannels stores both channels. An empty value selects the default. A
// channel that is no longer synced answers 200 with a status fragment: htmx does
// not swap a 4xx body.
func (h *Handlers) HandleSetChannels(w http.ResponseWriter, r *http.Request) {
	if !web.ParseForm(w, r) {
		return
	}
	command := strings.TrimSpace(r.PostFormValue("command_channel_id"))
	summary := strings.TrimSpace(r.PostFormValue("summary_channel_id"))
	err := h.D.Settings.SetChannels(r.Context(), r.PathValue("id"), command, summary)
	switch {
	case errors.Is(err, ports.ErrUnknownChannel):
		h.D.Render(w, r, StaleStatus())
	case err != nil:
		h.D.ServerError(w, r, "set channels", err)
	default:
		h.D.Render(w, r, SavedStatus())
	}
}

// Package channelshttp is the admin-only Channels page: which channel takes the
// /book and /unbook commands, and which one holds the reservation summary. It
// works on a server without premium, like Settings.
package channelshttp

import (
	"spot-assistant/internal/infrastructure/web"
)

// Register wires the Channels routes (admin only) onto the router.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	r.Get("/servers/{id}/channels", d.RequireAuth(d.RequireAdmin(h.HandleChannels)))
	r.Post("/servers/{id}/channels", d.RequireAuth(d.RequireAdmin(h.HandleSetChannels)))
}

// Package settingshttp is the admin-only Settings page: the bot re-invite, the
// "Refresh server data" button, the Tibia world and the four rank lists.
package settingshttp

import (
	"spot-assistant/internal/infrastructure/web"
)

// Register wires the Settings routes (admin only) onto the router.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	r.Get("/servers/{id}/settings", d.RequireAuth(d.RequireAdmin(h.HandleSettings)))
	r.Post("/servers/{id}/settings/ranks/{kind}", d.RequireAuth(d.RequireAdmin(h.HandleSetRanks)))
	r.Post("/servers/{id}/settings/world", d.RequireAuth(d.RequireAdmin(h.HandleSetWorld)))
	r.Post("/servers/{id}/settings/refresh", d.RequireAuth(d.RequireAdmin(h.HandleRefresh)))
}

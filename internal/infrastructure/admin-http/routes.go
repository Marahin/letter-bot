// Package adminhttp is the site-wide admin section: the list of every stored
// server with its premium switch. Every route is site-admin only and answers 404
// to anyone else (RequireSiteAdmin).
package adminhttp

import (
	"spot-assistant/internal/infrastructure/web"
)

// Register wires the admin routes onto the router.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	r.Get("/admin/guilds", d.RequireAuth(d.RequireSiteAdmin(h.HandleGuilds)))
	r.Post("/admin/guilds/{id}/premium", d.RequireAuth(d.RequireSiteAdmin(h.HandleSetPremium)))
}

// Package spotshttp is the Respawns page: the guild's respawn list, with add,
// rename, remove (delete or archive), restore and the default list import for
// managers.
package spotshttp

import (
	"net/http"

	"spot-assistant/internal/infrastructure/web"
)

// Register wires the Respawns routes onto the router. Viewers see the list;
// every change needs the manage tier.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	view := func(next http.HandlerFunc) http.HandlerFunc {
		return d.RequireAuth(d.RequireView(next))
	}
	manage := func(next http.HandlerFunc) http.HandlerFunc {
		return d.RequireAuth(d.RequireManage(next))
	}
	r.Get("/servers/{id}/spots", view(h.HandleList))
	r.Post("/servers/{id}/spots", manage(h.HandleCreate))
	r.Post("/servers/{id}/spots/import", manage(h.HandleImport))
	r.Post("/servers/{id}/spots/{spot}/rename", manage(h.HandleRename))
	r.Post("/servers/{id}/spots/{spot}/remove", manage(h.HandleRemove))
	r.Post("/servers/{id}/spots/{spot}/restore", manage(h.HandleRestore))
}

// Package reservationshttp is the Reservations page: the guild's reservations
// with a filter bar, and a dialog to book, edit and delete them. A respawn's own
// page (/spots/{spot}) is the same list, fixed to that respawn.
package reservationshttp

import (
	"net/http"

	"spot-assistant/internal/infrastructure/web"
)

// Register wires the Reservations routes. Viewers see the lists; booking needs
// the reserve tier and the handlers check each change against the owner or
// manage right.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	guard := func(tier func(http.HandlerFunc) http.HandlerFunc, next http.HandlerFunc) http.HandlerFunc {
		return d.RequireAuth(tier(next))
	}
	r.Get("/servers/{id}/reservations", guard(d.RequireView, h.HandleList))
	r.Get("/servers/{id}/spots/{spot}", guard(d.RequireView, h.HandleSpot))
	r.Post("/servers/{id}/reservations", guard(d.RequireReserve, h.HandleCreate))
	r.Get("/servers/{id}/reservations/authors", guard(d.RequireManage, h.HandleAuthors))
	r.Get("/servers/{id}/reservations/{reservation}/edit", guard(d.RequireReserve, h.HandleEditForm))
	r.Post("/servers/{id}/reservations/{reservation}/edit", guard(d.RequireReserve, h.HandleEdit))
	r.Post("/servers/{id}/reservations/{reservation}/delete", guard(d.RequireReserve, h.HandleDelete))
}

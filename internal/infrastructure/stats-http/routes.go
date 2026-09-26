// Package statshttp is the Stats section: the overview, the respawn, player and character
// tables with their detail pages, and a character's own page with its TibiaData profile.
package statshttp

import (
	"net/http"

	"spot-assistant/internal/infrastructure/web"
)

// Register wires the Stats routes. Every page is view tier, premium, and reads the shared
// day range.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return d.RequireAuth(d.RequireView(d.RequirePremium(d.WithRangeSelection(next))))
	}
	r.Get("/servers/{id}/stats", guard(h.HandleOverview))
	r.Get("/servers/{id}/stats/spots", guard(h.HandleSpots))
	r.Get("/servers/{id}/stats/players", guard(h.HandlePlayers))
	r.Get("/servers/{id}/stats/characters", guard(h.HandleCharacters))
	r.Get("/servers/{id}/stats/spots/{spot}", guard(h.HandleSpot))
	r.Get("/servers/{id}/stats/players/{user}", guard(h.HandlePlayer))
	r.Get("/servers/{id}/characters/{name}", guard(h.HandleCharacter))
}

// Package statshttp is the Stats section: the overview, the respawn, player and character
// tables with their detail pages, a character's own page with its TibiaData profile, and the
// cross-server world and spawn pages.
package statshttp

import (
	"net/http"

	"spot-assistant/internal/infrastructure/web"
)

// Register wires the Stats routes. Every page is public (a member gets the
// sidebar, anyone else the public bar) and reads the shared day range.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return d.PublicView(d.WithRangeSelection(next))
	}
	r.Get("/servers/{id}/stats", guard(h.HandleOverview))
	r.Get("/servers/{id}/stats/spots", guard(h.HandleSpots))
	r.Get("/servers/{id}/stats/players", guard(h.HandlePlayers))
	r.Get("/servers/{id}/stats/characters", guard(h.HandleCharacters))
	r.Get("/servers/{id}/stats/spots/{spot}", guard(h.HandleSpot))
	r.Get("/servers/{id}/stats/players/{user}", guard(h.HandlePlayer))
	r.Get("/servers/{id}/characters/{name}", guard(h.HandleCharacter))

	// The cross-server pages: public for everyone, with their own range.
	cross := d.WithCrossRangeSelection
	r.Get(web.WorldsPath, cross(h.HandleWorlds))
	r.Get(web.WorldsPath+"/{world}", cross(h.HandleWorld))
	r.Get(web.SpawnsPath, cross(h.HandleSpawns))
	r.Get(web.SpawnsPath+"/{key}", cross(h.HandleSpawn))
}

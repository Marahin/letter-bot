// Package toolshttp holds the public tools: the Loot Calculator. They need no
// sign-in, and render in the top bar when signed out and in the sidebar when
// signed in. The split runs here, on the server; nothing is sent to a third party.
package toolshttp

import (
	"spot-assistant/internal/infrastructure/web"
)

// LootCalculatorPath is the Loot Calculator page and its form action.
const LootCalculatorPath = web.LootCalculatorPath

// Register wires the tool routes.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	r.Get(LootCalculatorPath, h.HandleLootCalculator)
	r.Post(LootCalculatorPath, h.HandleLootCalculate)
}

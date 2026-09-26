// Package toolshttp holds the public tools: the Loot Calculator. They need no
// sign-in, and render in the top bar when signed out and in the sidebar when
// signed in.
package toolshttp

import (
	"spot-assistant/internal/infrastructure/web"
)

// LootCalculatorPath is the Loot Calculator page and its form action.
const LootCalculatorPath = "/tools/loot-calculator"

// Register wires the tool routes.
func Register(r *web.Router, d *web.Deps) {
	h := New(d)
	r.Get(LootCalculatorPath, h.HandleLootCalculator)
	r.Post(LootCalculatorPath, h.HandleLootCalculate)
}

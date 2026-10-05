//go:build devauth

// Command seed writes the dev and e2e data: two servers (one premium, one
// locked), their manage rank, and the premium server's respawns and reservations.
// It compiles only under the devauth build tag.
package main

import (
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/fxmodule/seedapp"
)

func main() {
	fx.New(seedapp.App()).Run()
}

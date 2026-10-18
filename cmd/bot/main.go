// Command bot runs the Discord bot that books hunting spots for Tibia players.
package main

import (
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/fxmodule/botapp"
)

func main() {
	fx.New(botapp.App()).Run()
}

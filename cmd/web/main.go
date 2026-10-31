// Command web runs TibiaLoot.com, the web panel that shares the database with the bot.
package main

import (
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/fxmodule/webapp"
)

func main() {
	fx.New(webapp.App()).Run()
}

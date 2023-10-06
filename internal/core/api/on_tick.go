package api

import (
	"spot-assistant/internal/ports"
)

func (a *Application) OnTick(bot ports.BotPort) {
	guilds := bot.GetGuilds()
	for _, guild := range guilds {
		go a.UpdateGuildSummaryAndLogError(bot, guild)
	}
}

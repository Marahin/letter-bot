package api

import "spot-assistant/internal/ports"

func (a *Application) OnReady(bot ports.BotPort) {
	a.log.Info("OnReady")

	bot.StartTicking()
}

package api

import (
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/ports"
)

func (a *Application) OnGuildCreate(bot ports.BotPort, guild *discord.Guild) {
	log := a.log.WithFields(logrus.Fields{"event": "OnGuildCreate", "guild.ID": guild.ID, "guild.Name": guild.Name})
	// Register commands
	err := bot.RegisterCommands(guild)
	if err != nil {
		log.Errorf("could not overwrite commands: %s", err)

		return
	}

	err = bot.EnsureChannel(guild)
	if err != nil {
		log.Errorf("could not ensure channels: %s", err)

		return
	}

	err = bot.EnsureRoles(guild)
	if err != nil {
		log.Errorf("could not ensure roles: %s", err)

		return
	}

	summaryChannel, err := bot.FindChannel(guild, discord.SummaryChannel)
	if err != nil {
		log.Errorf("could not find summary channel: %s", err)

		return
	}

	err = bot.CleanChannel(guild, summaryChannel)
	if err != nil {
		return
	}

	log.Info("successfully registered a guild")

	go a.UpdateGuildSummaryAndLogError(bot, guild)
}

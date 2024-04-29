package api

import (
	"spot-assistant/internal/core/dto/discord"
)

func (a *Application) OnGuildCreate(guild *discord.Guild) {
	log := a.log.With("event", "OnGuildCreate", "guild.ID", guild.ID, "guild.Name", guild.Name)
	// Register commands
	err := a.botSrv.RegisterCommands(guild)
	if err != nil {
		log.Errorf("could not overwrite commands: %s", err)

		return
	}

	err = a.botSrv.EnsureChannel(guild)
	if err != nil {
		log.Errorf("could not ensure channels: %s", err)

		return
	}

	err = a.botSrv.EnsureRoles(guild)
	if err != nil {
		log.Errorf("could not ensure roles: %s", err)

		return
	}

	summaryChannel, err := a.botSrv.FindChannelByName(guild, discord.SummaryChannel)
	if err != nil {
		log.Errorf("could not find summary channel: %s", err)

		return
	}

	err = a.botSrv.CleanChannel(guild, summaryChannel)
	if err != nil {
		return
	}

	log.Info("successfully registered a guild")

	go a.UpdateGuildSummaryAndLogError(guild)
}

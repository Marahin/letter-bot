package api

import (
	"context"
	"fmt"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/ports"
	"spot-assistant/util"

	"github.com/sirupsen/logrus"
)

// UpdateGuild makes a full-fledged guild update including summary re-generation.
func (a *Application) UpdateGuildSummary(bot ports.BotPort, guild *discord.Guild) error {
	log := a.log.WithFields(logrus.Fields{"guild.ID": guild.ID, "guild.Name": guild.Name, "name": "UpdateGuildSummary"})

	summaryChannel, err := bot.FindChannel(guild, "letter-summary")
	if err != nil {
		log.Errorf("could not find summary channel: %s", err)

		return err
	}

	// For each guild
	reservations, err := a.db.SelectUpcomingReservationsWithSpot(
		context.Background(), guild.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to retrieve upcoming reservations: %s", err)
	}

	if len(reservations) == 0 {
		log.Warning("no reservations for guild, skipping")

		return nil
	}

	summary, err := a.summarySrv.PrepareSummary(reservations)
	if err != nil {
		log.Errorf("could not generate summary: %s", err)

		return fmt.Errorf("failed to retrieve upcoming reservations: %s", err)
	}

	log.Info("updating summary")

	err = bot.SendLetterMessage(guild, summaryChannel, summary)
	if err != nil {
		log.Errorf("could not send letter message: %s", err)

		return fmt.Errorf("failed to retrieve upcoming reservations: %s", err)
	}

	return nil
}

func (a *Application) UpdateGuildSummaryAndLogError(bot ports.BotPort, guild *discord.Guild) {
	util.LogError(a.log, a.UpdateGuildSummary(bot, guild))
}

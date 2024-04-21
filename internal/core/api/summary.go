package api

import (
	"context"
	"fmt"
	"strconv"

	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"

	"github.com/sirupsen/logrus"
)

// UpdateGuild makes a full-fledged guild update including summary re-generation.
func (a *Application) UpdateGuildSummary(guild *discord.Guild) error {
	log := a.log.WithFields(logrus.Fields{"guild.ID": guild.ID, "guild.Name": guild.Name, "name": "UpdateGuildSummary"})

	summaryChannel, err := a.botSrv.FindChannelByName(guild, "letter-summary")
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

	err = a.botSrv.SendLetterMessage(guild, summaryChannel, summary)
	if err != nil {
		log.Errorf("could not send letter message: %s", err)

		return fmt.Errorf("failed to retrieve upcoming reservations: %s", err)
	}

	return nil
}

func (a *Application) UpdateGuildSummaryAndLogError(guild *discord.Guild) {
	errors.LogError(a.log, a.UpdateGuildSummary(guild))
}

func (a *Application) OnPrivateSummary(request summary.PrivateSummaryRequest) error {
	log := a.log.WithFields(logrus.Fields{"user.ID": request.UserID, "guild.ID": request.GuildID})
	log.Info("OnPrivateSummary")

	res, err := a.db.SelectUpcomingReservationsWithSpot(context.Background(), strconv.FormatInt(request.GuildID, 10))
	if err != nil {
		return fmt.Errorf("could not fetch upcoming reservations")
	}

	if len(res) == 0 {
		log.Warning("no reservations to display in DM; skipping")

		return nil
	}

	summary, err := a.summarySrv.PrepareSummary(res)
	if err != nil {
		log.Errorf("could not generate summary: %s", err)

		return fmt.Errorf("could not generate summary: %s", err)
	}

	dmChannel, err := a.botSrv.OpenDM(&discord.Member{ID: strconv.FormatInt(request.UserID, 10)})
	if err != nil {
		return err
	}

	err = a.botSrv.SendLetterMessage(nil, dmChannel, summary)
	if err != nil {
		log.Errorf("could not send letter message: %s", err)

		return fmt.Errorf("could not send letter message: %s", err)
	}

	return nil
}

package api

import (
	"context"
	"strconv"

	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"

	"github.com/sirupsen/logrus"
)

// UpdateGuild makes a full-fledged guild update including summary re-generation.
func (a *Application) UpdateGuildSummary(guild *discord.Guild) error {
	log := a.log.WithFields(logrus.Fields{"guild.ID": guild.ID, "guild.Name": guild.Name, "name": "UpdateGuildSummary"})

	// For each guild
	reservations, err := a.db.SelectUpcomingReservationsWithSpot(
		context.Background(), guild.ID,
	)
	if err != nil {
		return err
	}

	if len(reservations) == 0 {
		log.Warning("no reservations for guild, skipping")

		return nil
	}

	summ, err := a.summarySrv.PrepareSummary(reservations)
	if err != nil {
		return err
	}

	return a.commSrv.SendGuildSummary(guild, summ)
}

func (a *Application) UpdateGuildSummaryAndLogError(guild *discord.Guild) {
	errors.LogError(a.log, a.UpdateGuildSummary(guild))
}

func (a *Application) OnPrivateSummary(request summary.PrivateSummaryRequest) error {
	log := a.log.WithFields(logrus.Fields{"user.ID": request.UserID, "guild.ID": request.GuildID})
	log.Debug("OnPrivateSummary")

	res, err := a.db.SelectUpcomingReservationsWithSpot(context.Background(), strconv.FormatInt(request.GuildID, 10))
	if err != nil {
		return err
	}

	if len(res) == 0 {
		log.Warning("no reservations to display in DM; skipping")

		return nil
	}

	summ, err := a.summarySrv.PrepareSummary(res)
	if err != nil {
		return err
	}

	return a.commSrv.SendPrivateSummary(request, summ)
}

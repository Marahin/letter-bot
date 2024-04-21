package api

import (
	"context"
	"fmt"
	"spot-assistant/internal/core/dto/reservation"
	"strconv"

	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/ports"

	"github.com/sirupsen/logrus"
)

// UpdateGuild makes a full-fledged guild update including summary re-generation.
func (a *Application) UpdateGuildSummary(bot ports.BotPort, guild *discord.Guild) error {
	log := a.log.WithFields(logrus.Fields{"guild.ID": guild.ID, "guild.Name": guild.Name, "name": "UpdateGuildSummary"})

	summaryChannel, err := bot.FindChannelByName(guild, "letter-summary")
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
	errors.LogError(a.log, a.UpdateGuildSummary(bot, guild))
}

func (a *Application) OnPrivateSummary(bot ports.BotPort, request summary.PrivateSummaryRequest) error {
	log := a.log.WithFields(logrus.Fields{"user.ID": request.UserID, "guild.ID": request.GuildID})
	log.Info("OnPrivateSummary")

	res, err := a.fetchUpcomingReservationsWithSpot(request)
	if res == nil {
		fmt.Errorf("could not fetch upcoming reservations: %v", err)

		return nil
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

	dmChannel, err := bot.OpenDM(&discord.Member{ID: strconv.FormatInt(request.UserID, 10)})
	if err != nil {
		return err
	}

	err = bot.SendLetterMessage(nil, dmChannel, summary)
	if err != nil {
		log.Errorf("could not send letter message: %s", err)

		return fmt.Errorf("could not send letter message: %s", err)
	}

	return nil
}

func (a *Application) fetchUpcomingReservationsWithSpot(request summary.PrivateSummaryRequest) ([]*reservation.ReservationWithSpot, error) {
	var res []*reservation.ReservationWithSpot
	var err error

	if request.Spots != nil {
		res, err = a.db.SelectUpcomingReservationsWithSpotBySpots(context.Background(), strconv.FormatInt(request.GuildID, 10), request.Spots)
		if err != nil {
			return nil, fmt.Errorf("could not fetch upcoming reservations: %v", err)
		}
	} else {
		res, err = a.db.SelectUpcomingReservationsWithSpot(context.Background(), strconv.FormatInt(request.GuildID, 10))
		if err != nil {
			return nil, fmt.Errorf("could not fetch upcoming reservations: %v", err)
		}
	}

	return res, nil
}

package eventhandler

import (
	"context"
	"fmt"
	"strconv"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Handler) OnPrivateSummary(request summary.PrivateSummaryRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInteractionTimeout)
	defer cancel()

	guildIDStr := strconv.FormatInt(request.GuildID, 10)

	var (
		res []*reservation.ReservationWithSpot
		err error
	)

	if request.SpotName != "" {
		res, err = a.db.SelectUpcomingReservationsWithSpotForSpot(ctx, guildIDStr, request.SpotName)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return fmt.Errorf("no reservations for %s", request.SpotName)
		}
	} else {
		res, err = a.db.SelectUpcomingReservationsWithSpot(ctx, guildIDStr)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return nil
		}
	}

	summ, err := a.summarySrv.PrepareSummary(res)
	if err != nil {
		return err
	}

	return a.commSrv.SendPrivateSummary(request, summ)
}

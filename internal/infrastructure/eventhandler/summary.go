package eventhandler

import (
	"context"
	"fmt"
	"strconv"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func (h *Handler) OnPrivateSummary(request summary.PrivateSummaryRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInteractionTimeout)
	defer cancel()

	guildIDStr := strconv.FormatInt(request.GuildID, 10)

	var (
		res []*reservation.ReservationWithSpot
		err error
	)

	if request.SpotName != "" {
		res, err = h.db.SelectUpcomingReservationsWithSpotForSpot(ctx, guildIDStr, request.SpotName)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return fmt.Errorf("no reservations for %s", request.SpotName)
		}
	} else {
		res, err = h.db.SelectUpcomingReservationsWithSpot(ctx, guildIDStr)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return nil
		}
	}
	// metrics: update gauge for upcoming reservations in this guild
	if h.metrics != nil {
		// Guild name is not available in this handler; pass empty string
		h.metrics.SetUpcomingReservations(strconv.FormatInt(request.GuildID, 10), "", len(res))
	}

	summ, err := h.summarySrv.PrepareSummary(res)
	if err != nil {
		return err
	}

	return h.commSrv.SendPrivateSummary(request, summ)
}

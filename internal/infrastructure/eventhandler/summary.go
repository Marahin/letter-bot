package eventhandler

import (
	"context"
	"strconv"

	"spot-assistant/internal/core/dto/summary"
)

func (a *Handler) OnPrivateSummary(request summary.PrivateSummaryRequest) error {
	res, err := a.db.SelectUpcomingReservationsWithSpot(context.Background(), strconv.FormatInt(request.GuildID, 10))
	if err != nil {
		return err
	}

	// metrics: update gauge for upcoming reservations in this guild
    if a.metrics != nil {
        // Guild name is not available in this handler; pass empty string
        a.metrics.SetUpcomingReservations(strconv.FormatInt(request.GuildID, 10), "", len(res))
    }

	if len(res) == 0 {
		return nil
	}

	summ, err := a.summarySrv.PrepareSummary(res)
	if err != nil {
		return err
	}

	return a.commSrv.SendPrivateSummary(request, summ)
}

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

	if len(res) == 0 {
		return nil
	}

	summ, err := a.summarySrv.PrepareSummary(res)
	if err != nil {
		return err
	}

	return a.commSrv.SendPrivateSummary(request, summ)
}

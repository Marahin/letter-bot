package eventhandler

import (
	"context"
	"fmt"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	"strconv"
)

func (a *Handler) OnPrivateSummary(request summary.PrivateSummaryRequest) error {
	res, err := a.fetchUpcomingReservationsWithSpot(request)
	if err != nil {
		return err
	}

	if len(res) == 0 {
		return nil
	}

	prepareSummary, err := a.summarySrv.PrepareSummary(res)
	if err != nil {
		return err
	}

	return a.commSrv.SendPrivateSummary(request, prepareSummary)
}

func (a *Handler) fetchUpcomingReservationsWithSpot(request summary.PrivateSummaryRequest) ([]*reservation.ReservationWithSpot, error) {
	var res []*reservation.ReservationWithSpot
	var err error

	if request.SpotNames != nil {
		res, err = a.db.SelectUpcomingReservationsWithSpotBySpots(context.Background(), strconv.FormatInt(request.GuildID, 10), request.SpotNames)
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

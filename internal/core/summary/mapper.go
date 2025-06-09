package summary

import (
	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) MapReservation(reservation *reservation.Reservation) *summary.Booking {
	status := ""
	if a.onlineCheck != nil {
		online := a.onlineCheck.IsOnline(reservation.Author)
		if online {
			status = ":green_circle: "
		} else {
			status = ":red_circle: "
		}
	}
	return &summary.Booking{
		Author:          reservation.Author,
		StartAt:         reservation.StartAt,
		EndAt:           reservation.EndAt,
		AuthorDiscordID: reservation.AuthorDiscordID,
		Status:          status,
	}
}

func (a *Adapter) MapReservations(reservations []*reservation.Reservation) []*summary.Booking {
	return collections.PoorMansMap(reservations, func(res *reservation.Reservation) *summary.Booking {
		return a.MapReservation(res)
	})
}

package summary

import (
	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) MapReservation(reservation *reservation.Reservation) *summary.Booking {
	status := summary.Unknown
	if a.onlineCheck != nil {
		if a.onlineCheck.IsOnline(reservation.Author) {
			status = summary.Online
		} else {
			status = summary.Offline
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

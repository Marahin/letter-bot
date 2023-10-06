package summary

import (
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/util"
)

func (a *Adapter) MapReservation(reservation *reservation.Reservation) *summary.Booking {
	return &summary.Booking{
		Author:          reservation.Author,
		StartAt:         reservation.StartAt,
		EndAt:           reservation.EndAt,
		AuthorDiscordID: reservation.AuthorDiscordID,
	}
}

func (a *Adapter) MapReservations(reservations []*reservation.Reservation) []*summary.Booking {
	return util.PoorMansMap(reservations, func(res *reservation.Reservation) *summary.Booking {
		return a.MapReservation(res)
	})
}

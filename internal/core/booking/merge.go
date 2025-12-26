package booking

import (
	"context"
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func (a *Adapter) mergeAdjacentReservations(ctx context.Context, request book.BookRequest) error {
	upcoming, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(ctx, request.Guild, request.Member)
	if err != nil {
		return err
	}

	mergedStartAt, mergedEndAt, mergedIDs, _ := calculateBookingMerge(upcoming, request.Spot, request.StartAt, request.EndAt)

	if len(mergedIDs) <= 1 {
		return nil
	}

	if err := validateHuntLength(mergedEndAt.Sub(mergedStartAt)); err != nil {
		return err
	}

	primaryID := mergedIDs[0]
	err = a.reservationRepo.UpdateReservation(ctx, primaryID, mergedStartAt, mergedEndAt)
	if err != nil {
		return err
	}

	for _, idToDelete := range mergedIDs[1:] {
		err = a.reservationRepo.DeletePresentMemberReservation(ctx, request.Guild, request.Member, idToDelete)
		if err != nil {
			fmt.Printf("failed to delete bridged reservation %d: %v\n", idToDelete, err)
		}
	}

	return nil
}

func calculateBookingMerge(
	reservations []*reservation.ReservationWithSpot,
	spotName string,
	startAt, endAt time.Time,
) (
	mergedStartAt, mergedEndAt time.Time,
	mergedIDs []int64,
	unaffectedReservations []*reservation.ReservationWithSpot,
) {
	mergedStartAt = startAt
	mergedEndAt = endAt

	for _, r := range reservations {
		if r.Spot.Name != spotName {
			unaffectedReservations = append(unaffectedReservations, r)
			continue
		}

		gapAfter := mergedStartAt.Sub(r.EndAt)
		gapBefore := r.StartAt.Sub(mergedEndAt)

		if mergedStartAt.After(r.EndAt) && gapAfter > time.Minute {
			unaffectedReservations = append(unaffectedReservations, r)
			continue
		}

		if r.StartAt.After(mergedEndAt) && gapBefore > time.Minute {
			unaffectedReservations = append(unaffectedReservations, r)
			continue
		}

		mergedIDs = append(mergedIDs, r.Reservation.ID)
		if r.StartAt.Before(mergedStartAt) {
			mergedStartAt = r.StartAt
		}
		if r.EndAt.After(mergedEndAt) {
			mergedEndAt = r.EndAt
		}
	}

	return
}

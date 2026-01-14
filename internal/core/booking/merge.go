package booking

import (
	"context"
	"fmt"
	"time"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

type MergeResult struct {
	ShouldMerge          bool
	MergedStartAt        time.Time
	MergedEndAt          time.Time
	MergedReservationIDs []int64
}

func (m *MergeResult) PrimaryReservationID() int64 {
	if len(m.MergedReservationIDs) == 0 {
		return 0
	}
	return m.MergedReservationIDs[0]
}

func (m *MergeResult) ReservationsToDelete() []int64 {
	if len(m.MergedReservationIDs) <= 1 {
		return nil
	}
	return m.MergedReservationIDs[1:]
}

const adjacencyTolerance = time.Minute

func (a *Adapter) mergeAdjacentReservations(ctx context.Context, request book.BookRequest) error {
	upcoming, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(ctx, request.Guild, request.Member)
	if err != nil {
		return fmt.Errorf("failed to fetch upcoming reservations: %w", err)
	}

	matchingSpot := filterReservationsBySpot(upcoming, request.Spot)
	mergeResult := analyzeAdjacentReservations(matchingSpot, request.StartAt, request.EndAt)

	if !mergeResult.ShouldMerge {
		return nil
	}

	if err := validateHuntLength(calculateMergedDuration(mergeResult)); err != nil {
		return fmt.Errorf("merged reservation would exceed maximum length: %w", err)
	}

	return a.executeMerge(ctx, request, mergeResult)
}

func analyzeAdjacentReservations(
	reservations []*reservation.ReservationWithSpot,
	newReservationStart, newReservationEnd time.Time,
) *MergeResult {
	result := &MergeResult{
		MergedStartAt:        newReservationStart,
		MergedEndAt:          newReservationEnd,
		MergedReservationIDs: make([]int64, 0),
	}

	for _, r := range reservations {
		if isReservationAdjacentToMergedRange(result.MergedStartAt, result.MergedEndAt, r) {
			includeReservationInMerge(result, r)
		}
	}

	result.ShouldMerge = len(result.MergedReservationIDs) > 1
	return result
}

func (a *Adapter) executeMerge(ctx context.Context, request book.BookRequest, result *MergeResult) error {
	primaryID := result.PrimaryReservationID()

	if err := a.updateTimeOfMergedReservation(ctx, primaryID, result.MergedStartAt, result.MergedEndAt); err != nil {
		return fmt.Errorf("failed to update primary reservation %d: %w", primaryID, err)
	}

	if err := a.deleteMergedReservations(ctx, request, result.ReservationsToDelete()); err != nil {
		return fmt.Errorf("failed to delete merged reservations: %w", err)
	}

	a.log.With(
		"primaryID", primaryID,
		"deletedIDs", result.ReservationsToDelete(),
		"mergedStart", result.MergedStartAt,
		"mergedEnd", result.MergedEndAt,
		"spot", request.Spot,
	).Info("successfully merged adjacent reservations")

	return nil
}

func filterReservationsBySpot(reservations []*reservation.ReservationWithSpot, spotName string) []*reservation.ReservationWithSpot {
	return collections.PoorMansFilter(reservations, func(r *reservation.ReservationWithSpot) bool {
		return r.Spot.Name == spotName
	})
}

func calculateMergedDuration(result *MergeResult) time.Duration {
	return result.MergedEndAt.Sub(result.MergedStartAt)
}

func isReservationAdjacentToMergedRange(mergedStart, mergedEnd time.Time, r *reservation.ReservationWithSpot) bool {
	gapAfter := mergedStart.Sub(r.EndAt)
	gapBefore := r.StartAt.Sub(mergedEnd)

	if mergedStart.After(r.EndAt) && gapAfter > adjacencyTolerance {
		return false
	}

	if r.StartAt.After(mergedEnd) && gapBefore > adjacencyTolerance {
		return false
	}

	return true
}

func includeReservationInMerge(result *MergeResult, r *reservation.ReservationWithSpot) {
	result.MergedReservationIDs = append(result.MergedReservationIDs, r.Reservation.ID)

	if r.StartAt.Before(result.MergedStartAt) {
		result.MergedStartAt = r.StartAt
	}
	if r.EndAt.After(result.MergedEndAt) {
		result.MergedEndAt = r.EndAt
	}
}

func (a *Adapter) updateTimeOfMergedReservation(ctx context.Context, primaryID int64, startAt, endAt time.Time) error {
	return a.reservationRepo.UpdateReservation(ctx, primaryID, startAt, endAt)
}

func (a *Adapter) deleteMergedReservations(ctx context.Context, request book.BookRequest, idsToDelete []int64) error {
	for _, idToDelete := range idsToDelete {
		if err := a.reservationRepo.DeletePresentMemberReservation(ctx, request.Guild, request.Member, idToDelete); err != nil {
			return err
		}
	}
	return nil
}

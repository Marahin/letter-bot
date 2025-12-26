package booking

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func TestCalculateBookingMerge(t *testing.T) {
	today := time.Now()
	mkTime := func(h, m int) time.Time {
		return time.Date(today.Year(), today.Month(), today.Day(), h, m, 0, 0, time.UTC)
	}

	tests := []struct {
		name           string
		reservations   []*reservation.ReservationWithSpot
		spotName       string
		startAt        time.Time
		endAt          time.Time
		wantStart      time.Time
		wantEnd        time.Time
		wantMergedIDs  []int64
		wantUnaffected int
	}{
		{
			name:           "no existing reservations",
			reservations:   []*reservation.ReservationWithSpot{},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(10, 0),
			wantEnd:        mkTime(11, 0),
			wantMergedIDs:  nil,
			wantUnaffected: 0,
		},
		{
			name: "no overlap - gap exceeds tolerance",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(8, 0), EndAt: mkTime(9, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 2),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(10, 2),
			wantEnd:        mkTime(11, 0),
			wantMergedIDs:  nil,
			wantUnaffected: 1,
		},
		{
			name: "exact adjacency - append",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(9, 0),
			wantEnd:        mkTime(11, 0),
			wantMergedIDs:  []int64{1},
			wantUnaffected: 0,
		},
		{
			name: "exact adjacency - prepend",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(11, 0), EndAt: mkTime(12, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(10, 0),
			wantEnd:        mkTime(12, 0),
			wantMergedIDs:  []int64{1},
			wantUnaffected: 0,
		},
		{
			name: "1 minute gap - within tolerance",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 1),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(9, 0),
			wantEnd:        mkTime(11, 0),
			wantMergedIDs:  []int64{1},
			wantUnaffected: 0,
		},
		{
			name: "overlap",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 30)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(9, 0),
			wantEnd:        mkTime(11, 0),
			wantMergedIDs:  []int64{1},
			wantUnaffected: 0,
		},
		{
			name: "fully contained",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(12, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(9, 0),
			wantEnd:        mkTime(12, 0),
			wantMergedIDs:  []int64{1},
			wantUnaffected: 0,
		},
		{
			name: "different spot - no merge",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(10, 0), EndAt: mkTime(11, 0)}, Spot: reservation.Spot{Name: "Spot-2"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(10, 0),
			wantEnd:        mkTime(11, 0),
			wantMergedIDs:  nil,
			wantUnaffected: 1,
		},
		{
			name: "bridge multiple reservations",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
				{Reservation: reservation.Reservation{ID: 2, StartAt: mkTime(11, 0), EndAt: mkTime(12, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(9, 0),
			wantEnd:        mkTime(12, 0),
			wantMergedIDs:  []int64{1, 2},
			wantUnaffected: 0,
		},
		{
			name: "mixed spots - only merge matching",
			reservations: []*reservation.ReservationWithSpot{
				{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
				{Reservation: reservation.Reservation{ID: 2, StartAt: mkTime(10, 0), EndAt: mkTime(11, 0)}, Spot: reservation.Spot{Name: "Spot-2"}},
				{Reservation: reservation.Reservation{ID: 3, StartAt: mkTime(11, 0), EndAt: mkTime(12, 0)}, Spot: reservation.Spot{Name: "Spot-1"}},
			},
			spotName:       "Spot-1",
			startAt:        mkTime(10, 0),
			endAt:          mkTime(11, 0),
			wantStart:      mkTime(9, 0),
			wantEnd:        mkTime(12, 0),
			wantMergedIDs:  []int64{1, 3},
			wantUnaffected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, gotIDs, gotUnaffected := calculateBookingMerge(
				tt.reservations,
				tt.spotName,
				tt.startAt,
				tt.endAt,
			)

			assert.Equal(t, tt.wantStart, gotStart)
			assert.Equal(t, tt.wantEnd, gotEnd)
			assert.ElementsMatch(t, tt.wantMergedIDs, gotIDs)
			assert.Len(t, gotUnaffected, tt.wantUnaffected)
		})
	}
}

func TestMergeAdjacentReservations(t *testing.T) {
	today := time.Now()
	mkTime := func(h, m int) time.Time {
		return time.Date(today.Year(), today.Month(), today.Day(), h, m, 0, 0, time.UTC)
	}

	guildID := &guild.Guild{ID: "g1"}
	memberID := &member.Member{ID: "m1"}
	spotName := "Spot-1"

	t.Run("no merge needed", func(t *testing.T) {
		repo := mocks.NewMockReservationRepository(t)
		adapter := NewAdapter(nil, repo, nil)

		existing := []*reservation.ReservationWithSpot{
			{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(8, 0), EndAt: mkTime(9, 0)}, Spot: reservation.Spot{Name: spotName}},
		}
		repo.On("SelectUpcomingMemberReservationsWithSpots", mock.Anything, guildID, memberID).Return(existing, nil)

		req := book.BookRequest{
			Guild:   guildID,
			Member:  memberID,
			Spot:    spotName,
			StartAt: mkTime(10, 0),
			EndAt:   mkTime(11, 0),
		}

		err := adapter.mergeAdjacentReservations(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("simple merge - two adjacent reservations", func(t *testing.T) {
		repo := mocks.NewMockReservationRepository(t)
		adapter := NewAdapter(nil, repo, nil)

		existing := []*reservation.ReservationWithSpot{
			{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: spotName}},
			{Reservation: reservation.Reservation{ID: 2, StartAt: mkTime(10, 0), EndAt: mkTime(11, 0)}, Spot: reservation.Spot{Name: spotName}},
		}
		repo.On("SelectUpcomingMemberReservationsWithSpots", mock.Anything, guildID, memberID).Return(existing, nil)
		repo.On("UpdateReservation", mock.Anything, int64(1), mkTime(9, 0), mkTime(11, 0)).Return(nil)
		repo.On("DeletePresentMemberReservation", mock.Anything, guildID, memberID, int64(2)).Return(nil)

		req := book.BookRequest{
			Guild:   guildID,
			Member:  memberID,
			Spot:    spotName,
			StartAt: mkTime(10, 0),
			EndAt:   mkTime(11, 0),
		}

		err := adapter.mergeAdjacentReservations(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("bridge three reservations", func(t *testing.T) {
		repo := mocks.NewMockReservationRepository(t)
		adapter := NewAdapter(nil, repo, nil)

		existing := []*reservation.ReservationWithSpot{
			{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: spotName}},
			{Reservation: reservation.Reservation{ID: 2, StartAt: mkTime(10, 0), EndAt: mkTime(11, 0)}, Spot: reservation.Spot{Name: spotName}},
			{Reservation: reservation.Reservation{ID: 3, StartAt: mkTime(11, 0), EndAt: mkTime(12, 0)}, Spot: reservation.Spot{Name: spotName}},
		}
		repo.On("SelectUpcomingMemberReservationsWithSpots", mock.Anything, guildID, memberID).Return(existing, nil)
		repo.On("UpdateReservation", mock.Anything, int64(1), mkTime(9, 0), mkTime(12, 0)).Return(nil)
		repo.On("DeletePresentMemberReservation", mock.Anything, guildID, memberID, int64(2)).Return(nil)
		repo.On("DeletePresentMemberReservation", mock.Anything, guildID, memberID, int64(3)).Return(nil)

		req := book.BookRequest{
			Guild:   guildID,
			Member:  memberID,
			Spot:    spotName,
			StartAt: mkTime(10, 0),
			EndAt:   mkTime(11, 0),
		}

		err := adapter.mergeAdjacentReservations(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("validation error - merged duration exceeds limit", func(t *testing.T) {
		repo := mocks.NewMockReservationRepository(t)
		adapter := NewAdapter(nil, repo, nil)

		existing := []*reservation.ReservationWithSpot{
			{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(11, 0)}, Spot: reservation.Spot{Name: spotName}},
			{Reservation: reservation.Reservation{ID: 2, StartAt: mkTime(11, 0), EndAt: mkTime(13, 0)}, Spot: reservation.Spot{Name: spotName}},
		}
		repo.On("SelectUpcomingMemberReservationsWithSpots", mock.Anything, guildID, memberID).Return(existing, nil)

		req := book.BookRequest{
			Guild:   guildID,
			Member:  memberID,
			Spot:    spotName,
			StartAt: mkTime(11, 0),
			EndAt:   mkTime(13, 0),
		}

		err := adapter.mergeAdjacentReservations(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reservation cannot take more than 3 hours")
	})

	t.Run("repository error - select fails", func(t *testing.T) {
		repo := mocks.NewMockReservationRepository(t)
		adapter := NewAdapter(nil, repo, nil)

		repoErr := fmt.Errorf("database connection failed")
		repo.On("SelectUpcomingMemberReservationsWithSpots", mock.Anything, guildID, memberID).Return(nil, repoErr)

		req := book.BookRequest{
			Guild:   guildID,
			Member:  memberID,
			Spot:    spotName,
			StartAt: mkTime(10, 0),
			EndAt:   mkTime(11, 0),
		}

		err := adapter.mergeAdjacentReservations(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, repoErr, err)
	})

	t.Run("repository error - update fails", func(t *testing.T) {
		repo := mocks.NewMockReservationRepository(t)
		adapter := NewAdapter(nil, repo, nil)

		existing := []*reservation.ReservationWithSpot{
			{Reservation: reservation.Reservation{ID: 1, StartAt: mkTime(9, 0), EndAt: mkTime(10, 0)}, Spot: reservation.Spot{Name: spotName}},
			{Reservation: reservation.Reservation{ID: 2, StartAt: mkTime(10, 0), EndAt: mkTime(11, 0)}, Spot: reservation.Spot{Name: spotName}},
		}
		updateErr := fmt.Errorf("update failed")
		repo.On("SelectUpcomingMemberReservationsWithSpots", mock.Anything, guildID, memberID).Return(existing, nil)
		repo.On("UpdateReservation", mock.Anything, int64(1), mkTime(9, 0), mkTime(11, 0)).Return(updateErr)

		req := book.BookRequest{
			Guild:   guildID,
			Member:  memberID,
			Spot:    spotName,
			StartAt: mkTime(10, 0),
			EndAt:   mkTime(11, 0),
		}

		err := adapter.mergeAdjacentReservations(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, updateErr, err)
	})
}

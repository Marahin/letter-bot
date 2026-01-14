package booking

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func TestAnalyzeAdjacentReservations(t *testing.T) {
	t.Parallel()
	now := time.Now()

	createRes := func(id int64, start, end time.Time) *reservation.ReservationWithSpot {
		return &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{ID: id, StartAt: start, EndAt: end},
			Spot:        reservation.Spot{ID: 1, Name: "test-spot"},
		}
	}

	tests := []struct {
		name               string
		reservations       []*reservation.ReservationWithSpot
		inputStart         time.Time
		inputEnd           time.Time
		wantShouldMerge    bool
		wantStart          time.Time
		wantEnd            time.Time
		wantReservationIDs []int64
	}{
		{
			name:               "Single reservation returns false",
			reservations:       []*reservation.ReservationWithSpot{createRes(1, now, now.Add(time.Hour))},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    false,
			wantStart:          now,
			wantEnd:            now.Add(time.Hour),
			wantReservationIDs: []int64{1},
		},
		{
			name: "Two unrelated reservations returns false",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(3*time.Hour), now.Add(4*time.Hour)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    false,
			wantStart:          now,
			wantEnd:            now.Add(time.Hour),
			wantReservationIDs: []int64{1},
		},
		{
			name: "Exact adjacency after returns true",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(time.Hour), now.Add(2*time.Hour)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    true,
			wantStart:          now,
			wantEnd:            now.Add(2 * time.Hour),
			wantReservationIDs: []int64{1, 2},
		},
		{
			name: "Exact adjacency before returns true",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(-1*time.Hour), now),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    true,
			wantStart:          now.Add(-1 * time.Hour),
			wantEnd:            now.Add(time.Hour),
			wantReservationIDs: []int64{1, 2},
		},
		{
			name: "Gap within tolerance returns true",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(time.Hour).Add(30*time.Second), now.Add(2*time.Hour)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    true,
			wantStart:          now,
			wantEnd:            now.Add(2 * time.Hour),
			wantReservationIDs: []int64{1, 2},
		},
		{
			name: "Gap exactly at tolerance returns true",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(time.Hour).Add(time.Minute), now.Add(2*time.Hour)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    true,
			wantStart:          now,
			wantEnd:            now.Add(2 * time.Hour),
			wantReservationIDs: []int64{1, 2},
		},
		{
			name: "Gap exceeding tolerance returns false",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(time.Hour).Add(time.Minute).Add(time.Nanosecond), now.Add(2*time.Hour)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    false,
			wantStart:          now,
			wantEnd:            now.Add(time.Hour),
			wantReservationIDs: []int64{1},
		},
		{
			name: "Bridging two reservations returns true",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(-1*time.Hour), now),
				createRes(3, now.Add(time.Hour), now.Add(2*time.Hour)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    true,
			wantStart:          now.Add(-1 * time.Hour),
			wantEnd:            now.Add(2 * time.Hour),
			wantReservationIDs: []int64{1, 2, 3},
		},
		{
			name: "Subset violation returns true",
			reservations: []*reservation.ReservationWithSpot{
				createRes(1, now, now.Add(time.Hour)),
				createRes(2, now.Add(-30*time.Minute), now.Add(30*time.Minute)),
			},
			inputStart:         now,
			inputEnd:           now.Add(time.Hour),
			wantShouldMerge:    true,
			wantStart:          now.Add(-30 * time.Minute),
			wantEnd:            now.Add(time.Hour),
			wantReservationIDs: []int64{1, 2},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeAdjacentReservations(tt.reservations, tt.inputStart, tt.inputEnd)

			assert.Equal(t, tt.wantShouldMerge, got.ShouldMerge)
			assert.WithinDuration(t, tt.wantStart, got.MergedStartAt, time.Second)
			assert.WithinDuration(t, tt.wantEnd, got.MergedEndAt, time.Second)
			assert.ElementsMatch(t, tt.wantReservationIDs, got.MergedReservationIDs)
		})
	}
}

func TestMergeAdjacentReservations(t *testing.T) {
	assert := assert.New(t)
	mockRepo := mocks.NewMockReservationRepository(t)
	mockSpotRepo := mocks.NewMockSpotRepository(t)
	mockComm := mocks.NewMockCommunicationService(t)
	adapter := NewAdapter(mockSpotRepo, mockRepo, mockComm)
	now := time.Now()

	createRes := func(id int64, start, end time.Time) *reservation.ReservationWithSpot {
		return &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{ID: id, StartAt: start, EndAt: end},
			Spot:        reservation.Spot{ID: 1, Name: "test-spot"},
		}
	}

	req := book.BookRequest{
		Guild:   &guild.Guild{ID: "g1"},
		Member:  &member.Member{ID: "m1"},
		Spot:    "test-spot",
		StartAt: now,
		EndAt:   now.Add(time.Hour),
	}

	reservations := []*reservation.ReservationWithSpot{
		createRes(10, now, now.Add(time.Hour)),
		createRes(11, now.Add(time.Hour), now.Add(2*time.Hour)),
	}

	mockRepo.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, req.Guild, req.Member).Return(reservations, nil)
	mockRepo.On("UpdateReservation", mocks.ContextMock, int64(10), now, now.Add(2*time.Hour)).Return(nil)
	mockRepo.On("DeletePresentMemberReservation", mocks.ContextMock, req.Guild, req.Member, int64(11)).Return(nil)

	err := adapter.mergeAdjacentReservations(context.Background(), req)

	assert.NoError(err)
	mockRepo.AssertExpectations(t)
}

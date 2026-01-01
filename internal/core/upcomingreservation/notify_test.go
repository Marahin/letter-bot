package upcomingreservation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/upcomingreservation"
)

func TestNotifyUpcomingReservationsSuccess(t *testing.T) {
	// given
	assert := assert.New(t)
	mockResRepo := mocks.NewMockReservationRepository(t)
	mockMemberRepo := mocks.NewMockMemberRepository(t)
	mockCommService := mocks.NewMockCommunicationService(t)
	mockOnlineCheckService := mocks.NewMockOnlineCheckService(t)
	logger := zap.NewNop().Sugar()
	adapter := upcomingreservation.NewAdapter(mockResRepo, mockMemberRepo, mockCommService, mockOnlineCheckService).WithLogger(logger)
	g := &guild.Guild{ID: "test-guild-id"}
	ctx := context.Background()

	resWithSpot := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              1,
			AuthorDiscordID: "user-1",
			StartAt:         time.Now().Add(10 * time.Minute),
		},
		Spot: reservation.Spot{Name: "Spot 1"},
	}
	reservations := []*reservation.ReservationWithSpot{resWithSpot}

	mockResRepo.EXPECT().SelectReservationsForReservationStartsNotification(ctx, g.ID).Return(reservations, nil).Once()

	m := &member.Member{Username: "User 1"}
	mockMemberRepo.EXPECT().GetMemberByGuildAndId(g, "user-1").Return(m, nil).Once()

	mockOnlineCheckService.EXPECT().IsOnline(g.ID, m.Nick).Return(false).Once()

	mockCommService.EXPECT().NotifyUpcomingReservation(g, m, "Spot 1", resWithSpot.Reservation.StartAt).Return(nil).Once()

	completionSignal := make(chan struct{})
	mockResRepo.EXPECT().UpdateReservationStartsNotificationSent(mock.Anything, int64(1)).
		Run(func(ctx context.Context, id int64) {
			close(completionSignal)
		}).
		Return(nil).Once()

	// when
	err := adapter.NotifyUpcomingReservations(ctx, g)

	// assert
	assert.NoError(err)
	select {
	case <-completionSignal:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async notification processing")
	}
}

func TestNotifyUpcomingReservationsFailOnRepoSelection(t *testing.T) {
	// given
	assert := assert.New(t)
	mockResRepo := mocks.NewMockReservationRepository(t)
	mockMemberRepo := mocks.NewMockMemberRepository(t)
	mockCommService := mocks.NewMockCommunicationService(t)
	mockOnlineCheckService := mocks.NewMockOnlineCheckService(t)
	logger := zap.NewNop().Sugar()
	adapter := upcomingreservation.NewAdapter(mockResRepo, mockMemberRepo, mockCommService, mockOnlineCheckService).WithLogger(logger)
	g := &guild.Guild{ID: "test-guild-id"}
	ctx := context.Background()

	mockResRepo.EXPECT().SelectReservationsForReservationStartsNotification(ctx, g.ID).Return(nil, errors.New("db error")).Once()

	// when
	err := adapter.NotifyUpcomingReservations(ctx, g)

	// assert
	assert.Error(err)
	assert.Equal("db error", err.Error())
}

func TestNotifyUpcomingReservationsFailOnMemberLookup(t *testing.T) {
	// given
	assert := assert.New(t)
	mockResRepo := mocks.NewMockReservationRepository(t)
	mockMemberRepo := mocks.NewMockMemberRepository(t)
	mockCommService := mocks.NewMockCommunicationService(t)
	mockOnlineCheckService := mocks.NewMockOnlineCheckService(t)
	logger := zap.NewNop().Sugar()
	adapter := upcomingreservation.NewAdapter(mockResRepo, mockMemberRepo, mockCommService, mockOnlineCheckService).WithLogger(logger)
	g := &guild.Guild{ID: "test-guild-id"}
	ctx := context.Background()

	resWithSpot := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: 2, AuthorDiscordID: "user-2"},
		Spot:        reservation.Spot{Name: "Spot 2"},
	}
	reservations := []*reservation.ReservationWithSpot{resWithSpot}

	mockResRepo.EXPECT().SelectReservationsForReservationStartsNotification(ctx, g.ID).Return(reservations, nil).Once()

	lookupCalled := make(chan struct{})
	mockMemberRepo.EXPECT().GetMemberByGuildAndId(g, "user-2").
		Run(func(g *guild.Guild, memberId string) {
			close(lookupCalled)
		}).
		Return(nil, errors.New("member not found")).Once()

	// when
	err := adapter.NotifyUpcomingReservations(ctx, g)

	// assert
	assert.NoError(err)
	select {
	case <-lookupCalled:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for member lookup")
	}
}

func TestNotifyUpcomingReservationsFailOnNotification(t *testing.T) {
	// given
	assert := assert.New(t)
	mockResRepo := mocks.NewMockReservationRepository(t)
	mockMemberRepo := mocks.NewMockMemberRepository(t)
	mockCommService := mocks.NewMockCommunicationService(t)
	mockOnlineCheckService := mocks.NewMockOnlineCheckService(t)
	logger := zap.NewNop().Sugar()
	adapter := upcomingreservation.NewAdapter(mockResRepo, mockMemberRepo, mockCommService, mockOnlineCheckService).WithLogger(logger)
	g := &guild.Guild{ID: "test-guild-id"}
	ctx := context.Background()

	resWithSpot := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              3,
			AuthorDiscordID: "user-3",
			StartAt:         time.Now().Add(10 * time.Minute),
		},
		Spot: reservation.Spot{Name: "Spot 3"},
	}
	reservations := []*reservation.ReservationWithSpot{resWithSpot}

	mockResRepo.EXPECT().SelectReservationsForReservationStartsNotification(ctx, g.ID).Return(reservations, nil).Once()

	m := &member.Member{Username: "User 3"}
	mockMemberRepo.EXPECT().GetMemberByGuildAndId(g, "user-3").Return(m, nil).Once()

	mockOnlineCheckService.EXPECT().IsOnline(g.ID, m.Nick).Return(false).Once()

	notifyCalled := make(chan struct{})
	mockCommService.EXPECT().NotifyUpcomingReservation(g, m, "Spot 3", resWithSpot.Reservation.StartAt).
		Run(func(guild *guild.Guild, member *member.Member, spotName string, startAt time.Time) {
			close(notifyCalled)
		}).
		Return(errors.New("dm closed")).Once()

	// when
	err := adapter.NotifyUpcomingReservations(ctx, g)

	// assert
	assert.NoError(err)
	select {
	case <-notifyCalled:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification attempt")
	}
}

func TestNotifyUpcomingReservationsFailOnStatusUpdate(t *testing.T) {
	// given
	assert := assert.New(t)
	mockResRepo := mocks.NewMockReservationRepository(t)
	mockMemberRepo := mocks.NewMockMemberRepository(t)
	mockCommService := mocks.NewMockCommunicationService(t)
	mockOnlineCheckService := mocks.NewMockOnlineCheckService(t)
	logger := zap.NewNop().Sugar()
	adapter := upcomingreservation.NewAdapter(mockResRepo, mockMemberRepo, mockCommService, mockOnlineCheckService).WithLogger(logger)
	g := &guild.Guild{ID: "test-guild-id"}
	ctx := context.Background()

	resWithSpot := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              4,
			AuthorDiscordID: "user-4",
			StartAt:         time.Now().Add(10 * time.Minute),
		},
		Spot: reservation.Spot{Name: "Spot 4"},
	}
	reservations := []*reservation.ReservationWithSpot{resWithSpot}

	mockResRepo.EXPECT().SelectReservationsForReservationStartsNotification(ctx, g.ID).Return(reservations, nil).Once()

	m := &member.Member{Username: "User 4"}
	mockMemberRepo.EXPECT().GetMemberByGuildAndId(g, "user-4").Return(m, nil).Once()

	mockOnlineCheckService.EXPECT().IsOnline(g.ID, m.Nick).Return(false).Once()

	mockCommService.EXPECT().NotifyUpcomingReservation(g, m, "Spot 4", resWithSpot.Reservation.StartAt).Return(nil).Once()

	updateCalled := make(chan struct{})
	mockResRepo.EXPECT().UpdateReservationStartsNotificationSent(mock.Anything, int64(4)).
		Run(func(ctx context.Context, id int64) {
			close(updateCalled)
		}).
		Return(errors.New("update failed")).Once()

	// when
	err := adapter.NotifyUpcomingReservations(ctx, g)

	// assert
	assert.NoError(err)
	select {
	case <-updateCalled:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for update status attempt")
	}
}

func TestNotifyUpcomingReservationsSkipIfOnline(t *testing.T) {
	// given
	assert := assert.New(t)
	mockResRepo := mocks.NewMockReservationRepository(t)
	mockMemberRepo := mocks.NewMockMemberRepository(t)
	mockCommService := mocks.NewMockCommunicationService(t)
	mockOnlineCheckService := mocks.NewMockOnlineCheckService(t)
	logger := zap.NewNop().Sugar()
	adapter := upcomingreservation.NewAdapter(mockResRepo, mockMemberRepo, mockCommService, mockOnlineCheckService).WithLogger(logger)
	g := &guild.Guild{ID: "test-guild-id"}
	ctx := context.Background()

	resWithSpot := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              1,
			AuthorDiscordID: "user-1",
			StartAt:         time.Now().Add(10 * time.Minute),
		},
		Spot: reservation.Spot{Name: "Spot 1"},
	}
	reservations := []*reservation.ReservationWithSpot{resWithSpot}

	mockResRepo.EXPECT().SelectReservationsForReservationStartsNotification(ctx, g.ID).Return(reservations, nil).Once()

	m := &member.Member{Username: "User 1", Nick: "CharName"}
	mockMemberRepo.EXPECT().GetMemberByGuildAndId(g, "user-1").Return(m, nil).Once()

	mockOnlineCheckService.EXPECT().IsOnline(g.ID, "CharName").Return(true).Once()

	updateCalled := make(chan struct{})
	mockResRepo.EXPECT().UpdateReservationStartsNotificationSent(mock.Anything, int64(1)).
		Run(func(ctx context.Context, id int64) {
			close(updateCalled)
		}).
		Return(nil).Once()

	// when
	err := adapter.NotifyUpcomingReservations(ctx, g)

	// assert
	assert.NoError(err)
	select {
	case <-updateCalled:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for update status")
	}
}

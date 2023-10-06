package booking

import (
	"context"
	"fmt"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"time"

	"github.com/stretchr/testify/mock"
)

var ContextMock = mock.AnythingOfType(fmt.Sprintf("%T", context.Background()))

type MockSpotRepo struct {
	mock.Mock
}

func (a *MockSpotRepo) SelectAllSpots(ctx context.Context) ([]*spot.Spot, error) {
	args := a.Called(ctx)
	return args.Get(0).([]*spot.Spot), args.Error(1)
}

type MockReservationRepo struct {
	mock.Mock
}

func (a *MockReservationRepo) Find(ctx context.Context, id int64) (*reservation.Reservation, error) {
	args := a.Called(ctx, id)
	return args.Get(0).(*reservation.Reservation), args.Error(1)
}

func (a *MockReservationRepo) SelectUpcomingReservationsWithSpot(ctx context.Context, guildId string) ([]*reservation.ReservationWithSpot, error) {
	args := a.Called(ctx, guildId)
	return args.Get(0).([]*reservation.ReservationWithSpot), args.Error(1)
}

func (a *MockReservationRepo) SelectOverlappingReservations(ctx context.Context, spot string, startAt time.Time, endAt time.Time, guildId string) ([]*reservation.Reservation, error) {
	args := a.Called(ctx, spot, startAt, endAt, guildId)

	return args.Get(0).([]*reservation.Reservation), args.Error(1)
}

func (a *MockReservationRepo) CreateAndDeleteConflicting(ctx context.Context, member *discord.Member, guild *discord.Guild, conflicts []*reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) ([]*reservation.Reservation, error) {
	args := a.Called(ctx, member, guild, conflicts, spotId, startAt, endAt)

	return args.Get(0).([]*reservation.Reservation), args.Error(1)

}

func (a *MockReservationRepo) SelectUpcomingMemberReservationsWithSpots(ctx context.Context, guild *discord.Guild, member *discord.Member) ([]*reservation.ReservationWithSpot, error) {
	args := a.Called(ctx, guild, member)

	return args.Get(0).([]*reservation.ReservationWithSpot), args.Error(1)

}

func (a *MockReservationRepo) DeletePresentMemberReservation(ctx context.Context, g *discord.Guild, m *discord.Member, reservationId int64) error {
	args := a.Called(ctx, g, m, reservationId)

	return args.Error(0)
}

func (a *MockReservationRepo) FindReservationWithSpot(ctx context.Context, id int64, guildID, authorDiscordID string) (*reservation.ReservationWithSpot, error) {
	args := a.Called(ctx, id, guildID, authorDiscordID)

	return args.Get(0).(*reservation.ReservationWithSpot), args.Error(1)
}

package mocks

import (
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"time"

	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/core/dto/reservation"
)

type MockBookingService struct {
	mock.Mock
}

func (a *MockBookingService) Book(m *member.Member, g *guild.Guild, spotName string, startAt time.Time, endAt time.Time, overbook bool, hasPermissions bool) ([]*reservation.ClippedOrRemovedReservation, error) {
	args := a.Called(m, g, spotName, startAt, endAt, overbook, hasPermissions)

	return args.Get(0).([]*reservation.ClippedOrRemovedReservation), args.Error(1)
}

func (a *MockBookingService) FindAvailableSpots(filter string) ([]string, error) {
	args := a.Called(filter)

	return args.Get(0).([]string), args.Error(1)
}

func (a *MockBookingService) GetSuggestedHours(baseTime time.Time, filter string) []string {
	args := a.Called(baseTime, filter)

	return args.Get(0).([]string)
}

func (a *MockBookingService) UnbookAutocomplete(g *guild.Guild, m *member.Member, filter string) ([]*reservation.ReservationWithSpot, error) {
	args := a.Called(g, m, filter)

	return args.Get(0).([]*reservation.ReservationWithSpot), args.Error(1)
}

func (a *MockBookingService) Unbook(g *guild.Guild, m *member.Member, reservationId int64) (*reservation.ReservationWithSpot, error) {
	args := a.Called(g, m, reservationId)

	return args.Get(0).(*reservation.ReservationWithSpot), args.Error(1)
}

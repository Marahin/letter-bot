package mocks

import (
	"spot-assistant/internal/core/dto/reservation"
	dto "spot-assistant/internal/core/dto/summary"

	"github.com/stretchr/testify/mock"
)

type MockSummaryService struct {
	mock.Mock
}

func (a *MockSummaryService) PrepareSummary(reservations []*reservation.ReservationWithSpot) (*dto.Summary, error) {
	args := a.Called(reservations)

	return args.Get(0).(*dto.Summary), args.Error(1)
}

func (a *MockSummaryService) PrepareSpotSummary(reservations []*reservation.ReservationWithSpot, spotName string) (*dto.Summary, error) {
	args := a.Called(reservations, spotName)

	return args.Get(0).(*dto.Summary), args.Error(1)
}

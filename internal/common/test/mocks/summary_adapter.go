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

func (m *MockSummaryService) RefreshOnlinePlayers() error {
	args := m.Called()
	return args.Error(0)
}

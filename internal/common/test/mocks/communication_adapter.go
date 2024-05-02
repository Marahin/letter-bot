package mocks

import (
	"github.com/stretchr/testify/mock"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/summary"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

type MockCommunicationService struct {
	mock.Mock
}

func (a *MockCommunicationService) NotifyOverbookedMember(request book.BookRequest, res *reservation.ClippedOrRemovedReservation) {
	a.Called(request, res)
}

func (a *MockCommunicationService) SendGuildSummary(guild *guild.Guild, summary *summary.Summary) error {
	args := a.Called(guild, summary)
	return args.Error(0)
}

func (a *MockCommunicationService) SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error {
	args := a.Called(request, summary)
	return args.Error(0)
}

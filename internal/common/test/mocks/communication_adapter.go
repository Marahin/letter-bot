package mocks

import (
	"github.com/stretchr/testify/mock"
	"spot-assistant/internal/core/dto/summary"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

type MockCommunicationService struct {
	mock.Mock
}

func (a *MockCommunicationService) NotifyOverbookedMember(member *discord.Member, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) {
	a.Called(member, request, res)
}

func (a *MockCommunicationService) SendGuildSummary(guild *discord.Guild, summary *summary.Summary) error {
	args := a.Called(guild, summary)
	return args.Error(0)
}

func (a *MockCommunicationService) SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error {
	args := a.Called(request, summary)
	return args.Error(0)
}

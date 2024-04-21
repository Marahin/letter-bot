package mocks

import (
	"github.com/stretchr/testify/mock"

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

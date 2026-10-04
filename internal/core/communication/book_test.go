package communication

import (
	"testing"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func TestAdapter_NotifyOverbookedMember(t *testing.T) {
	// given
	m := &member.Member{
		ID:       "conflicting-author-id",
		Username: "sample-member",
		Nick:     "sample-nickname",
	}
	g := &guild.Guild{
		ID:   "123",
		Name: "sample-guild",
	}
	request := book.BookRequest{
		Guild:  g,
		Member: m,
	}
	res := &reservation.ClippedOrRemovedReservation{
		Original: &reservation.Reservation{
			AuthorDiscordID: "conflicting-author-id",
		},
	}
	memberOperations := mocks.NewMockMemberRepository(t)
	memberOperations.On("GetMemberByGuildAndID", g, res.Original.AuthorDiscordID).Return(m, nil).Once()
	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("SendDMOverbookedNotification", m, request, res).Return(nil).Once()
	adapter := NewAdapter(botOperations, memberOperations)

	// when
	adapter.NotifyOverbookedMember(request, res)

	// assert
	botOperations.AssertExpectations(t)
}

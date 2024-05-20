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
	member := &member.Member{
		ID:       "conflicting-author-id",
		Username: "sample-member",
		Nick:     "sample-nickname",
	}
	guild := &guild.Guild{
		ID:   "123",
		Name: "sample-guild",
	}
	request := book.BookRequest{
		Guild:  guild,
		Member: member,
	}
	res := &reservation.ClippedOrRemovedReservation{
		Original: &reservation.Reservation{
			AuthorDiscordID: "conflicting-author-id",
		},
	}
	memberOperations := new(mocks.MockBot)
	memberOperations.On("GetMemberByGuildAndId", guild, res.Original.AuthorDiscordID).Return(member, nil).Once()
	botOperations := new(mocks.MockBot)
	botOperations.On("SendDMOverbookedNotification", member, request, res).Return(nil).Once()
	adapter := NewAdapter(botOperations, memberOperations)

	// when
	adapter.NotifyOverbookedMember(request, res)

	// assert
	botOperations.AssertExpectations(t)
}

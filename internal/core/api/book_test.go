package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func TestOnBookWithoutConflictingReservations(t *testing.T) {
	// given
	assert := assert.New(t)
	member := &discord.Member{
		ID: "test-member-id",
	}
	guild := &discord.Guild{
		ID: "test-guild-id",
	}
	summaryChannel := &discord.Channel{
		ID:   "test-channel-id",
		Name: "letter-summary",
		Type: discord.ChannelTypeGuildText,
	}
	startAt := time.Now()
	endAt := startAt.Add(2 * time.Hour)
	spotName := "test-spot"
	bookingSrv := new(mocks.MockBookingService)
	bookingSrv.On("Book", member, guild, spotName, startAt, endAt, false, false).Return(make([]*reservation.ClippedOrRemovedReservation, 0), nil)
	defer bookingSrv.AssertExpectations(t)
	reservationRepo := new(mocks.MockReservationRepo)
	reservationRepo.On("SelectUpcomingReservationsWithSpot", mocks.ContextMock, guild.ID).Return(make([]*reservation.ReservationWithSpot, 0), nil)
	defer reservationRepo.AssertExpectations(t)
	botPort := new(mocks.MockBot)
	botPort.On("MemberHasRole", guild, member, "Postman").Return(false)
	botPort.On("FindChannelByName", guild, "letter-summary").Return(summaryChannel, nil)
	summarySrv := new(mocks.MockSummaryService)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when
	res, err := adapter.OnBook(botPort, book.BookRequest{
		Member:  member,
		Guild:   guild,
		StartAt: startAt,
		EndAt:   endAt,
		Spot:    "test-spot",
	})

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.Eventually(func() bool { // wait for asynchronous summary refresh attempt
		return botPort.AssertExpectations(t)
	}, 2*time.Second, 500*time.Millisecond)
}

func TestOnBookWithConflictingReservations(t *testing.T) {
	// given
	assert := assert.New(t)
	member := &discord.Member{
		ID: "test-member-id",
	}
	guild := &discord.Guild{
		ID: "test-guild-id",
	}
	summaryChannel := &discord.Channel{
		ID:   "test-channel-id",
		Name: "letter-summary",
		Type: discord.ChannelTypeGuildText,
	}
	startAt := time.Now()
	endAt := startAt.Add(2 * time.Hour)
	spot := reservation.Spot{
		ID:   1,
		Name: "test-spot",
	}
	conflictingMember := &discord.Member{
		ID:       "test-conflicting-author-id",
		Username: "conflicting-author",
		Nick:     "conflicting-author",
	}
	conflictingReservations := []*reservation.ClippedOrRemovedReservation{
		{
			Original: &reservation.Reservation{
				ID:              1,
				Author:          conflictingMember.Nick,
				AuthorDiscordID: conflictingMember.ID,
				StartAt:         startAt,
				EndAt:           endAt,
				SpotID:          spot.ID,
				GuildID:         guild.ID,
			},
			New: []*reservation.Reservation{},
		},
	}
	finalReservations := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				ID:              2,
				AuthorDiscordID: member.ID,
				StartAt:         startAt,
				EndAt:           endAt,
			},
			Spot: spot,
		},
	}
	outcomeSummary := &summary.Summary{}
	bookingSrv := new(mocks.MockBookingService)
	bookingSrv.On("Book", member, guild, spot.Name, startAt, endAt, false, false).Return(conflictingReservations, nil)
	reservationRepo := new(mocks.MockReservationRepo)
	reservationRepo.On("SelectUpcomingReservationsWithSpot", mocks.ContextMock, guild.ID).Return(finalReservations, nil)
	defer bookingSrv.AssertExpectations(t)
	botPort := new(mocks.MockBot)
	botPort.On("MemberHasRole", guild, member, "Postman").Return(false)
	botPort.On("FindChannelByName", guild, "letter-summary").Return(summaryChannel, nil)
	botPort.On("GetMember", guild, conflictingMember.ID).Return(conflictingMember, nil)
	botPort.On("SendLetterMessage", guild, summaryChannel, outcomeSummary).Return(nil)
	botPort.On("SendDM", conflictingMember, "Your reservation was overbooked by <@!test-member-id>\n* <@!test-conflicting-author-id> test-spot has been entirely removed").Return(nil)
	summarySrv := new(mocks.MockSummaryService)
	summarySrv.On("PrepareSummary", finalReservations).Return(outcomeSummary, nil)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when
	res, err := adapter.OnBook(botPort, book.BookRequest{
		Member:  member,
		Guild:   guild,
		StartAt: startAt,
		EndAt:   endAt,
		Spot:    "test-spot",
	})

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.Eventually(func() bool { // wait for asynchronous summary refresh attempt
		return botPort.AssertExpectations(t) && reservationRepo.AssertExpectations(t) && summarySrv.AssertExpectations(t)
	}, 2*time.Second, 500*time.Millisecond)
}

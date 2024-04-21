package api

import (
	"fmt"
	"github.com/stretchr/testify/mock"
	stringsHelper "spot-assistant/internal/common/strings"
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
	summarySrv := new(mocks.MockSummaryService)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)
	botPort := new(mocks.MockBot)
	botPort.On("WithEventHandler", adapter).Return(botPort)
	botPort.On("MemberHasRole", guild, member, "Postman").Return(false)
	botPort.On("FindChannelByName", guild, "letter-summary").Return(summaryChannel, nil)
	botPort.On("WithEventHandler", adapter).Return(botPort)
	adapter.WithBot(botPort)

	// when
	res, err := adapter.OnBook(book.BookRequest{
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
	summarySrv := new(mocks.MockSummaryService)
	summarySrv.On("PrepareSummary", finalReservations).Return(outcomeSummary, nil)
	botPort := new(mocks.MockBot)
	botPort.On("WithEventHandler", mock.AnythingOfType("*api.Application")).Return(botPort)
	botPort.On("MemberHasRole", guild, member, "Postman").Return(false)
	botPort.On("FindChannelByName", guild, "letter-summary").Return(summaryChannel, nil)
	botPort.On("GetMember", guild, conflictingMember.ID).Return(conflictingMember, nil)
	botPort.On("SendLetterMessage", guild, summaryChannel, outcomeSummary).Return(nil)
	botPort.On("SendDM", conflictingMember, fmt.Sprintf("Your reservation was overbooked by <@!test-member-id>\n* <@!test-conflicting-author-id> test-spot has been entirely removed (originally: **%s - %s**)", conflictingReservations[0].Original.StartAt.Format(stringsHelper.DC_LONG_TIME_FORMAT), conflictingReservations[0].Original.EndAt.Format(stringsHelper.DC_LONG_TIME_FORMAT))).Return(nil)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv).WithBot(botPort)

	// when
	res, err := adapter.OnBook(book.BookRequest{
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

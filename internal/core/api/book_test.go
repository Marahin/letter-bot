package api

import (
	"github.com/stretchr/testify/mock"
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
	request := book.BookRequest{
		Member:  member,
		Guild:   guild,
		StartAt: startAt,
		EndAt:   endAt,
		Spot:    "test-spot",
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
	communicationSrv := new(mocks.MockCommunicationService)
	communicationSrv.On("NotifyOverbookedMember", conflictingMember, request, conflictingReservations[0]).Return()
	communicationSrv.On("SendGuildSummary", guild, outcomeSummary).Return(nil)
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
	botPort.On("GetMember", guild, conflictingMember.ID).Return(conflictingMember, nil)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv).WithBot(botPort).WithCommunication(communicationSrv)

	// when
	res, err := adapter.OnBook(request)

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.Eventually(func() bool { // wait for asynchronous summary refresh attempt
		return botPort.AssertExpectations(t) &&
			reservationRepo.AssertExpectations(t) &&
			summarySrv.AssertExpectations(t) &&
			communicationSrv.AssertExpectations(t)
	}, 2*time.Second, 500*time.Millisecond)
}

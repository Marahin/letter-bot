package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

func TestOnBookWithoutConflictingWithoutOverbook(t *testing.T) {
	// given
	assert := assert.New(t)
	const overbook = false
	const hasPrivilegedRole = false
	member := &discord.Member{
		ID: "test-member-id",
	}
	guild := &discord.Guild{
		ID: "test-guild-id",
	}
	spot := "test-spot"
	startAt := time.Now()
	endAt := time.Now().Add(1 * time.Hour)
	bookingSrv := new(mocks.MockBookingService)
	bookingSrv.On("Book", member, guild, spot, startAt, endAt, overbook, hasPrivilegedRole).Return(make([]*reservation.Reservation, 0), nil)
	defer bookingSrv.AssertExpectations(t)
	reservationRepo := new(mocks.MockReservationRepo)
	reservationRepo.On("SelectUpcomingReservationsWithSpot", mocks.ContextMock, guild.ID).Return(make([]*reservation.ReservationWithSpot, 0), nil)
	summarySrv := new(mocks.MockSummaryService)
	bot := new(mocks.MockBot)
	bot.On("MemberHasRole", guild, member, discord.PrivilegedRole).Return(hasPrivilegedRole)
	bot.On("FindChannel", guild, discord.SummaryChannel).Return(&discord.Channel{}, nil)
	defer bot.AssertExpectations(t)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when
	response, err := adapter.OnBook(bot, book.BookRequest{
		Member:   member,
		Guild:    guild,
		Spot:     spot,
		StartAt:  startAt,
		EndAt:    endAt,
		Overbook: false,
	})

	// then
	assert.Nil(err)
	assert.Equal(spot, response.Spot)
	assert.Equal(startAt, response.StartAt)
	assert.Equal(endAt, response.EndAt)
	assert.Empty(response.ConflictingReservations)

	assert.Eventually(func() bool {
		return reservationRepo.AssertExpectations(t) && summarySrv.AssertExpectations(t)
	}, 2*time.Second, 500*time.Millisecond)
}

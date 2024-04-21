package api

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

func TestOnUnbookAutocomplete(t *testing.T) {
	// given
	assert := assert.New(t)
	summarySrv := new(mocks.MockSummaryService)
	reservationRepo := new(mocks.MockReservationRepo)
	bookingSrv := new(mocks.MockBookingService)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)
	member := &discord.Member{
		ID: "test-member-id",
	}
	guild := &discord.Guild{
		ID: "test-guild-id",
	}
	filter := "some filter text"
	reservations := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				ID:              1,
				Author:          "test-reservation-author",
				StartAt:         time.Now(),
				EndAt:           time.Now().Add(2 * time.Hour),
				SpotID:          1,
				GuildID:         guild.ID,
				AuthorDiscordID: member.ID,
			},
			Spot: reservation.Spot{
				ID:   1,
				Name: "test-spot-name",
			},
		},
	}
	bookingSrv.On("UnbookAutocomplete", guild, member, filter).Return(reservations, nil)
	defer bookingSrv.AssertExpectations(t)

	// when
	res, err := adapter.OnUnbookAutocomplete(book.UnbookAutocompleteRequest{
		Member: member,
		Guild:  guild,
		Value:  filter,
	})

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.Equal(reservations, res.Choices)
}

func TestOnUnbookAutocompleteServiceError(t *testing.T) {
	// given
	assert := assert.New(t)
	summarySrv := new(mocks.MockSummaryService)
	reservationRepo := new(mocks.MockReservationRepo)
	bookingSrv := new(mocks.MockBookingService)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)
	member := &discord.Member{
		ID: "test-member-id",
	}
	guild := &discord.Guild{
		ID: "test-guild-id",
	}
	filter := ""
	bookingSrv.On("UnbookAutocomplete", guild, member, filter).Return([]*reservation.ReservationWithSpot{}, errors.New("test-error"))
	defer bookingSrv.AssertExpectations(t)

	// when
	_, err := adapter.OnUnbookAutocomplete(book.UnbookAutocompleteRequest{
		Member: member,
		Guild:  guild,
		Value:  filter,
	})

	// assert
	assert.NotNil(err)
}

func TestUnbook(t *testing.T) {
	// given
	assert := assert.New(t)
	summarySrv := new(mocks.MockSummaryService)
	reservationRepo := new(mocks.MockReservationRepo)
	request := book.UnbookRequest{
		Guild: &discord.Guild{
			ID:   "test-guild-id",
			Name: "test-guild",
		},
		Member: &discord.Member{
			ID: "test-member-id",
		},
		ReservationID: 1,
	}
	existingReservation := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: 1},
		Spot:        reservation.Spot{ID: 1},
	}
	bot := new(mocks.MockBot)
	bot.On("FindChannelByName", request.Guild, "letter-summary").Return(&discord.Channel{Name: "letter-summary"}, nil)
	bookingSrv := new(mocks.MockBookingService)
	bookingSrv.On("Unbook", request.Guild, request.Member, request.ReservationID).Return(existingReservation, nil)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)
	reservationRepo.On("SelectUpcomingReservationsWithSpot", mocks.ContextMock, request.Guild.ID).Return([]*reservation.ReservationWithSpot{}, nil)

	// when
	res, err := adapter.OnUnbook(bot, request)

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.Equal(existingReservation, res)

	assert.Eventually(func() bool {
		return summarySrv.AssertExpectations(t) && bot.AssertExpectations(t) &&
			reservationRepo.AssertExpectations(t) && bookingSrv.AssertExpectations(t)
	}, 5*time.Second, 100*time.Millisecond)

}

func TestUnbookOnError(t *testing.T) {
	// given
	assert := assert.New(t)
	summarySrv := new(mocks.MockSummaryService)
	reservationRepo := new(mocks.MockReservationRepo)
	request := book.UnbookRequest{
		Guild: &discord.Guild{
			ID:   "test-guild-id",
			Name: "test-guild",
		},
		Member: &discord.Member{
			ID: "test-member-id",
		},
		ReservationID: 1,
	}
	existingReservation := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: 1},
		Spot:        reservation.Spot{ID: 1},
	}
	bot := new(mocks.MockBot)
	defer bot.AssertExpectations(t)
	bookingSrv := new(mocks.MockBookingService)
	bookingSrv.On("Unbook", request.Guild, request.Member, request.ReservationID).Return(existingReservation, errors.New("test-error")).Times(0)
	defer bookingSrv.AssertExpectations(t)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when
	_, err := adapter.OnUnbook(bot, request)

	// assert
	assert.NotNil(err)
}

package api

import (
	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateGuildSummary(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	summary := &summary.Summary{
		Title: "summary",
	}
	summaryCh := &discord.Channel{ID: "test-channel-id", Name: "letter-summary"}
	reservations := []*reservation.ReservationWithSpot{
		{
			Spot: reservation.Spot{
				ID:   1,
				Name: "test-spot-name",
			},
			Reservation: reservation.Reservation{
				ID:      1,
				SpotID:  1,
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
			},
		},
	}
	mockBot := new(mocks.MockBot)
	mockBot.On("SendLetterMessage", guild, summaryCh, summary).Return(nil)
	mockBot.On("FindChannelByName", guild, "letter-summary").Return(summaryCh, nil)
	mockReservationRepo := new(mocks.MockReservationRepo)
	mockReservationRepo.On("SelectUpcomingReservationsWithSpot", mocks.ContextMock, guild.ID).Return(reservations, nil)
	mockSummarySrv := new(mocks.MockSummaryService)
	mockSummarySrv.On("PrepareSummary", reservations).Return(summary, nil)
	mockBookingSrv := new(mocks.MockBookingService)
	adapter := NewApplication(mockReservationRepo, mockSummarySrv, mockBookingSrv)

	// when
	err := adapter.UpdateGuildSummary(mockBot, guild)

	// assert
	assert.Nil(err)
	mockReservationRepo.AssertExpectations(t)
	mockSummarySrv.AssertExpectations(t)
	mockBot.AssertExpectations(t)
	mockBookingSrv.AssertExpectations(t)
}

func TestOnPrivateSummaryWithSpots(t *testing.T) {
	// given
	assert := assert.New(t)

	request := summary.PrivateSummaryRequest{
		UserID:  45,
		GuildID: 1,
		Spots:   []string{"test-spot-name", "test-resp-2", "test-resp-3"},
	}

	reservations := []*reservation.ReservationWithSpot{
		{
			Spot: reservation.Spot{
				ID:   1,
				Name: "test-spot-name",
			},
			Reservation: reservation.Reservation{
				ID:      1,
				SpotID:  1,
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
			},
		},
	}

	summary := summary.Summary{
		Title:  "summary",
		URL:    "https://discord.gg/spot-assistant",
		Footer: "test-footer",
	}

	mockBot := new(mocks.MockBot)
	mockBot.On("PrepareSummary", reservations).Return(summary)
	mockReservationRepo := new(mocks.MockReservationRepo)
	mockReservationRepo.On("SelectUpcomingReservationsWithSpotBySpots",
		mocks.ContextMock,
		strconv.FormatInt(request.GuildID, 10),
		request.Spots).Return(reservations)
	mockSummarySrv := new(mocks.MockSummaryService)
	mockSummarySrv.On("PrepareSummary", reservations).Return(summary, nil)
	mockBookingSrv := new(mocks.MockBookingService)
	adapter := NewApplication(mockReservationRepo, mockSummarySrv, mockBookingSrv)

	// when
	err := adapter.OnPrivateSummary(mockBot, request)

	// assert
	assert.Nil(err)
	mockReservationRepo.AssertExpectations(t)
	mockSummarySrv.AssertExpectations(t)
	mockBot.AssertExpectations(t)
	mockBookingSrv.AssertExpectations(t)
}

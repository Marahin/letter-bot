package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
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
	mockComm := new(mocks.MockCommunicationService)
	mockComm.On("SendGuildSummary", guild, summary).Return(nil)
	mockBot := new(mocks.MockBot)
	mockBot.On("WithEventHandler", mock.AnythingOfType("*api.Application")).Return(mockBot)
	mockReservationRepo := new(mocks.MockReservationRepo)
	mockReservationRepo.On("SelectUpcomingReservationsWithSpot", mocks.ContextMock, guild.ID).Return(reservations, nil)
	mockSummarySrv := new(mocks.MockSummaryService)
	mockSummarySrv.On("PrepareSummary", reservations).Return(summary, nil)
	mockBookingSrv := new(mocks.MockBookingService)
	adapter := NewApplication(mockReservationRepo, mockSummarySrv, mockBookingSrv).WithBot(mockBot).WithCommunication(mockComm)

	// when
	err := adapter.UpdateGuildSummary(guild)

	// assert
	assert.Nil(err)
	mockComm.AssertExpectations(t)
	mockReservationRepo.AssertExpectations(t)
	mockSummarySrv.AssertExpectations(t)
	mockBot.AssertExpectations(t)
	mockBookingSrv.AssertExpectations(t)
}

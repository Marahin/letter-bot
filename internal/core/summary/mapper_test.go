package summary

import (
	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/reservation"
	"testing"
	"time"

	dto "spot-assistant/internal/core/dto/summary"

	"github.com/stretchr/testify/assert"
)

func TestMapReservation(t *testing.T) {
	// Given
	assert := assert.New(t)
	chartSrvMock := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(chartSrvMock, mockOnlineCheckService)
	input := &reservation.Reservation{
		Author:  "test author",
		StartAt: time.Now(),
		EndAt:   time.Now().Add(2 * time.Hour),
		GuildID: "guild1",
	}
	// mock PlayerStatus to return Online for this author
	mockOnlineCheckService.On("PlayerStatus", input.GuildID, input.Author).Return(dto.Online)

	// when
	res := adapter.MapReservation(input)

	// assert
	assert.NotNil(res)
	assert.Equal(input.Author, res.Author)
	assert.Equal(input.StartAt, res.StartAt)
	assert.Equal(input.EndAt, res.EndAt)
	assert.Equal(dto.Online, res.Status)
}

func TestMapReservations(t *testing.T) {
	// Given
	assert := assert.New(t)
	chartSrvMock := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(chartSrvMock, mockOnlineCheckService)
	input := []*reservation.Reservation{
		{
			Author:  "test author",
			StartAt: time.Now(),
			EndAt:   time.Now().Add(2 * time.Hour),
			GuildID: "guild1",
		},
		{
			Author:  "test author 2",
			StartAt: time.Now().Add(5 * time.Minute),
			EndAt:   time.Now().Add(3 * time.Hour),
			GuildID: "guild1",
		},
	}
	// mock PlayerStatus for both authors
	mockOnlineCheckService.On("PlayerStatus", input[0].GuildID, input[0].Author).Return(dto.Online)
	mockOnlineCheckService.On("PlayerStatus", input[1].GuildID, input[1].Author).Return(dto.Offline)

	// when
	res := adapter.MapReservations(input)

	// assert
	assert.Len(res, 2)
	for i, booking := range res {
		assert.Equal(input[i].Author, booking.Author)
		assert.Equal(input[i].StartAt, booking.StartAt)
		assert.Equal(input[i].EndAt, booking.EndAt)
	}
	assert.Equal(dto.Online, res[0].Status)
	assert.Equal(dto.Offline, res[1].Status)
}

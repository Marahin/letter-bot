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
	is := assert.New(t)
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
	is.NotNil(res)
	is.Equal(input.Author, res.Author)
	is.Equal(input.StartAt, res.StartAt)
	is.Equal(input.EndAt, res.EndAt)
	is.Equal(dto.Online, res.Status)
}

func TestMapReservations(t *testing.T) {
	// Given
	is := assert.New(t)
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
	is.Len(res, 2)
	for i, booking := range res {
		is.Equal(input[i].Author, booking.Author)
		is.Equal(input[i].StartAt, booking.StartAt)
		is.Equal(input[i].EndAt, booking.EndAt)
	}
	is.Equal(dto.Online, res[0].Status)
	is.Equal(dto.Offline, res[1].Status)
}

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
	}

	// mock IsOnline to return true for this author
	mockOnlineCheckService.On("IsOnline", input.Author).Return(true)

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
		},
		{
			Author:  "test author 2",
			StartAt: time.Now().Add(5 * time.Minute),
			EndAt:   time.Now().Add(3 * time.Hour),
		},
	}

	// mock IsOnline for both authors
	mockOnlineCheckService.On("IsOnline", input[0].Author).Return(true)
	mockOnlineCheckService.On("IsOnline", input[1].Author).Return(false)

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

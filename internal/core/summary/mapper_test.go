package summary

import (
	"spot-assistant/internal/core/dto/reservation"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMapReservation(t *testing.T) {
	// Given
	assert := assert.New(t)
	chartSrvMock := new(MockChartAdapter)
	adapter := NewAdapter(chartSrvMock)
	input := &reservation.Reservation{
		Author:  "test author",
		StartAt: time.Now(),
		EndAt:   time.Now().Add(2 * time.Hour),
	}

	// when
	res := adapter.MapReservation(input)

	// assert
	assert.NotNil(res)
	assert.Equal(res.Author, input.Author)
	assert.Equal(res.StartAt, input.StartAt)
	assert.Equal(res.EndAt, input.EndAt)
}

func TestMapReservations(t *testing.T) {
	// Given
	assert := assert.New(t)
	chartSrvMock := new(MockChartAdapter)
	adapter := NewAdapter(chartSrvMock)
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

	// when
	res := adapter.MapReservations(input)

	// assert
	assert.Len(input, 2)
	for i, booking := range res {
		assert.Equal(booking.Author, input[i].Author)
		assert.Equal(booking.StartAt, input[i].StartAt)
		assert.Equal(booking.EndAt, input[i].EndAt)
	}
}

package reservation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReservationWithSpot_Label(t *testing.T) {
	// given
	r := &ReservationWithSpot{
		Reservation: Reservation{
			StartAt: time.Date(2023, 8, 10, 14, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2023, 8, 10, 16, 0, 0, 0, time.UTC),
		},
		Spot: Spot{Name: "Library"},
	}

	// when
	label := r.Label()

	// then
	assert.Equal(t, "2023-08-10 14:00 - 2023-08-10 16:00 Library", label)
}

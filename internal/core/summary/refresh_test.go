package summary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/reservation"
	dto "spot-assistant/internal/core/dto/summary"
)

func reserved(spot, author string, start, end time.Time) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{Author: author, StartAt: start, EndAt: end, GuildID: "guild1"},
		Spot:        reservation.Spot{Name: spot},
	}
}

func TestNextRefresh(t *testing.T) {
	now := mapperNow
	cases := map[string]struct {
		res      []*reservation.ReservationWithSpot
		expected time.Time
		ok       bool
	}{
		"none": {nil, time.Time{}, false},
		"the end of an ongoing one": {
			[]*reservation.ReservationWithSpot{reserved("a", "x", now.Add(-time.Hour), now.Add(time.Hour))},
			now.Add(time.Hour), true,
		},
		"the earliest start or end": {
			[]*reservation.ReservationWithSpot{
				reserved("a", "x", now.Add(-time.Hour), now.Add(3*time.Hour)),
				reserved("b", "y", now.Add(2*time.Hour), now.Add(4*time.Hour)),
			},
			now.Add(2 * time.Hour), true,
		},
		"ignores the past and now": {
			[]*reservation.ReservationWithSpot{reserved("a", "x", now.Add(-2*time.Hour), now)},
			time.Time{}, false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			next, ok := NextRefresh(tc.res, now)

			// then
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, next)
		})
	}
}

func TestLedger_GroupsBySpotWithoutTheChart(t *testing.T) {
	// given
	adapter, _ := newMapperAdapter(t)
	later := reserved("b", "y", mapperNow.Add(2*time.Hour), mapperNow.Add(3*time.Hour))
	earlier := reserved("b", "x", mapperNow.Add(time.Hour), mapperNow.Add(2*time.Hour))
	other := reserved("a", "z", mapperNow.Add(time.Hour), mapperNow.Add(2*time.Hour))

	// when
	ledger := adapter.Ledger([]*reservation.ReservationWithSpot{later, other, earlier})

	// then
	assert.Equal(t, dto.Ledger{
		{Spot: "a", Bookings: []*dto.Booking{{Author: "z", StartAt: other.StartAt, EndAt: other.EndAt}}},
		{Spot: "b", Bookings: []*dto.Booking{
			{Author: "x", StartAt: earlier.StartAt, EndAt: earlier.EndAt},
			{Author: "y", StartAt: later.StartAt, EndAt: later.EndAt},
		}},
	}, ledger)
}

func TestLedger_Empty(t *testing.T) {
	// given
	adapter := NewAdapter(mocks.NewMockChartAdapter(t), mocks.NewMockOnlineCheckService(t))

	// when
	ledger := adapter.Ledger(nil)

	// then
	assert.Empty(t, ledger)
}

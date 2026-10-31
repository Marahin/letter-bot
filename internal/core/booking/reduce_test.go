package booking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/reservation"
)

func TestReduceAllAuthorReservationsByLongestPerSpot(t *testing.T) {
	base := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }
	res := func(spot string, from, to int) *reservation.ReservationWithSpot {
		return &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{StartAt: at(from), EndAt: at(to)},
			Spot:        reservation.Spot{Name: spot},
		}
	}
	type span struct {
		spot     string
		from, to int
	}

	cases := map[string]struct {
		input []*reservation.ReservationWithSpot
		want  []span
	}{
		"different spots stay apart": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave", 0, 2), res("Banuta", 0, 2)},
			want:  []span{{"Hero Cave", 0, 2}, {"Banuta", 0, 2}},
		},
		"same spot and same time keeps one": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave", 0, 2), res("Hero Cave", 0, 2)},
			want:  []span{{"Hero Cave", 0, 2}},
		},
		"same start keeps the later end": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave", 0, 2), res("Hero Cave -1", 0, 3)},
			want:  []span{{"Hero Cave", 0, 3}},
		},
		"same end keeps the earlier start": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave (NORTH)", 1, 3), res("Hero Cave", 0, 3)},
			want:  []span{{"Hero Cave (NORTH)", 0, 3}},
		},
		"overlap joins the two": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave", 0, 2), res("Hero Cave", 1, 4)},
			want:  []span{{"Hero Cave", 0, 4}},
		},
		"containing reservation wins": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave", 1, 2), res("Hero Cave", 0, 4)},
			want:  []span{{"Hero Cave", 0, 4}},
		},
		"gap keeps both": {
			input: []*reservation.ReservationWithSpot{res("Hero Cave", 0, 1), res("Hero Cave", 2, 3)},
			want:  []span{{"Hero Cave", 0, 1}, {"Hero Cave", 2, 3}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			got := reduceAllAuthorReservationsByLongestPerSpot(tc.input)

			// then
			spans := make([]span, 0, len(got))
			for _, r := range got {
				spans = append(spans, span{r.Spot.Name, int(r.StartAt.Sub(base).Hours()), int(r.EndAt.Sub(base).Hours())})
			}
			assert.Equal(t, tc.want, spans)
		})
	}
}

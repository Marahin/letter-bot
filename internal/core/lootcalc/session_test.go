package lootcalc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/loot"
)

func TestParseStamp(t *testing.T) {
	// given
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	tests := []struct {
		name string
		in   string
		want time.Time
		ok   bool
	}{
		{name: "summer time", in: "2026-09-25, 20:14:39", want: time.Date(2026, 9, 25, 20, 14, 39, 0, berlin), ok: true},
		{name: "padded", in: " 2026-01-02, 03:04:05 ", want: time.Date(2026, 1, 2, 3, 4, 5, 0, berlin), ok: true},
		{name: "empty", in: ""},
		{name: "garbage", in: "yesterday"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, ok := parseStamp(tt.in, berlin)

			// then
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.True(t, tt.want.Equal(got), "got %v", got)
			}
		})
	}
}

func TestInterval(t *testing.T) {
	// given
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	tests := []struct {
		name     string
		from, to string
		want     time.Duration
		ok       bool
	}{
		{name: "same day", from: "2026-09-25, 20:14:39", to: "2026-09-25, 21:27:43", want: 73*time.Minute + 4*time.Second, ok: true},
		{name: "past midnight", from: "2026-09-25, 23:30:00", to: "2026-09-26, 00:30:00", want: time.Hour, ok: true},
		// The clocks go back at 03:00 on 2026-10-25, so 01:30 to 03:30 lasts three hours.
		{name: "DST day", from: "2026-10-25, 01:30:00", to: "2026-10-25, 03:30:00", want: 3 * time.Hour, ok: true},
		{name: "no header", from: "", to: ""},
		{name: "inverted", from: "2026-09-25, 21:00:00", to: "2026-09-25, 20:00:00"},
		{name: "zero length", from: "2026-09-25, 21:00:00", to: "2026-09-25, 21:00:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			from, to, ok := Interval(tt.from, tt.to, berlin)

			// then
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, to.Sub(from))
			} else {
				assert.True(t, from.IsZero() && to.IsZero())
			}
		})
	}
}

func TestSession_Submission(t *testing.T) {
	// given
	s, err := Parse(vueSample)
	require.NoError(t, err)

	// when
	sub := s.Submission()

	// then
	assert.Equal(t, "2020-06-01, 11:18:39", sub.From)
	assert.Equal(t, "2020-06-01, 12:31:43", sub.To)
	require.Len(t, sub.Members, 3)
	assert.Equal(t, loot.Member{Name: "Killer Potato", Leader: true, Loot: 1816623, Supplies: 528367, Balance: 0, Damage: 4043228, Healing: 398456}, sub.Members[2])
}

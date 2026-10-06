package reservationforms

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClock_Accepts(t *testing.T) {
	tests := map[string][2]int{
		"18:30": {18, 30},
		"9:05":  {9, 5},
		"19.00": {19, 0},
		"19;00": {19, 0},
		"19,15": {19, 15},
		"18h30": {18, 30},
		"18h":   {18, 0},
		"18H":   {18, 0},
		"7":     {7, 0},
		"18":    {18, 0},
		"930":   {9, 30},
		"1830":  {18, 30},
		"24:00": {0, 0},
		"24:30": {0, 30},
		"0:00":  {0, 0},
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			// when
			hour, minute, err := ParseClock(input)

			// then
			require.NoError(t, err)
			assert.Equal(t, want, [2]int{hour, minute})
		})
	}
}

func TestParseClock_TrimsSpaces(t *testing.T) {
	// when
	hour, minute, err := ParseClock("  7:15 ")

	// then
	require.NoError(t, err)
	assert.Equal(t, [2]int{7, 15}, [2]int{hour, minute})
}

func TestParseClock_Rejects(t *testing.T) {
	for _, input := range []string{"", "ab", "25:00", "12:60", "12:5", "123:00", "12345", "+5", "-1", "1:2:3", "ab:cd"} {
		t.Run(input, func(t *testing.T) {
			// when
			_, _, err := ParseClock(input)

			// then
			assert.ErrorIs(t, err, ErrTimeFormat)
		})
	}
}

var berlin = mustLocation("Europe/Berlin")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func at(day, hour, minute int) time.Time {
	return time.Date(2026, 10, day, hour, minute, 0, 0, berlin)
}

func TestNextWindow(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 30, 40, 0, berlin)
	tests := []struct {
		name, start, end string
		wantStart        time.Time
		wantEnd          time.Time
	}{
		{"later today", "19:00", "21:00", at(5, 19, 0), at(5, 21, 0)},
		{"the current minute is today", "18:30", "20:00", at(5, 18, 30), at(5, 20, 0)},
		{"a past time is tomorrow", "18:29", "20:00", at(6, 18, 29), at(6, 20, 0)},
		{"the end after midnight", "23:00", "01:00", at(5, 23, 0), at(6, 1, 0)},
		{"the same start and end is a day", "20:00", "20:00", at(5, 20, 0), at(6, 20, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			startAt, endAt, err := NextWindow(tt.start, tt.end, now)

			// then
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, startAt)
			assert.Equal(t, tt.wantEnd, endAt)
		})
	}
}

func TestNextWindow_RejectsABadTime(t *testing.T) {
	now := at(5, 18, 0)

	// when
	_, _, startErr := NextWindow("x", "20:00", now)
	_, _, endErr := NextWindow("19:00", "x", now)

	// then
	assert.ErrorIs(t, startErr, ErrTimeFormat)
	assert.ErrorIs(t, endErr, ErrTimeFormat)
}

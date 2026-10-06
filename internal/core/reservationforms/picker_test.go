package reservationforms

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
)

func named(names ...string) []*spot.Spot {
	spots := make([]*spot.Spot, 0, len(names))
	for i, name := range names {
		spots = append(spots, &spot.Spot{ID: int64(i + 1), Name: name})
	}
	return spots
}

// numbered makes n respawns whose names sort in the order they are made.
func numbered(n int) []*spot.Spot {
	names := make([]string, 0, n)
	for i := range n {
		names = append(names, fmt.Sprintf("%c%03d", 'A'+i/10, i))
	}
	return named(names...)
}

func groupSizes(groups []reservation.RespawnGroup) []int {
	sizes := make([]int, 0, len(groups))
	for _, g := range groups {
		sizes = append(sizes, len(g.Spots))
	}
	return sizes
}

func TestPageRespawns_Sizes(t *testing.T) {
	tests := []struct {
		name      string
		spots     int
		page      int
		wantPages int
		wantSizes []int
	}{
		{"none", 0, 0, 1, []int{}},
		{"one group", 3, 0, 1, []int{3}},
		{"balanced, not 25/25/3", 53, 0, 1, []int{18, 18, 17}},
		{"a full page", 75, 0, 1, []int{25, 25, 25}},
		{"one more than a page", 76, 1, 2, []int{1}},
		{"two hundred, last page", 200, 2, 3, []int{25, 25}},
		{"a page before the first", 200, -1, 3, []int{25, 25, 25}},
		{"a page after the last", 200, 9, 3, []int{25, 25}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			picker := PageRespawns(numbered(tt.spots), tt.page)

			// then
			assert.Equal(t, tt.wantPages, picker.Pages)
			assert.Equal(t, tt.wantSizes, groupSizes(picker.Groups))
			assert.GreaterOrEqual(t, picker.Page, 0)
			assert.Less(t, picker.Page, picker.Pages)
		})
	}
}

func TestPageRespawns_SortsByNameIgnoringCase(t *testing.T) {
	// given
	spots := named("banuta", "Ankrahmun", "Cyclops", "asura")

	// when
	picker := PageRespawns(spots, 0)

	// then
	require.Len(t, picker.Groups, 1)
	assert.Equal(t, []reservation.Spot{{ID: 2, Name: "Ankrahmun"}, {ID: 4, Name: "asura"}, {ID: 1, Name: "banuta"}, {ID: 3, Name: "Cyclops"}}, picker.Groups[0].Spots)
	assert.Equal(t, "A", picker.Groups[0].From)
	assert.Equal(t, "C", picker.Groups[0].To)
}

func TestPageRespawns_Labels(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  [][2]string
	}{
		{
			"first letters differ",
			append(repeat("Asura", 13), repeat("Hero", 13)...),
			[][2]string{{"A", "A"}, {"H", "H"}},
		},
		{
			"first letters collide",
			append(append(repeat("Banuta", 12), "Kazordoon"), append([]string{"Komodo"}, repeat("Zao", 12)...)...),
			[][2]string{{"B", "Ka"}, {"Ko", "Z"}},
		},
		{
			"long common prefix stops at three",
			append(append(repeat("Asura", 12), "Library -1"), append([]string{"Library -2"}, repeat("Zao", 12)...)...),
			[][2]string{{"A", "Lib"}, {"Lib", "Z"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			picker := PageRespawns(named(tt.names...), 0)

			// then
			got := make([][2]string, 0, len(picker.Groups))
			for _, g := range picker.Groups {
				got = append(got, [2]string{g.From, g.To})
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPageRespawns_LabelsAcrossPages(t *testing.T) {
	// given: the last respawn of page 1 and the first of page 2 share "Ka"
	names := append(append(repeat("Asura", 74), "Kazordoon"), "Kalahar", "Zao")
	names[74], names[75] = "Kalahar", "Kazordoon"

	// when
	first := PageRespawns(named(names...), 0)
	second := PageRespawns(named(names...), 1)

	// then
	assert.Equal(t, "Kal", first.Groups[2].To)
	assert.Equal(t, "Kaz", second.Groups[0].From)
}

func TestPrefix(t *testing.T) {
	assert.Equal(t, "Ab", prefix(" ab dendriel", 3))
	assert.Equal(t, "Ż", prefix("żółw", 1))
	assert.Equal(t, "", prefix("", 2))
}

func repeat(name string, n int) []string {
	names := make([]string, 0, n)
	for i := range n {
		names = append(names, fmt.Sprintf("%s %02d", name, i))
	}
	return names
}

func ranked(id int64, name string, bookings int64, last time.Time) spot.Ranked {
	return spot.Ranked{Spot: spot.Spot{ID: id, Name: name}, Bookings: bookings, LastStartAt: last}
}

func TestUsualRespawns(t *testing.T) {
	// given
	day := time.Date(2026, 10, 1, 18, 0, 0, 0, berlin)
	archivedAt := day
	archived := ranked(9, "Old", 50, day)
	archived.ArchivedAt = &archivedAt
	mine := []spot.Ranked{
		ranked(1, "Asura", 3, day),
		ranked(2, "Banuta", 7, day),
		ranked(3, "Cyclops", 3, day.Add(time.Hour)),
		archived,
	}
	popular := []spot.Ranked{ranked(2, "Banuta", 40, day), ranked(4, "Dragons", 30, day), ranked(5, "Elves", 20, day)}

	// when
	usual := UsualRespawns(mine, popular, 4)

	// then
	assert.Equal(t, []reservation.UsualRespawn{
		{Spot: reservation.Spot{ID: 2, Name: "Banuta"}, Bookings: 7},
		{Spot: reservation.Spot{ID: 3, Name: "Cyclops"}, Bookings: 3},
		{Spot: reservation.Spot{ID: 1, Name: "Asura"}, Bookings: 3},
		{Spot: reservation.Spot{ID: 4, Name: "Dragons"}, Bookings: 0},
	}, usual)
}

func TestUsualRespawns_TiesByName(t *testing.T) {
	// given
	day := time.Date(2026, 10, 1, 18, 0, 0, 0, berlin)

	// when
	usual := UsualRespawns(nil, []spot.Ranked{ranked(2, "banuta", 1, day), ranked(1, "Asura", 1, day)}, 25)

	// then
	require.Len(t, usual, 2)
	assert.Equal(t, "Asura", usual[0].Spot.Name)
	assert.Empty(t, UsualRespawns(nil, nil, 25))
}

func TestStartSlots(t *testing.T) {
	tests := []struct {
		name   string
		now    time.Time
		window int
		first  time.Time
		last   time.Time
	}{
		{"on the hour", at(5, 18, 0), 0, at(5, 18, 30), at(6, 6, 0)},
		{"before the half hour", time.Date(2026, 10, 5, 18, 29, 59, 0, berlin), 0, at(5, 18, 30), at(6, 6, 0)},
		{"on the half hour", at(5, 18, 30), 0, at(5, 19, 0), at(6, 6, 30)},
		{"over midnight", at(5, 23, 50), 0, at(6, 0, 0), at(6, 11, 30)},
		{"the second window", at(5, 18, 0), 1, at(6, 6, 30), at(6, 18, 0)},
		{"the last window", at(5, 18, 0), 3, at(7, 6, 30), at(7, 18, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			slots := StartSlots(tt.now, tt.window)

			// then
			require.Len(t, slots, WindowSlots)
			assert.True(t, tt.first.Equal(slots[0]), "first %s", slots[0])
			assert.True(t, tt.last.Equal(slots[len(slots)-1]), "last %s", slots[len(slots)-1])
		})
	}
}

func clocks(slots []time.Time, n int) []string {
	out := make([]string, 0, n)
	for _, s := range slots[:n] {
		out = append(out, s.In(berlin).Format("15:04 MST"))
	}
	return out
}

func TestStartSlots_DST(t *testing.T) {
	// given
	autumn := time.Date(2026, 10, 25, 1, 10, 0, 0, berlin)
	spring := time.Date(2026, 3, 29, 1, 10, 0, 0, berlin)

	// when
	repeated := StartSlots(autumn, 0)
	skipped := StartSlots(spring, 0)

	// then
	assert.Equal(t, []string{"01:30 CEST", "02:00 CEST", "02:30 CEST", "02:00 CET", "02:30 CET", "03:00 CET"}, clocks(repeated, 6))
	assert.Equal(t, []string{"01:30 CET", "03:00 CEST", "03:30 CEST"}, clocks(skipped, 3))
}

func TestWindowOf(t *testing.T) {
	now := at(5, 18, 0)
	tests := []struct {
		name  string
		start time.Time
		want  int
	}{
		{"before the first slot", at(5, 18, 10), 0},
		{"the last slot of the first window", at(6, 6, 0), 0},
		{"the first slot of the second window", at(6, 6, 30), 1},
		{"far ahead", at(20, 6, 30), Windows - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, WindowOf(tt.start, now))
		})
	}
}

func TestLengths(t *testing.T) {
	// when
	lengths := Lengths()

	// then
	assert.Len(t, lengths, 6)
	assert.Equal(t, SlotStep, lengths[0])
	assert.Equal(t, booking.MaximumReservationLength, lengths[len(lengths)-1])
}

func TestResolveStart(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 45, 30, 0, berlin)
	tests := []struct {
		name    string
		choice  reservation.TimeChoice
		want    time.Time
		wantErr error
	}{
		{"now", reservation.TimeChoice{Now: true, StartAt: at(5, 20, 0)}, at(5, 18, 45), nil},
		{"no start", reservation.TimeChoice{}, time.Time{}, ErrChoiceIncomplete},
		{"a later slot", reservation.TimeChoice{StartAt: at(5, 19, 0)}, at(5, 19, 0), nil},
		{"the current minute", reservation.TimeChoice{StartAt: at(5, 18, 45)}, at(5, 18, 45), nil},
		{"a slot that just passed", reservation.TimeChoice{StartAt: at(5, 18, 30)}, at(5, 18, 45), nil},
		{"an old slot", reservation.TimeChoice{StartAt: at(5, 18, 0)}, time.Time{}, booking.ErrStartInPast},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, err := ResolveStart(tt.choice, now)

			// then
			assert.ErrorIs(t, err, tt.wantErr)
			assert.True(t, tt.want.Equal(got), "got %s", got)
		})
	}
}

func TestStartOptions(t *testing.T) {
	now := at(5, 18, 0)
	booked := []*reservation.Reservation{{Author: "Druid", StartAt: at(5, 19, 0), EndAt: at(5, 20, 0)}}

	t.Run("the first window starts with now", func(t *testing.T) {
		// when
		options := startOptions(now, 0, reservation.TimeChoice{StartAt: at(5, 19, 30)}, time.Time{}, booked)

		// then
		require.Len(t, options, SelectOptions)
		assert.True(t, options[0].Now)
		assert.True(t, options[0].StartAt.Equal(now))
		assert.True(t, options[1].StartAt.Equal(at(5, 18, 30)))
		assert.Nil(t, options[1].BookedBy)
		assert.Same(t, booked[0], options[2].BookedBy, "19:00 is booked")
		assert.Same(t, booked[0], options[3].BookedBy, "19:30 is booked")
		assert.Nil(t, options[4].BookedBy, "20:00 is free again")
		assert.True(t, options[3].Selected)
		assert.Equal(t, 1, countSelected(options))
	})

	t.Run("a later window has no now", func(t *testing.T) {
		// when
		options := startOptions(now, 1, reservation.TimeChoice{Now: true}, time.Time{}, nil)

		// then
		assert.Len(t, options, WindowSlots)
		assert.False(t, options[0].Now)
		assert.Equal(t, 0, countSelected(options), "now is chosen, but not shown")
	})

	t.Run("the current start off the grid comes first", func(t *testing.T) {
		// given
		current := at(5, 18, 10)

		// when
		options := startOptions(now, 0, reservation.TimeChoice{StartAt: current}, current, nil)

		// then
		require.Len(t, options, SelectOptions)
		assert.True(t, options[0].Current)
		assert.True(t, options[0].Selected)
		assert.True(t, options[1].Now)
		assert.True(t, options[SelectOptions-1].StartAt.Equal(at(5, 5, 30).AddDate(0, 0, 1)), "the last slot is dropped")
	})

	t.Run("the current start on the grid is not repeated", func(t *testing.T) {
		// when
		options := startOptions(now, 0, reservation.TimeChoice{StartAt: at(5, 19, 0)}, at(5, 19, 0), nil)

		// then
		assert.False(t, options[0].Current)
		assert.Equal(t, 1, countSelected(options))
	})

	t.Run("the current start of another window is not shown", func(t *testing.T) {
		// when
		options := startOptions(now, 0, reservation.TimeChoice{}, at(6, 9, 10), nil)

		// then
		assert.False(t, options[0].Current)
	})

	t.Run("now is selected", func(t *testing.T) {
		// when
		options := startOptions(now, 0, reservation.TimeChoice{Now: true}, time.Time{}, nil)

		// then
		assert.True(t, options[0].Selected)
		assert.Equal(t, 1, countSelected(options))
	})
}

func countSelected(options []reservation.StartOption) int {
	n := 0
	for _, o := range options {
		if o.Selected {
			n++
		}
	}
	return n
}

func TestLengthOptions(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		// when
		options := lengthOptions(time.Hour, 0, time.Time{}, time.Time{})

		// then
		assert.Len(t, options, 6)
		assert.True(t, options[1].Selected)
	})

	t.Run("the current length off the grid comes first", func(t *testing.T) {
		// when
		options := lengthOptions(70*time.Minute, 70*time.Minute, time.Time{}, time.Time{})

		// then
		require.Len(t, options, 7)
		assert.Equal(t, reservation.LengthOption{Length: 70 * time.Minute, Current: true, Selected: true}, options[0])
	})

	t.Run("an ongoing reservation drops the lengths that already ended", func(t *testing.T) {
		// when
		options := lengthOptions(2*time.Hour, 2*time.Hour, at(5, 17, 0), at(5, 18, 0))

		// then
		require.Len(t, options, 4)
		assert.Equal(t, 90*time.Minute, options[0].Length)
		assert.True(t, options[1].Selected)
	})
}

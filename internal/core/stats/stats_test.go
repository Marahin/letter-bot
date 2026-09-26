package stats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

var (
	berlin, _ = time.LoadLocation("Europe/Berlin")
	now       = time.Date(2026, 9, 26, 14, 30, 0, 0, berlin)
	errBoom   = errors.New("boom")
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, berlin) }

func withExp(res, secs, expRes, expSecs, exp int64) stats.Totals {
	return stats.Totals{Reservations: res, Seconds: secs, ExpReservations: expRes, ExpSeconds: expSecs, Exp: exp}
}

func TestResolveRange_DefaultsToLast30DaysThroughToday(t *testing.T) {
	// when
	rng := ResolveRange(nil, now)

	// then
	require.Len(t, rng.Days, DefaultDays)
	assert.Equal(t, day(2026, 8, 28), rng.From)
	assert.Equal(t, day(2026, 9, 27), rng.To)
	assert.Equal(t, day(2026, 9, 26), rng.Days[len(rng.Days)-1])
}

func TestResolveRange_SpansFromFirstToLastPickedDay(t *testing.T) {
	// given
	picked := []time.Time{day(2026, 9, 10), day(2026, 9, 3).Add(5 * time.Hour), day(2026, 9, 5)}

	// when
	rng := ResolveRange(picked, now)

	// then
	assert.Equal(t, day(2026, 9, 3), rng.From)
	assert.Equal(t, day(2026, 9, 11), rng.To)
	assert.Len(t, rng.Days, 8)
}

func TestResolveRange_CapsAtMaxDaysKeepingTheNewest(t *testing.T) {
	// given
	picked := []time.Time{day(2023, 1, 1), day(2026, 9, 1)}

	// when
	rng := ResolveRange(picked, now)

	// then
	require.Len(t, rng.Days, MaxDays)
	assert.Equal(t, day(2026, 9, 1), rng.Days[MaxDays-1])
	assert.Equal(t, day(2026, 9, 2), rng.To)
}

func TestResolveRange_KeepsCalendarDaysAcrossDST(t *testing.T) {
	// given: the switch to winter time on 2026-10-25 makes that day 25 hours long
	picked := []time.Time{day(2026, 10, 24), day(2026, 10, 26)}

	// when
	rng := ResolveRange(picked, now)

	// then
	require.Len(t, rng.Days, 3)
	for _, d := range rng.Days {
		assert.Zero(t, d.Hour())
	}
}

func TestFillDaily_AddsZeroDaysInOrder(t *testing.T) {
	// given
	days := []time.Time{day(2026, 9, 1), day(2026, 9, 2), day(2026, 9, 3)}
	rows := []stats.Day{{Day: day(2026, 9, 3), Totals: withExp(2, 7200, 0, 0, 0)}}

	// when
	out := FillDaily(rows, days)

	// then
	require.Len(t, out, 3)
	assert.Zero(t, out[0].Reservations)
	assert.Zero(t, out[1].Reservations)
	assert.Equal(t, int64(2), out[2].Reservations)
	assert.Equal(t, day(2026, 9, 1), out[0].Day)
}

func TestSum(t *testing.T) {
	// given
	days := []stats.Day{{Totals: withExp(1, 3600, 1, 3600, 10)}, {Totals: withExp(2, 7200, 0, 0, 0)}}

	// when
	got := Sum(days)

	// then
	assert.Equal(t, withExp(3, 10800, 1, 3600, 10), got)
}

func TestMidnight(t *testing.T) {
	assert.Equal(t, day(2026, 9, 26), Midnight(now))
}

func newService(t *testing.T) (*Service, *mocks.MockStatsRepository, *mocks.MockSpotRepository) {
	repo := mocks.NewMockStatsRepository(t)
	spots := mocks.NewMockSpotRepository(t)
	return New(repo, spots), repo, spots
}

func names(rows []stats.CharacterRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

func byHoursQuery(f stats.Filter, limit int) stats.Query {
	return stats.Query{Filter: f, Sort: stats.Sort{Key: stats.SortHours}, Limit: limit}
}

func TestService_Overview(t *testing.T) {
	// given
	svc, repo, _ := newService(t)
	rng := ResolveRange([]time.Time{day(2026, 9, 1), day(2026, 9, 2)}, now)
	f := stats.Filter{GuildID: "g", From: rng.From, To: rng.To}
	repo.EXPECT().Daily(mock.Anything, f).Return([]stats.Day{{Day: day(2026, 9, 2), Totals: withExp(3, 10800, 1, 3600, 100)}}, nil)
	repo.EXPECT().SpotTotals(mock.Anything, byHoursQuery(f, LeaderboardSize)).Return(stats.Page[stats.SpotRow]{
		Rows:  []stats.SpotRow{{SpotID: 1, Name: "Hero Cave", Totals: withExp(2, 7200, 1, 3600, 100)}},
		Total: 2,
	}, nil)
	repo.EXPECT().PlayerTotals(mock.Anything, byHoursQuery(f, LeaderboardSize)).Return(stats.Page[stats.PlayerRow]{
		Rows:  []stats.PlayerRow{{UserID: "u", Name: "A/B", Totals: withExp(3, 10800, 1, 3600, 100)}},
		Total: 1,
	}, nil)
	repo.EXPECT().CharacterLeaderboards(mock.Anything, f, LeaderboardSize, int64(3*3600)).Return(stats.CharacterBoards{
		ByExp:        []stats.CharacterRow{{Name: "alpha"}, {Name: "Charlie"}},
		ByExpPerHour: []stats.CharacterRow{{Name: "alpha"}},
		Characters:   4,
	}, nil)

	// when
	o, err := svc.Overview(context.Background(), "g", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, withExp(3, 10800, 1, 3600, 100), o.Totals)
	assert.Equal(t, 2, o.Spots)
	assert.Equal(t, 1, o.Players)
	assert.Equal(t, 4, o.Characters)
	require.Len(t, o.Daily, 2)
	assert.Zero(t, o.Daily[0].Reservations)
	assert.Equal(t, "Hero Cave", o.TopSpots[0].Name)
	assert.Equal(t, []string{"alpha"}, names(o.Leaderboards.CharactersByExpPerHour))
	assert.Equal(t, []string{"alpha", "Charlie"}, names(o.Leaderboards.CharactersByExp))
	assert.Len(t, o.Leaderboards.PlayersByHours, 1)
}

func TestService_Overview_Errors(t *testing.T) {
	rng := ResolveRange(nil, now)
	steps := []string{"daily", "spots", "players", "characters"}
	for i := range steps {
		t.Run(steps[i], func(t *testing.T) {
			// given
			svc, repo, _ := newService(t)
			fail := func(n int) error {
				if n == i {
					return errBoom
				}
				return nil
			}
			repo.EXPECT().Daily(mock.Anything, mock.Anything).Return(nil, fail(0))
			if i >= 1 {
				repo.EXPECT().SpotTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.SpotRow]{}, fail(1))
			}
			if i >= 2 {
				repo.EXPECT().PlayerTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.PlayerRow]{}, fail(2))
			}
			if i >= 3 {
				repo.EXPECT().CharacterLeaderboards(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(stats.CharacterBoards{}, fail(3))
			}

			// when
			_, err := svc.Overview(context.Background(), "g", rng)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestService_Lists_PassSortAndLimitToTheRepository(t *testing.T) {
	// given
	svc, repo, _ := newService(t)
	rng := ResolveRange(nil, now)
	byName := stats.Sort{Key: stats.SortName, Asc: true}
	q := stats.Query{Filter: stats.Filter{GuildID: "g", From: rng.From, To: rng.To}, Sort: byName, Limit: 500}
	spotPage := stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{{Name: "a"}}, Total: 7}
	playerPage := stats.Page[stats.PlayerRow]{Rows: []stats.PlayerRow{{Name: "a"}}, Total: 8}
	characterPage := stats.Page[stats.CharacterRow]{Rows: []stats.CharacterRow{{Name: "a"}}, Total: 9}
	repo.EXPECT().SpotTotals(mock.Anything, q).Return(spotPage, nil).Once()
	repo.EXPECT().PlayerTotals(mock.Anything, q).Return(playerPage, nil).Once()
	repo.EXPECT().CharacterTotals(mock.Anything, q).Return(characterPage, nil).Once()

	// when
	spots, err1 := svc.Spots(context.Background(), "g", rng, byName, 500)
	players, err2 := svc.Players(context.Background(), "g", rng, byName, 500)
	characters, err3 := svc.Characters(context.Background(), "g", rng, byName, 500)

	// then
	require.NoError(t, errors.Join(err1, err2, err3))
	assert.Equal(t, spotPage, spots)
	assert.Equal(t, playerPage, players)
	assert.Equal(t, characterPage, characters)
}

func TestService_Spot(t *testing.T) {
	// given
	svc, repo, spots := newService(t)
	rng := ResolveRange(nil, now)
	f := stats.Filter{GuildID: "g", From: rng.From, To: rng.To, SpotID: 4}
	sp := &spot.Spot{ID: 4, Name: "Hero Cave"}
	players := stats.Page[stats.PlayerRow]{Rows: []stats.PlayerRow{{Name: "y"}}, Total: 30}
	characters := stats.Page[stats.CharacterRow]{Rows: []stats.CharacterRow{{Name: "alpha"}}, Total: 1}
	spots.EXPECT().SelectGuildSpotByID(mock.Anything, "g", int64(4)).Return(sp, nil)
	repo.EXPECT().Daily(mock.Anything, f).Return([]stats.Day{{Day: day(2026, 9, 20), Totals: withExp(2, 7200, 0, 0, 0)}}, nil)
	repo.EXPECT().PlayerTotals(mock.Anything, byHoursQuery(f, BreakdownSize)).Return(players, nil)
	repo.EXPECT().CharacterTotals(mock.Anything, byHoursQuery(f, BreakdownSize)).Return(characters, nil)

	// when
	d, err := svc.Spot(context.Background(), "g", 4, rng)

	// then
	require.NoError(t, err)
	assert.Same(t, sp, d.Spot)
	assert.Equal(t, int64(2), d.Totals.Reservations)
	assert.Equal(t, players, d.Players)
	assert.Equal(t, characters, d.Characters)
	assert.Len(t, d.Daily, DefaultDays)
}

func TestService_Spot_NotFoundAndErrors(t *testing.T) {
	rng := ResolveRange(nil, now)
	t.Run("foreign spot", func(t *testing.T) {
		// given
		svc, _, spots := newService(t)
		spots.EXPECT().SelectGuildSpotByID(mock.Anything, "g", int64(9)).Return(nil, ports.ErrNotFound)

		// when
		_, err := svc.Spot(context.Background(), "g", 9, rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	for i, step := range []string{"daily", "players", "characters"} {
		t.Run(step, func(t *testing.T) {
			// given
			svc, repo, spots := newService(t)
			fail := func(n int) error {
				if n == i {
					return errBoom
				}
				return nil
			}
			spots.EXPECT().SelectGuildSpotByID(mock.Anything, "g", int64(4)).Return(&spot.Spot{ID: 4}, nil)
			repo.EXPECT().Daily(mock.Anything, mock.Anything).Return(nil, fail(0))
			if i >= 1 {
				repo.EXPECT().PlayerTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.PlayerRow]{}, fail(1))
			}
			if i >= 2 {
				repo.EXPECT().CharacterTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.CharacterRow]{}, fail(2))
			}

			// when
			_, err := svc.Spot(context.Background(), "g", 4, rng)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestService_Player(t *testing.T) {
	// given
	svc, repo, _ := newService(t)
	rng := ResolveRange(nil, now)
	f := stats.Filter{GuildID: "g", From: rng.From, To: rng.To, UserID: "u1"}
	spots := stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{{Name: "b"}}, Total: 1}
	characters := stats.Page[stats.CharacterRow]{Rows: []stats.CharacterRow{{Name: "alpha"}}, Total: 4}
	repo.EXPECT().LatestPlayerName(mock.Anything, "g", "u1").Return("Quiet Nyx/Storm", nil)
	repo.EXPECT().Daily(mock.Anything, f).Return([]stats.Day{{Day: day(2026, 9, 20), Totals: withExp(2, 10800, 0, 0, 0)}}, nil)
	repo.EXPECT().SpotTotals(mock.Anything, byHoursQuery(f, BreakdownSize)).Return(spots, nil)
	repo.EXPECT().CharacterTotals(mock.Anything, byHoursQuery(f, BreakdownSize)).Return(characters, nil)

	// when
	d, err := svc.Player(context.Background(), "g", "u1", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Quiet Nyx/Storm", d.Name)
	assert.Equal(t, spots, d.Spots)
	assert.Equal(t, int64(10800), d.Totals.Seconds)
	assert.Equal(t, characters, d.Characters)
}

func TestService_Player_NotFoundAndErrors(t *testing.T) {
	rng := ResolveRange(nil, now)
	t.Run("empty id", func(t *testing.T) {
		// given
		svc, _, _ := newService(t)

		// when
		_, err := svc.Player(context.Background(), "g", "", rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	t.Run("unknown user", func(t *testing.T) {
		// given
		svc, repo, _ := newService(t)
		repo.EXPECT().LatestPlayerName(mock.Anything, "g", "x").Return("", ports.ErrNotFound)

		// when
		_, err := svc.Player(context.Background(), "g", "x", rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	for i, step := range []string{"daily", "spots", "characters"} {
		t.Run(step, func(t *testing.T) {
			// given
			svc, repo, _ := newService(t)
			fail := func(n int) error {
				if n == i {
					return errBoom
				}
				return nil
			}
			repo.EXPECT().LatestPlayerName(mock.Anything, "g", "u").Return("n", nil)
			repo.EXPECT().Daily(mock.Anything, mock.Anything).Return(nil, fail(0))
			if i >= 1 {
				repo.EXPECT().SpotTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.SpotRow]{}, fail(1))
			}
			if i >= 2 {
				repo.EXPECT().CharacterTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.CharacterRow]{}, fail(2))
			}

			// when
			_, err := svc.Player(context.Background(), "g", "u", rng)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestService_DataDays_LooksBackTheWindowThroughToday(t *testing.T) {
	// given
	svc, repo, _ := newService(t)
	to := day(2026, 9, 27)
	repo.EXPECT().ReservationDays(mock.Anything, "g", to.AddDate(0, 0, -DataDaysWindow), to).Return([]time.Time{day(2026, 9, 1)}, nil)

	// when
	days, err := svc.DataDays(context.Background(), "g", now)

	// then
	require.NoError(t, err)
	assert.Equal(t, []time.Time{day(2026, 9, 1)}, days)
}

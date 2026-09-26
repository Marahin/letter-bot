package characters

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/guildsworld"
	"spot-assistant/internal/core/dto/stats"
	corestats "spot-assistant/internal/core/stats"
	"spot-assistant/internal/ports"
)

var (
	now     = time.Date(2026, 9, 26, 14, 0, 0, 0, time.Local)
	rng     = corestats.ResolveRange(nil, now)
	errBoom = errors.New("boom")
)

type fixture struct {
	svc   *Service
	chars *mocks.MockCharacterAPI
	world *mocks.MockWorldNameRepository
	exp   *mocks.MockExperienceRepository
	stats *mocks.MockStatsRepository
}

func newFixture(t *testing.T) fixture {
	f := fixture{
		chars: mocks.NewMockCharacterAPI(t),
		world: mocks.NewMockWorldNameRepository(t),
		exp:   mocks.NewMockExperienceRepository(t),
		stats: mocks.NewMockStatsRepository(t),
	}
	f.svc = New(f.chars, f.world, f.exp, f.stats, nil)
	return f
}

func filter() stats.Filter {
	return stats.Filter{GuildID: "g", From: rng.From, To: rng.To, CharacterKey: "quiet nyx"}
}

func spotsQuery() stats.Query {
	return stats.Query{Filter: filter(), Sort: stats.Sort{Key: stats.SortHours}, Limit: corestats.BreakdownSize}
}

func (f fixture) expectStats(daily []stats.Day, spots stats.Page[stats.SpotRow]) {
	f.stats.EXPECT().Daily(mock.Anything, filter()).Return(daily, nil)
	f.stats.EXPECT().SpotTotals(mock.Anything, spotsQuery()).Return(spots, nil)
	f.stats.EXPECT().CharacterReservations(mock.Anything, filter(), RecentLimit).Return([]stats.CharacterReservation{{ID: 1}}, nil)
}

func TestProfile_CombinesTibiaDataHistoryAndStats(t *testing.T) {
	// given
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, "quiet NYX").Return(&character.Character{Name: "Quiet Nyx", Level: 500}, nil)
	f.world.EXPECT().SelectGuildWorld(mock.Anything, "g").Return(&guildsworld.GuildsWorld{WorldName: "Celesta"}, nil)
	f.exp.EXPECT().SnapshotHistory(mock.Anything, "Celesta", "quiet nyx", rng.From, rng.To).Return([]experience.Snapshot{
		{Level: 499, Experience: 100, ObservedAt: now.Add(-48 * time.Hour)},
		{Level: 500, Experience: 200, ObservedAt: now.Add(-2 * time.Hour)},
		{Level: 500, Experience: 250, ObservedAt: now.Add(-1 * time.Hour)},
	}, nil)
	spots := stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{{Name: "b"}, {Name: "a"}}, Total: 2}
	f.expectStats([]stats.Day{
		{Day: corestats.Midnight(now), Totals: stats.Totals{Reservations: 1, Seconds: 3600}},
		{Day: corestats.Midnight(now).AddDate(0, 0, -1), Totals: stats.Totals{Reservations: 2, Seconds: 7200, ExpReservations: 1, ExpSeconds: 3600, Exp: 9}},
	}, spots)

	// when
	p, err := f.svc.Profile(context.Background(), "g", "  quiet NYX ", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, stats.ProfileFound, p.Source)
	assert.Equal(t, "Quiet Nyx", p.Name)
	assert.Equal(t, "quiet nyx", p.Key)
	assert.Equal(t, "Celesta", p.World)
	require.Len(t, p.History, 2)
	assert.Equal(t, int64(250), p.History[1].Experience)
	assert.Equal(t, spots, p.Spots)
	assert.Equal(t, int64(3), p.Totals.Reservations)
	assert.Len(t, p.Daily, corestats.DefaultDays)
	assert.Len(t, p.Recent, 1)
}

func TestProfile_TibiaDataFailureDegradesToStats(t *testing.T) {
	// given
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, "Quiet Nyx").Return(nil, ports.ErrUpstreamUnavailable)
	f.world.EXPECT().SelectGuildWorld(mock.Anything, "g").Return(nil, ports.ErrNotFound)
	f.expectStats(nil, stats.Page[stats.SpotRow]{})

	// when
	p, err := f.svc.Profile(context.Background(), "g", "Quiet Nyx", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, stats.ProfileUnavailable, p.Source)
	assert.Nil(t, p.Character)
	assert.Empty(t, p.World)
	assert.Empty(t, p.History)
}

func TestProfile_UnknownEverywhereIsNotFound(t *testing.T) {
	// given
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, "Quiet Nyx").Return(nil, ports.ErrCharacterNotFound)
	f.world.EXPECT().SelectGuildWorld(mock.Anything, "g").Return(nil, ports.ErrNotFound)
	f.expectStats(nil, stats.Page[stats.SpotRow]{})

	// when
	_, err := f.svc.Profile(context.Background(), "g", "Quiet Nyx", rng)

	// then
	assert.ErrorIs(t, err, ports.ErrNotFound)
}

func TestProfile_DeletedCharacterWithReservationsStillShows(t *testing.T) {
	// given
	f := newFixture(t)
	f.chars.EXPECT().GetCharacter(mock.Anything, "Quiet Nyx").Return(nil, ports.ErrCharacterNotFound)
	f.world.EXPECT().SelectGuildWorld(mock.Anything, "g").Return(nil, ports.ErrNotFound)
	f.expectStats([]stats.Day{{Day: corestats.Midnight(now), Totals: stats.Totals{Reservations: 1}}}, stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{{Name: "a"}}, Total: 1})

	// when
	p, err := f.svc.Profile(context.Background(), "g", "Quiet Nyx", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, stats.ProfileNotFound, p.Source)
}

func TestProfile_BlankNameIsNotFound(t *testing.T) {
	// given
	f := newFixture(t)

	// when
	_, err := f.svc.Profile(context.Background(), "g", "   ", rng)

	// then
	assert.ErrorIs(t, err, ports.ErrNotFound)
}

func TestProfile_RepositoryErrors(t *testing.T) {
	steps := []string{"world", "history", "daily", "spots", "recent"}
	for i, step := range steps {
		t.Run(step, func(t *testing.T) {
			// given
			f := newFixture(t)
			fail := func(n int) error {
				if n == i {
					return errBoom
				}
				return nil
			}
			f.chars.EXPECT().GetCharacter(mock.Anything, "Quiet Nyx").Return(&character.Character{}, nil)
			if i == 0 {
				f.world.EXPECT().SelectGuildWorld(mock.Anything, "g").Return(nil, errBoom)
			} else {
				f.world.EXPECT().SelectGuildWorld(mock.Anything, "g").Return(&guildsworld.GuildsWorld{WorldName: "Celesta"}, nil)
				f.exp.EXPECT().SnapshotHistory(mock.Anything, "Celesta", "quiet nyx", rng.From, rng.To).Return(nil, fail(1))
			}
			if i >= 2 {
				f.stats.EXPECT().Daily(mock.Anything, filter()).Return(nil, fail(2))
			}
			if i >= 3 {
				f.stats.EXPECT().SpotTotals(mock.Anything, spotsQuery()).Return(stats.Page[stats.SpotRow]{}, fail(3))
			}
			if i >= 4 {
				f.stats.EXPECT().CharacterReservations(mock.Anything, filter(), RecentLimit).Return(nil, fail(4))
			}

			// when
			_, err := f.svc.Profile(context.Background(), "g", "Quiet Nyx", rng)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestDailyHistory_KeepsTheLastSnapshotOfEachDay(t *testing.T) {
	// given
	d1 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.Local)
	snaps := []experience.Snapshot{
		{Level: 1, Experience: 10, ObservedAt: d1},
		{Level: 2, Experience: 20, ObservedAt: d1.Add(5 * time.Hour)},
		{Level: 3, Experience: 30, ObservedAt: d1.Add(26 * time.Hour)},
	}

	// when
	got := DailyHistory(snaps)

	// then
	require.Len(t, got, 2)
	assert.Equal(t, stats.HistoryPoint{Day: corestats.Midnight(d1), Level: 2, Experience: 20}, got[0])
	assert.Equal(t, int64(30), got[1].Experience)
	assert.Empty(t, DailyHistory(nil))
}

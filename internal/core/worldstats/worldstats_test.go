package worldstats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

var (
	errBoom = errors.New("boom")
	rng     = stats.Range{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
)

func newService(t *testing.T) (*Service, *mocks.MockWorldStatsRepository) {
	repo := mocks.NewMockWorldStatsRepository(t)
	return New(repo), repo
}

func onWorld(key string) stats.WorldFilter {
	return stats.WorldFilter{From: rng.From, To: rng.To, WorldKey: key}
}

func TestKey(t *testing.T) {
	assert.Equal(t, "hero cave", Key("  Hero Cave "))
	assert.Equal(t, "", Key("   "))
}

func TestService_PublicWorldsAndWorlds(t *testing.T) {
	// given
	svc, repo := newService(t)
	worlds := []stats.World{{Key: "celesta", Name: "Celesta"}}
	rows := []stats.WorldRow{{Key: "celesta", Name: "Celesta", Guilds: 2}}
	repo.EXPECT().PublicWorlds(mock.Anything).Return(worlds, nil)
	repo.EXPECT().WorldTotals(mock.Anything, stats.WorldFilter{From: rng.From, To: rng.To}).Return(rows, nil)

	// when
	gotWorlds, err1 := svc.PublicWorlds(context.Background())
	gotRows, err2 := svc.Worlds(context.Background(), rng)

	// then
	require.NoError(t, errors.Join(err1, err2))
	assert.Equal(t, worlds, gotWorlds)
	assert.Equal(t, rows, gotRows)
}

func TestService_World(t *testing.T) {
	// given
	svc, repo := newService(t)
	f := onWorld("celesta")
	totals := stats.Totals{Reservations: 4}
	guilds := []stats.WorldGuildRow{{GuildID: "g", Name: "Guild"}}
	byExp := []stats.SpawnRow{{Key: "hero cave"}}
	byProfit := []stats.SpawnRow{{Key: "banuta"}}
	boards := stats.WorldCharacterBoards{
		ByExp:    []stats.WorldCharacterRow{{GuildID: "g", CharacterRow: stats.CharacterRow{Key: "a"}}},
		ByProfit: []stats.WorldCharacterRow{{GuildID: "g", CharacterRow: stats.CharacterRow{Key: "b"}}},
	}
	repo.EXPECT().WorldTotals(mock.Anything, f).Return([]stats.WorldRow{{Key: "celesta", Name: "Celesta", Totals: totals}}, nil)
	repo.EXPECT().WorldGuildTotals(mock.Anything, f).Return(guilds, nil)
	repo.EXPECT().SpawnTotals(mock.Anything, stats.WorldQuery{WorldFilter: f, Sort: stats.Sort{Key: stats.SortExpPerHour}, Limit: BoardSize, MinExpSeconds: 3 * 3600}).
		Return(stats.Page[stats.SpawnRow]{Rows: byExp, Total: 1}, nil)
	repo.EXPECT().SpawnTotals(mock.Anything, stats.WorldQuery{WorldFilter: f, Sort: stats.Sort{Key: stats.SortProfitPerHour}, Limit: BoardSize, MinLootSeconds: 3 * 3600}).
		Return(stats.Page[stats.SpawnRow]{Rows: byProfit, Total: 1}, nil)
	repo.EXPECT().WorldCharacterBoards(mock.Anything, f, BoardSize).Return(boards, nil)

	// when
	d, err := svc.World(context.Background(), " Celesta", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, &stats.WorldDetail{
		Key: "celesta", Name: "Celesta", Range: rng, Totals: totals, Guilds: guilds,
		SpawnsByExpPerHour: byExp, SpawnsByProfitPerHour: byProfit,
		CharactersByExp: boards.ByExp, CharactersByProfit: boards.ByProfit,
	}, d)
}

func TestService_World_NotFound(t *testing.T) {
	t.Run("blank name", func(t *testing.T) {
		// given
		svc, _ := newService(t)

		// when
		_, err := svc.World(context.Background(), "  ", rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	t.Run("no public guild has it", func(t *testing.T) {
		// given
		svc, repo := newService(t)
		repo.EXPECT().WorldTotals(mock.Anything, onWorld("nowhere")).Return(nil, nil)

		// when
		_, err := svc.World(context.Background(), "Nowhere", rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
}

func TestService_World_Errors(t *testing.T) {
	for i, step := range []string{"totals", "guilds", "exp spawns", "profit spawns", "characters"} {
		t.Run(step, func(t *testing.T) {
			// given
			svc, repo := newService(t)
			fail := func(n int) error {
				if n == i {
					return errBoom
				}
				return nil
			}
			repo.EXPECT().WorldTotals(mock.Anything, mock.Anything).Return([]stats.WorldRow{{Key: "celesta"}}, fail(0))
			if i >= 1 {
				repo.EXPECT().WorldGuildTotals(mock.Anything, mock.Anything).Return(nil, fail(1))
			}
			if i >= 2 {
				repo.EXPECT().SpawnTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.SpawnRow]{}, fail(2)).Once()
			}
			if i >= 3 {
				repo.EXPECT().SpawnTotals(mock.Anything, mock.Anything).Return(stats.Page[stats.SpawnRow]{}, fail(3)).Once()
			}
			if i >= 4 {
				repo.EXPECT().WorldCharacterBoards(mock.Anything, mock.Anything, mock.Anything).Return(stats.WorldCharacterBoards{}, fail(4))
			}

			// when
			_, err := svc.World(context.Background(), "Celesta", rng)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestService_Spawns_PassesWorldSortAndLimit(t *testing.T) {
	// given
	svc, repo := newService(t)
	byName := stats.Sort{Key: stats.SortName, Asc: true}
	page := stats.Page[stats.SpawnRow]{Rows: []stats.SpawnRow{{Key: "a"}}, Total: 3}
	repo.EXPECT().SpawnTotals(mock.Anything, stats.WorldQuery{WorldFilter: onWorld("antica"), Sort: byName, Limit: 500}).Return(page, nil)

	// when
	got, err := svc.Spawns(context.Background(), rng, " Antica ", byName, 500)

	// then
	require.NoError(t, err)
	assert.Equal(t, page, got)
}

func TestService_Spawn(t *testing.T) {
	// given
	svc, repo := newService(t)
	f := stats.WorldFilter{From: rng.From, To: rng.To, SpawnKey: "hero cave"}
	spots := []stats.SpawnSpotRow{
		{WorldKey: "antica", World: "Antica", GuildID: "a", SpotID: 1, Totals: stats.Totals{Reservations: 1, Seconds: 3600, LootReservations: 1, Profit: 10}},
		{WorldKey: "celesta", World: "Celesta", GuildID: "c1", SpotID: 2, Totals: stats.Totals{Reservations: 2, Seconds: 7200}},
		{WorldKey: "celesta", World: "Celesta", GuildID: "c2", SpotID: 3, Totals: stats.Totals{Reservations: 1, Seconds: 3600, Exp: 5, ExpReservations: 1}},
		{WorldKey: "celesta", World: "Celesta", GuildID: "c2", SpotID: 4, Totals: stats.Totals{Reservations: 1, Seconds: 60}},
	}
	repo.EXPECT().SpawnName(mock.Anything, "hero cave").Return("Hero Cave", nil)
	repo.EXPECT().SpawnSpotTotals(mock.Anything, f).Return(spots, nil)

	// when
	d, err := svc.Spawn(context.Background(), "Hero Cave", rng)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Hero Cave", d.Name)
	assert.Equal(t, "hero cave", d.Key)
	assert.Equal(t, stats.Totals{Reservations: 5, Seconds: 3*3600 + 3600 + 60, ExpReservations: 1, Exp: 5, LootReservations: 1, Profit: 10}, d.Totals)
	assert.Equal(t, []stats.WorldRow{
		{Key: "celesta", Name: "Celesta", Guilds: 2, Totals: stats.Totals{Reservations: 4, Seconds: 3*3600 + 60, ExpReservations: 1, Exp: 5}},
		{Key: "antica", Name: "Antica", Guilds: 1, Totals: stats.Totals{Reservations: 1, Seconds: 3600, LootReservations: 1, Profit: 10}},
	}, d.Worlds)
	assert.Equal(t, spots, d.Spots)
}

func TestService_Spawn_NotFoundAndErrors(t *testing.T) {
	t.Run("blank key", func(t *testing.T) {
		// given
		svc, _ := newService(t)

		// when
		_, err := svc.Spawn(context.Background(), " ", rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	t.Run("unknown spawn", func(t *testing.T) {
		// given
		svc, repo := newService(t)
		repo.EXPECT().SpawnName(mock.Anything, "nope").Return("", ports.ErrNotFound)

		// when
		_, err := svc.Spawn(context.Background(), "nope", rng)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	t.Run("spots", func(t *testing.T) {
		// given
		svc, repo := newService(t)
		repo.EXPECT().SpawnName(mock.Anything, "a").Return("A", nil)
		repo.EXPECT().SpawnSpotTotals(mock.Anything, mock.Anything).Return(nil, errBoom)

		// when
		_, err := svc.Spawn(context.Background(), "a", rng)

		// then
		assert.ErrorIs(t, err, errBoom)
	})
}

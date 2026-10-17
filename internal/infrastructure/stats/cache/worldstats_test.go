package cache

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
	day     = time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	filter  = stats.WorldFilter{From: day, To: day.AddDate(0, 0, 1), WorldKey: "celesta"}
	errBoom = errors.New("boom")
)

func newCache(t *testing.T) (*WorldStats, *mocks.MockWorldStatsRepository, *time.Time) {
	inner := mocks.NewMockWorldStatsRepository(t)
	c := NewWorldStats(inner)
	clock := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return clock }
	return c, inner, &clock
}

func TestWorldStats_ServesAHitUntilTheTTLEnds(t *testing.T) {
	// given
	c, inner, clock := newCache(t)
	ctx := context.Background()
	inner.EXPECT().WorldTotals(mock.Anything, filter).Return([]stats.WorldRow{{Key: "celesta", Guilds: 1}}, nil).Once()

	// when
	first, err1 := c.WorldTotals(ctx, filter)
	second, err2 := c.WorldTotals(ctx, filter)

	// then
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, first, second)

	// given
	*clock = clock.Add(TTL)
	inner.EXPECT().WorldTotals(mock.Anything, filter).Return([]stats.WorldRow{{Key: "celesta", Guilds: 2}}, nil).Once()

	// when
	third, err := c.WorldTotals(ctx, filter)

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, third[0].Guilds)
}

func TestWorldStats_KeysByMethodAndArguments(t *testing.T) {
	// given
	c, inner, _ := newCache(t)
	ctx := context.Background()
	other := filter
	other.WorldKey = "antica"
	query := stats.WorldQuery{WorldFilter: filter, Sort: stats.Sort{Key: stats.SortExpPerHour}, Limit: 10, MinExpSeconds: 10800}
	descending, asc, limit, minLoot := query, query, query, query
	asc.Sort.Asc = true
	limit.Limit = 20
	minLoot.MinExpSeconds, minLoot.MinLootSeconds = 0, 10800
	descending.Sort.Key = stats.SortProfitPerHour
	inner.EXPECT().WorldTotals(mock.Anything, filter).Return(nil, nil).Once()
	inner.EXPECT().WorldTotals(mock.Anything, other).Return(nil, nil).Once()
	inner.EXPECT().WorldGuildTotals(mock.Anything, filter).Return(nil, nil).Once()
	inner.EXPECT().SpawnSpotTotals(mock.Anything, filter).Return(nil, nil).Once()
	inner.EXPECT().PublicWorlds(mock.Anything).Return([]stats.World{{Key: "celesta"}}, nil).Once()
	inner.EXPECT().SpawnName(mock.Anything, "dragons").Return("Dragons", nil).Once()
	inner.EXPECT().SpawnName(mock.Anything, "hero cave").Return("Hero Cave", nil).Once()
	inner.EXPECT().WorldCharacterBoards(mock.Anything, filter, 10).Return(stats.WorldCharacterBoards{}, nil).Once()
	inner.EXPECT().WorldCharacterBoards(mock.Anything, filter, 5).Return(stats.WorldCharacterBoards{}, nil).Once()
	for _, q := range []stats.WorldQuery{query, descending, asc, limit, minLoot} {
		inner.EXPECT().SpawnTotals(mock.Anything, q).Return(stats.Page[stats.SpawnRow]{Total: q.Limit}, nil).Once()
	}

	// when
	for range 2 {
		_, _ = c.WorldTotals(ctx, filter)
		_, _ = c.WorldTotals(ctx, other)
		_, _ = c.WorldGuildTotals(ctx, filter)
		_, _ = c.SpawnSpotTotals(ctx, filter)
		_, _ = c.PublicWorlds(ctx)
		_, _ = c.SpawnName(ctx, "dragons")
		_, _ = c.SpawnName(ctx, "hero cave")
		_, _ = c.WorldCharacterBoards(ctx, filter, 10)
		_, _ = c.WorldCharacterBoards(ctx, filter, 5)
		for _, q := range []stats.WorldQuery{query, descending, asc, limit, minLoot} {
			_, _ = c.SpawnTotals(ctx, q)
		}
	}
	name, err := c.SpawnName(ctx, "hero cave")

	// then
	require.NoError(t, err)
	assert.Equal(t, "Hero Cave", name)
}

func TestWorldStats_DoesNotKeepErrors(t *testing.T) {
	// given
	c, inner, _ := newCache(t)
	ctx := context.Background()
	inner.EXPECT().SpawnName(mock.Anything, "nope").Return("", ports.ErrNotFound).Once()
	inner.EXPECT().SpawnName(mock.Anything, "nope").Return("Nope", nil).Once()
	inner.EXPECT().PublicWorlds(mock.Anything).Return(nil, errBoom).Once()
	inner.EXPECT().PublicWorlds(mock.Anything).Return([]stats.World{{Key: "celesta"}}, nil).Once()

	// when
	_, errFirst := c.SpawnName(ctx, "nope")
	name, errSecond := c.SpawnName(ctx, "nope")
	_, errWorlds := c.PublicWorlds(ctx)
	worlds, errWorldsAgain := c.PublicWorlds(ctx)

	// then
	assert.ErrorIs(t, errFirst, ports.ErrNotFound)
	require.NoError(t, errSecond)
	assert.Equal(t, "Nope", name)
	assert.ErrorIs(t, errWorlds, errBoom)
	require.NoError(t, errWorldsAgain)
	assert.Len(t, worlds, 1)
}

func TestWorldStats_EvictsTheFirstToExpireWhenFull(t *testing.T) {
	// given
	c, inner, clock := newCache(t)
	ctx := context.Background()
	c.maxEntries = 2
	start := *clock
	inner.EXPECT().SpawnName(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, k string) (string, error) {
		return k, nil
	})
	_, _ = c.SpawnName(ctx, "a")
	*clock = start.Add(10 * time.Second)
	_, _ = c.SpawnName(ctx, "b")

	// when
	*clock = start.Add(20 * time.Second)
	_, _ = c.SpawnName(ctx, "c")

	// then
	assert.NotContains(t, c.entries, key("SpawnName", "a"))
	assert.Contains(t, c.entries, key("SpawnName", "b"))
	assert.Contains(t, c.entries, key("SpawnName", "c"))
}

func TestWorldStats_DropsEveryExpiredEntryOnAStore(t *testing.T) {
	// given
	c, inner, clock := newCache(t)
	ctx := context.Background()
	c.maxEntries = 3
	start := *clock
	inner.EXPECT().SpawnName(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, k string) (string, error) {
		return k, nil
	})
	for i, k := range []string{"a", "b", "c"} {
		*clock = start.Add(time.Duration(i) * 10 * time.Second)
		_, _ = c.SpawnName(ctx, k)
	}

	// when
	*clock = start.Add(15*time.Second + TTL)
	_, _ = c.SpawnName(ctx, "d")

	// then
	assert.Len(t, c.entries, 2)
	assert.Contains(t, c.entries, key("SpawnName", "c"))
	assert.Contains(t, c.entries, key("SpawnName", "d"))
}

func TestWorldStats_ImplementsThePort(t *testing.T) {
	// then
	var _ ports.WorldStatsRepository = NewWorldStats(nil)
}

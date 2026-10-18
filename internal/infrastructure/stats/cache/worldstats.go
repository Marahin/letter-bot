// Package cache keeps the answers of the cross-server stats queries in memory: the public world
// and spawn pages read every public server, and anyone can open them.
package cache

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

const (
	// TTL is how stale a public page may be.
	TTL = time.Minute
	// MaxEntries bounds the memory that many distinct ranges, worlds and sorts can take.
	MaxEntries = 512
)

type entry struct {
	value   any
	expires time.Time
}

// WorldStats is a ports.WorldStatsRepository that keeps each answer for TTL, keyed by the method
// and its arguments. Errors are not kept. When full, it drops the expired entries, then the one
// that expires first. The cached slices are shared: callers must not change them.
type WorldStats struct {
	inner      ports.WorldStatsRepository
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]entry
}

func NewWorldStats(inner ports.WorldStatsRepository) *WorldStats {
	return &WorldStats{
		inner:      inner,
		ttl:        TTL,
		maxEntries: MaxEntries,
		now:        time.Now,
		entries:    map[string]entry{},
	}
}

func (c *WorldStats) PublicWorlds(ctx context.Context) ([]stats.World, error) {
	return cached(c, "PublicWorlds", func() ([]stats.World, error) { return c.inner.PublicWorlds(ctx) })
}

func (c *WorldStats) WorldTotals(ctx context.Context, f stats.WorldFilter) ([]stats.WorldRow, error) {
	return cached(c, key("WorldTotals", filterKey(f)), func() ([]stats.WorldRow, error) { return c.inner.WorldTotals(ctx, f) })
}

func (c *WorldStats) WorldGuildTotals(ctx context.Context, f stats.WorldFilter) ([]stats.WorldGuildRow, error) {
	return cached(c, key("WorldGuildTotals", filterKey(f)), func() ([]stats.WorldGuildRow, error) { return c.inner.WorldGuildTotals(ctx, f) })
}

func (c *WorldStats) SpawnTotals(ctx context.Context, q stats.WorldQuery) (stats.Page[stats.SpawnRow], error) {
	k := key("SpawnTotals", filterKey(q.WorldFilter), string(q.Sort.Key), strconv.FormatBool(q.Sort.Asc),
		strconv.Itoa(q.Limit), strconv.FormatInt(q.MinExpSeconds, 10), strconv.FormatInt(q.MinLootSeconds, 10))
	return cached(c, k, func() (stats.Page[stats.SpawnRow], error) { return c.inner.SpawnTotals(ctx, q) })
}

func (c *WorldStats) SpawnSpotTotals(ctx context.Context, f stats.WorldFilter) ([]stats.SpawnSpotRow, error) {
	return cached(c, key("SpawnSpotTotals", filterKey(f)), func() ([]stats.SpawnSpotRow, error) { return c.inner.SpawnSpotTotals(ctx, f) })
}

func (c *WorldStats) SpawnName(ctx context.Context, spawnKey string) (string, error) {
	return cached(c, key("SpawnName", spawnKey), func() (string, error) { return c.inner.SpawnName(ctx, spawnKey) })
}

func (c *WorldStats) WorldCharacterBoards(ctx context.Context, f stats.WorldFilter, n int) (stats.WorldCharacterBoards, error) {
	return cached(c, key("WorldCharacterBoards", filterKey(f), strconv.Itoa(n)), func() (stats.WorldCharacterBoards, error) {
		return c.inner.WorldCharacterBoards(ctx, f, n)
	})
}

func key(parts ...string) string {
	return strings.Join(parts, "\x00")
}

func filterKey(f stats.WorldFilter) string {
	return key(strconv.FormatInt(f.From.UnixNano(), 10), strconv.FormatInt(f.To.UnixNano(), 10), f.WorldKey, f.SpawnKey)
}

func cached[T any](c *WorldStats, k string, load func() (T, error)) (T, error) {
	now := c.now()
	c.mu.Lock()
	e, ok := c.entries[k]
	c.mu.Unlock()
	if v, isT := e.value.(T); ok && isT && now.Before(e.expires) {
		return v, nil
	}
	v, err := load()
	if err != nil {
		return v, err
	}
	c.store(k, entry{value: v, expires: now.Add(c.ttl)}, now)
	return v, nil
}

func (c *WorldStats) store(k string, e entry, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for old, oe := range c.entries {
		if !now.Before(oe.expires) {
			delete(c.entries, old)
		}
	}
	if _, ok := c.entries[k]; !ok && len(c.entries) >= c.maxEntries {
		c.evictFirstToExpire()
	}
	c.entries[k] = e
}

func (c *WorldStats) evictFirstToExpire() {
	var (
		first        string
		firstExpires time.Time
		found        bool
	)
	for k, e := range c.entries {
		if !found || e.expires.Before(firstExpires) {
			first, firstExpires, found = k, e.expires, true
		}
	}
	delete(c.entries, first)
}

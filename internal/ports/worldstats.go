package ports

import (
	"context"

	"spot-assistant/internal/core/dto/stats"
)

// WorldStatsRepository reads the stats of every public guild: premium, with the bot present and
// a world set.
type WorldStatsRepository interface {
	// PublicWorlds lists the worlds of those guilds by name.
	PublicWorlds(ctx context.Context) ([]stats.World, error)
	// WorldTotals has one row per world (f.WorldKey narrows it), with zero totals for a world
	// without reservations in the range.
	WorldTotals(ctx context.Context, f stats.WorldFilter) ([]stats.WorldRow, error)
	// WorldGuildTotals has one row per guild of the filtered world(s), every guild included.
	WorldGuildTotals(ctx context.Context, f stats.WorldFilter) ([]stats.WorldGuildRow, error)
	// SpawnTotals groups the reservations by respawn name.
	SpawnTotals(ctx context.Context, q stats.WorldQuery) (stats.Page[stats.SpawnRow], error)
	// SpawnSpotTotals groups the reservations of f.SpawnKey by world, guild and respawn.
	SpawnSpotTotals(ctx context.Context, f stats.WorldFilter) ([]stats.SpawnSpotRow, error)
	// SpawnName returns the most common spelling of the spawn, or ErrNotFound.
	SpawnName(ctx context.Context, key string) (string, error)
	// WorldCharacterBoards returns the n characters with the most experience and the n with the
	// most profit (equal shares).
	WorldCharacterBoards(ctx context.Context, f stats.WorldFilter, n int) (stats.WorldCharacterBoards, error)
}

type WorldStatsService interface {
	PublicWorlds(ctx context.Context) ([]stats.World, error)
	Worlds(ctx context.Context, rng stats.Range) ([]stats.WorldRow, error)
	// World returns ErrNotFound for a world no public guild has set.
	World(ctx context.Context, name string, rng stats.Range) (*stats.WorldDetail, error)
	// Spawns returns the first limit spawns in sort order; world "" means every world.
	Spawns(ctx context.Context, rng stats.Range, world string, sort stats.Sort, limit int) (stats.Page[stats.SpawnRow], error)
	// Spawn returns ErrNotFound for a respawn name no public guild has.
	Spawn(ctx context.Context, key string, rng stats.Range) (*stats.SpawnDetail, error)
}

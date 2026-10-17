// Package worldstats builds the public cross-guild stats: the worlds and the spawn directory.
// They read only guilds with premium, the bot present and a world set.
package worldstats

import (
	"context"
	"sort"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/stats"
	corestats "spot-assistant/internal/core/stats"
	"spot-assistant/internal/ports"
)

// BoardSize is the number of rows in each world page leaderboard.
const BoardSize = 10

type Service struct {
	repo ports.WorldStatsRepository
}

func New(repo ports.WorldStatsRepository) *Service {
	return &Service{repo: repo}
}

// Key normalizes a world or a respawn name as the SQL does (lower(btrim(name))), like a character name.
func Key(name string) string {
	return experience.CharacterKey(name)
}

func filter(rng stats.Range) stats.WorldFilter {
	return stats.WorldFilter{From: rng.From, To: rng.To}
}

func (s *Service) PublicWorlds(ctx context.Context) ([]stats.World, error) {
	return s.repo.PublicWorlds(ctx)
}

func (s *Service) Worlds(ctx context.Context, rng stats.Range) ([]stats.WorldRow, error) {
	return s.repo.WorldTotals(ctx, filter(rng))
}

func (s *Service) World(ctx context.Context, name string, rng stats.Range) (*stats.WorldDetail, error) {
	f := filter(rng)
	f.WorldKey = Key(name)
	if f.WorldKey == "" {
		return nil, ports.ErrNotFound
	}
	worlds, err := s.repo.WorldTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	if len(worlds) == 0 {
		return nil, ports.ErrNotFound
	}
	guilds, err := s.repo.WorldGuildTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	byExp, err := s.repo.SpawnTotals(ctx, stats.WorldQuery{
		WorldFilter: f, Sort: stats.Sort{Key: stats.SortExpPerHour}, Limit: BoardSize,
		MinExpSeconds: int64(corestats.MinExpPerHourHours * 3600),
	})
	if err != nil {
		return nil, err
	}
	byProfit, err := s.repo.SpawnTotals(ctx, stats.WorldQuery{
		WorldFilter: f, Sort: stats.Sort{Key: stats.SortProfitPerHour}, Limit: BoardSize,
		MinLootSeconds: int64(corestats.MinProfitPerHourHours * 3600),
	})
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.WorldCharacterBoards(ctx, f, BoardSize)
	if err != nil {
		return nil, err
	}
	return &stats.WorldDetail{
		Key:                   worlds[0].Key,
		Name:                  worlds[0].Name,
		Range:                 rng,
		Totals:                worlds[0].Totals,
		Guilds:                guilds,
		SpawnsByExpPerHour:    byExp.Rows,
		SpawnsByProfitPerHour: byProfit.Rows,
		CharactersByExp:       characters.ByExp,
		CharactersByProfit:    characters.ByProfit,
	}, nil
}

func (s *Service) Spawns(ctx context.Context, rng stats.Range, world string, order stats.Sort, limit int) (stats.Page[stats.SpawnRow], error) {
	f := filter(rng)
	f.WorldKey = Key(world)
	return s.repo.SpawnTotals(ctx, stats.WorldQuery{WorldFilter: f, Sort: order, Limit: limit})
}

func (s *Service) Spawn(ctx context.Context, key string, rng stats.Range) (*stats.SpawnDetail, error) {
	f := filter(rng)
	f.SpawnKey = Key(key)
	if f.SpawnKey == "" {
		return nil, ports.ErrNotFound
	}
	name, err := s.repo.SpawnName(ctx, f.SpawnKey)
	if err != nil {
		return nil, err
	}
	spots, err := s.repo.SpawnSpotTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	d := &stats.SpawnDetail{Key: f.SpawnKey, Name: name, Range: rng, Spots: spots, Worlds: byWorld(spots)}
	for _, sp := range spots {
		d.Totals = d.Totals.Add(sp.Totals)
	}
	return d, nil
}

// byWorld sums the respawn rows per world, the most booked first.
func byWorld(spots []stats.SpawnSpotRow) []stats.WorldRow {
	index := map[string]int{}
	guilds := map[string]map[string]bool{}
	var out []stats.WorldRow
	for _, sp := range spots {
		i, ok := index[sp.WorldKey]
		if !ok {
			i = len(out)
			index[sp.WorldKey] = i
			guilds[sp.WorldKey] = map[string]bool{}
			out = append(out, stats.WorldRow{Key: sp.WorldKey, Name: sp.World})
		}
		out[i].Totals = out[i].Totals.Add(sp.Totals)
		if !guilds[sp.WorldKey][sp.GuildID] {
			guilds[sp.WorldKey][sp.GuildID] = true
			out[i].Guilds++
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		return out[i].Key < out[j].Key
	})
	return out
}

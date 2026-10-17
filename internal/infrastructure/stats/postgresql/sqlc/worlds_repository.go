package sqlc

import (
	"context"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/db/postgresql"
)

// WorldStatsRepository implements ports.WorldStatsRepository.
type WorldStatsRepository struct {
	q *Queries
}

func NewWorldStatsRepository(db DBTX) *WorldStatsRepository {
	return &WorldStatsRepository{q: New(db)}
}

func (r *WorldStatsRepository) PublicWorlds(ctx context.Context) ([]stats.World, error) {
	rows, err := r.q.PublicWorlds(ctx)
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	out := make([]stats.World, len(rows))
	for i, row := range rows {
		out[i] = stats.World{Key: row.WorldKey, Name: row.Name}
	}
	return out, nil
}

func (r *WorldStatsRepository) WorldTotals(ctx context.Context, f stats.WorldFilter) ([]stats.WorldRow, error) {
	rows, err := r.q.WorldTotals(ctx, WorldTotalsParams{WorldKey: f.WorldKey, FromT: ts(f.From), ToT: ts(f.To), SpawnKey: f.SpawnKey})
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	out := make([]stats.WorldRow, len(rows))
	for i, row := range rows {
		out[i] = stats.WorldRow{
			Key:    row.WorldKey,
			Name:   row.Name,
			Guilds: int(row.Guilds),
			Totals: stats.Totals{
				Reservations: row.Reservations, Seconds: row.Seconds,
				ExpReservations: row.ExpReservations, ExpSeconds: row.ExpSeconds, Exp: row.Exp,
				LootReservations: row.LootReservations, LootSeconds: row.LootSeconds,
				Profit: row.Profit, Loot: row.Loot, Supplies: row.Supplies,
			},
		}
	}
	return out, nil
}

func (r *WorldStatsRepository) WorldGuildTotals(ctx context.Context, f stats.WorldFilter) ([]stats.WorldGuildRow, error) {
	rows, err := r.q.WorldGuildTotals(ctx, WorldGuildTotalsParams{WorldKey: f.WorldKey, FromT: ts(f.From), ToT: ts(f.To), SpawnKey: f.SpawnKey})
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	out := make([]stats.WorldGuildRow, len(rows))
	for i, row := range rows {
		out[i] = stats.WorldGuildRow{
			GuildID: row.GuildID,
			Name:    row.Name,
			Icon:    row.Icon,
			Totals: stats.Totals{
				Reservations: row.Reservations, Seconds: row.Seconds,
				ExpReservations: row.ExpReservations, ExpSeconds: row.ExpSeconds, Exp: row.Exp,
				LootReservations: row.LootReservations, LootSeconds: row.LootSeconds,
				Profit: row.Profit, Loot: row.Loot, Supplies: row.Supplies,
			},
		}
	}
	return out, nil
}

func (r *WorldStatsRepository) SpawnTotals(ctx context.Context, q stats.WorldQuery) (stats.Page[stats.SpawnRow], error) {
	f := q.WorldFilter
	rows, err := r.q.SpawnTotals(ctx, SpawnTotalsParams{
		SortKey: string(q.Sort.Key), Ascending: q.Sort.Asc, Lim: int32(q.Limit),
		WorldKey: f.WorldKey, FromT: ts(f.From), ToT: ts(f.To), SpawnKey: f.SpawnKey,
		MinExpSeconds: q.MinExpSeconds, MinLootSeconds: q.MinLootSeconds,
	})
	if err != nil {
		return stats.Page[stats.SpawnRow]{}, postgresql.MapError(err)
	}
	page := stats.Page[stats.SpawnRow]{Rows: make([]stats.SpawnRow, len(rows))}
	for i, row := range rows {
		page.Total = int(row.TotalRows)
		page.Rows[i] = stats.SpawnRow{
			Key:    row.SpawnKey,
			Name:   row.Name,
			Worlds: int(row.Worlds),
			Totals: stats.Totals{
				Reservations: row.Reservations, Seconds: row.Seconds,
				ExpReservations: row.ExpReservations, ExpSeconds: row.ExpSeconds, Exp: row.Exp,
				LootReservations: row.LootReservations, LootSeconds: row.LootSeconds,
				Profit: row.Profit, Loot: row.Loot, Supplies: row.Supplies,
			},
		}
	}
	return page, nil
}

func (r *WorldStatsRepository) SpawnSpotTotals(ctx context.Context, f stats.WorldFilter) ([]stats.SpawnSpotRow, error) {
	rows, err := r.q.SpawnSpotTotals(ctx, SpawnSpotTotalsParams{WorldKey: f.WorldKey, FromT: ts(f.From), ToT: ts(f.To), SpawnKey: f.SpawnKey})
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	out := make([]stats.SpawnSpotRow, len(rows))
	for i, row := range rows {
		out[i] = stats.SpawnSpotRow{
			WorldKey:  row.WorldKey,
			World:     row.WorldName,
			GuildID:   row.GuildID,
			GuildName: row.GuildName,
			SpotID:    row.SpotID,
			SpotName:  row.SpotName,
			Archived:  row.Archived,
			Totals: stats.Totals{
				Reservations: row.Reservations, Seconds: row.Seconds,
				ExpReservations: row.ExpReservations, ExpSeconds: row.ExpSeconds, Exp: row.Exp,
				LootReservations: row.LootReservations, LootSeconds: row.LootSeconds,
				Profit: row.Profit, Loot: row.Loot, Supplies: row.Supplies,
			},
		}
	}
	return out, nil
}

func (r *WorldStatsRepository) SpawnName(ctx context.Context, key string) (string, error) {
	name, err := r.q.SpawnName(ctx, key)
	return name, postgresql.MapError(err)
}

func (r *WorldStatsRepository) WorldCharacterBoards(ctx context.Context, f stats.WorldFilter, n int) (stats.WorldCharacterBoards, error) {
	rows, err := r.q.WorldCharacterBoards(ctx, WorldCharacterBoardsParams{
		WorldKey: f.WorldKey, FromT: ts(f.From), ToT: ts(f.To), SpawnKey: f.SpawnKey, Lim: int32(n),
	})
	if err != nil {
		return stats.WorldCharacterBoards{}, postgresql.MapError(err)
	}
	var boards stats.WorldCharacterBoards
	for _, row := range rows {
		c := stats.WorldCharacterRow{
			GuildID: row.GuildID,
			CharacterRow: stats.CharacterRow{
				Key:  row.CharacterKey,
				Name: row.Name,
				Totals: stats.Totals{
					Reservations: row.Reservations, Seconds: row.Seconds,
					ExpReservations: row.ExpReservations, ExpSeconds: row.ExpSeconds, Exp: row.Exp,
					LootReservations: row.LootReservations, LootSeconds: row.LootSeconds,
					Profit: row.Profit, Loot: row.Loot, Supplies: row.Supplies,
				},
			},
		}
		switch row.Board {
		case "exp":
			boards.ByExp = append(boards.ByExp, c)
		case "profit":
			boards.ByProfit = append(boards.ByProfit, c)
		}
	}
	return boards, nil
}

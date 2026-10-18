package sqlc

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

func newWorldsMock(t *testing.T) (pgxmock.PgxPoolIface, *WorldStatsRepository) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock, NewWorldStatsRepository(mock)
}

// figures are the totals columns of a row: 1..10, so a swapped mapping shows.
func figures() []any {
	return []any{int64(1), int64(2), int64(3), int64(4), int64(5), int64(6), int64(7), int64(8), int64(9), int64(10)}
}

var figuresTotals = stats.Totals{
	Reservations: 1, Seconds: 2, ExpReservations: 3, ExpSeconds: 4, Exp: 5,
	LootReservations: 6, LootSeconds: 7, Profit: 8, Loot: 9, Supplies: 10,
}

func row(head []any, tail ...any) []any {
	return append(append(head, figures()...), tail...)
}

func TestWorldStatsRepository_MapsRows(t *testing.T) {
	// given
	mock, repo := newWorldsMock(t)
	ctx := context.Background()
	f := stats.WorldFilter{WorldKey: "celesta", SpawnKey: "hero cave"}
	mock.ExpectQuery("name: PublicWorlds").WillReturnRows(pgxmock.NewRows([]string{"world_key", "name"}).AddRow("celesta", "Celesta"))
	mock.ExpectQuery("name: WorldTotals").WithArgs("celesta", pgxmock.AnyArg(), pgxmock.AnyArg(), "hero cave").
		WillReturnRows(pgxmock.NewRows(append([]string{"world_key", "name", "guilds"}, totalsCols...)).AddRow(row([]any{"celesta", "Celesta", int64(2)})...))
	mock.ExpectQuery("name: WorldGuildTotals").WithArgs("celesta", pgxmock.AnyArg(), pgxmock.AnyArg(), "hero cave").
		WillReturnRows(pgxmock.NewRows(append([]string{"guild_id", "name", "icon"}, totalsCols...)).AddRow(row([]any{"g", "Guild", "ic"})...))
	mock.ExpectQuery("name: SpawnTotals").WithArgs("profit_h", true, int32(5), "celesta", pgxmock.AnyArg(), pgxmock.AnyArg(), "hero cave", int64(60), int64(120)).
		WillReturnRows(pgxmock.NewRows(append(append([]string{"spawn_key", "name", "worlds"}, totalsCols...), "total_rows")).AddRow(row([]any{"hero cave", "Hero Cave", int64(2)}, int64(11))...))
	mock.ExpectQuery("name: SpawnSpotTotals").WithArgs("celesta", pgxmock.AnyArg(), pgxmock.AnyArg(), "hero cave").
		WillReturnRows(pgxmock.NewRows(append([]string{"world_key", "world_name", "guild_id", "guild_name", "spot_id", "spot_name", "archived"}, totalsCols...)).
			AddRow(row([]any{"celesta", "Celesta", "g", "Guild", int64(7), "Hero Cave", true})...))
	mock.ExpectQuery("name: SpawnName").WithArgs("hero cave").WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("Hero Cave"))
	mock.ExpectQuery("name: SpawnName").WithArgs("nope").WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("name: WorldCharacterBoards").WithArgs("celesta", pgxmock.AnyArg(), pgxmock.AnyArg(), "hero cave", int32(3)).
		WillReturnRows(pgxmock.NewRows(append([]string{"board", "character_key", "name", "guild_id"}, totalsCols...)).
			AddRow(row([]any{"exp", "a", "A", "g"})...).
			AddRow(row([]any{"profit", "b", "B", "h"})...))

	// when
	worlds, err1 := repo.PublicWorlds(ctx)
	totals, err2 := repo.WorldTotals(ctx, f)
	guilds, err3 := repo.WorldGuildTotals(ctx, f)
	spawns, err4 := repo.SpawnTotals(ctx, stats.WorldQuery{WorldFilter: f, Sort: stats.Sort{Key: stats.SortProfitPerHour, Asc: true}, Limit: 5, MinExpSeconds: 60, MinLootSeconds: 120})
	spots, err5 := repo.SpawnSpotTotals(ctx, f)
	name, err6 := repo.SpawnName(ctx, "hero cave")
	_, errMissing := repo.SpawnName(ctx, "nope")
	boards, err7 := repo.WorldCharacterBoards(ctx, f, 3)

	// then
	require.NoError(t, errors.Join(err1, err2, err3, err4, err5, err6, err7))
	assert.Equal(t, []stats.World{{Key: "celesta", Name: "Celesta"}}, worlds)
	assert.Equal(t, []stats.WorldRow{{Key: "celesta", Name: "Celesta", Guilds: 2, Totals: figuresTotals}}, totals)
	assert.Equal(t, []stats.WorldGuildRow{{GuildID: "g", Name: "Guild", Icon: "ic", Totals: figuresTotals}}, guilds)
	assert.Equal(t, stats.Page[stats.SpawnRow]{Rows: []stats.SpawnRow{{Key: "hero cave", Name: "Hero Cave", Worlds: 2, Totals: figuresTotals}}, Total: 11}, spawns)
	assert.Equal(t, []stats.SpawnSpotRow{{WorldKey: "celesta", World: "Celesta", GuildID: "g", GuildName: "Guild", SpotID: 7, SpotName: "Hero Cave", Archived: true, Totals: figuresTotals}}, spots)
	assert.Equal(t, "Hero Cave", name)
	assert.ErrorIs(t, errMissing, ports.ErrNotFound)
	assert.Equal(t, stats.WorldCharacterBoards{
		ByExp:    []stats.WorldCharacterRow{{GuildID: "g", CharacterRow: stats.CharacterRow{Key: "a", Name: "A", Totals: figuresTotals}}},
		ByProfit: []stats.WorldCharacterRow{{GuildID: "h", CharacterRow: stats.CharacterRow{Key: "b", Name: "B", Totals: figuresTotals}}},
	}, boards)
}

func TestWorldStatsRepository_MapsQueryErrors(t *testing.T) {
	for name, tt := range map[string]struct {
		err  error
		want error
	}{
		"passed through": {err: errBoom, want: errBoom},
		"mapped":         {err: pgx.ErrNoRows, want: ports.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			mock, repo := newWorldsMock(t)
			ctx := context.Background()
			mock.ExpectQuery("name: PublicWorlds").WillReturnError(tt.err)
			for _, q := range []struct {
				name string
				args int
			}{{"WorldTotals", 4}, {"WorldGuildTotals", 4}, {"SpawnTotals", 9}, {"SpawnSpotTotals", 4}, {"WorldCharacterBoards", 5}} {
				mock.ExpectQuery("name: " + q.name + " :").WithArgs(anyArgs(q.args)...).WillReturnError(tt.err)
			}

			// when
			_, err1 := repo.PublicWorlds(ctx)
			_, err2 := repo.WorldTotals(ctx, stats.WorldFilter{})
			_, err3 := repo.WorldGuildTotals(ctx, stats.WorldFilter{})
			_, err4 := repo.SpawnTotals(ctx, stats.WorldQuery{})
			_, err5 := repo.SpawnSpotTotals(ctx, stats.WorldFilter{})
			_, err6 := repo.WorldCharacterBoards(ctx, stats.WorldFilter{}, 1)

			// then
			for _, err := range []error{err1, err2, err3, err4, err5, err6} {
				assert.ErrorIs(t, err, tt.want)
			}
		})
	}
}

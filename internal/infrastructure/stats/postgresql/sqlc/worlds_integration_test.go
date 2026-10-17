package sqlc_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/stats/postgresql/sqlc"
	"spot-assistant/internal/ports"
)

// TestWorldStatsRepository_Queries runs the cross-guild queries against a real database. Set
// LETTER_TEST_DATABASE_URL to run it.
func TestWorldStatsRepository_Queries(t *testing.T) {
	dsn := os.Getenv("LETTER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LETTER_TEST_DATABASE_URL not set")
	}

	// given
	ctx := context.Background()
	_, err := postgresql.Migrate(ctx, dsn, zap.NewNop().Sugar())
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	guilds := []string{"it-worlds-a", "it-worlds-b", "it-worlds-c", "it-worlds-d", "it-worlds-free", "it-worlds-absent", "it-worlds-noworld"}
	cleanup := func() {
		_, err := pool.Exec(ctx, `DELETE FROM loot_sessions WHERE substring(content_hash FROM 1 FOR 10) = 'it-worlds-'::bytea`)
		require.NoError(t, err)
		for _, table := range []string{"web_reservation", "web_spot", "guilds_world", "guilds"} {
			_, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE guild_id = ANY($1)`, guilds)
			require.NoError(t, err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	exec := func(q string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO guilds (guild_id, name, icon, bot_present, premium, premium_forever) VALUES
		('it-worlds-a', 'It Alpha Guild', 'abc', true, true, false),
		('it-worlds-b', 'It Bravo Guild', '', true, false, true),
		('it-worlds-c', 'It Charlie Guild', '', true, true, false),
		('it-worlds-d', 'It Delta Guild', '', true, true, false),
		('it-worlds-free', 'It Free Guild', '', true, false, false),
		('it-worlds-absent', 'It Absent Guild', '', false, true, false),
		('it-worlds-noworld', 'It Worldless Guild', '', true, true, false)`)
	exec(`INSERT INTO guilds_world (guild_id, world_name) VALUES
		('it-worlds-a', 'Itworlda'), ('it-worlds-c', 'Itworlda'), ('it-worlds-d', ' itworlda '),
		('it-worlds-b', 'itworldb'), ('it-worlds-free', 'Itworlda'), ('it-worlds-absent', 'Itworldc')`)
	exec(`INSERT INTO web_spot (id, name, created_at, guild_id) VALUES
		(990201, 'It Hero Cave', now(), 'it-worlds-a'), (990202, 'It Banuta', now(), 'it-worlds-a'),
		(990203, 'it hero cave ', now(), 'it-worlds-b'), (990204, 'It Hero Cave', now(), 'it-worlds-c'),
		(990205, 'It Secret', now(), 'it-worlds-free'), (990206, 'It Secret', now(), 'it-worlds-absent'),
		(990207, 'It Secret', now(), 'it-worlds-noworld')`)
	at := func(d, h int) time.Time { return time.Date(2026, 9, d, h, 0, 0, 0, time.UTC) }
	for _, r := range []struct {
		id, spot   int64
		guild      string
		author     string
		start, end time.Time
	}{
		{990201, 990201, "it-worlds-a", "Alpha/Bravo", at(1, 10), at(1, 12)},
		{990202, 990202, "it-worlds-a", "Alpha", at(2, 10), at(2, 11)},
		{990203, 990203, "it-worlds-b", "Charlie", at(1, 10), at(1, 13)},
		{990204, 990205, "it-worlds-free", "Alpha", at(1, 10), at(1, 12)},
		{990205, 990201, "it-worlds-a", "Delta", at(3, 10), at(3, 11)},
		{990206, 990206, "it-worlds-absent", "Alpha", at(1, 10), at(1, 12)},
	} {
		exec(`INSERT INTO web_reservation (id, author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id)
			VALUES ($1, $2, now(), $3, $4, $5, $6, '')`, r.id, r.author, r.start, r.end, r.spot, r.guild)
	}
	exec(`INSERT INTO reservation_experience (reservation_id, character_key, character_name, gain, status) VALUES
		(990201, 'alpha', 'Alpha', 1000, 'ok'), (990203, 'charlie', 'Charlie', 3000, 'ok'), (990204, 'alpha', 'Alpha', 99999, 'ok')`)
	exec(`INSERT INTO loot_sessions (id, content_hash, started_at, ended_at, duration_seconds, members, loot, supplies, balance, damage, healing) VALUES
		(990201, 'it-worlds-a', $1, $2, 3600, 2, 500000, 100000, 400000, 0, 0),
		(990202, 'it-worlds-b', $3, $4, 1800, 1, 10000, 100000, -90000, 0, 0)`, at(1, 10), at(1, 11), at(1, 10), at(1, 10).Add(30*time.Minute))
	exec(`INSERT INTO loot_session_members (session_id, character_key, character_name, position, loot, supplies, balance, damage, healing) VALUES
		(990201, 'alpha', 'Alpha', 0, 0, 0, 0, 0, 0), (990201, 'bravo', 'Bravo', 1, 0, 0, 0, 0, 0),
		(990202, 'charlie', 'Charlie', 0, 0, 0, 0, 0, 0)`)
	exec(`INSERT INTO reservation_loot (reservation_id, session_id, overlap_seconds) VALUES (990201, 990201, 3600), (990203, 990202, 1800)`)

	repo := sqlc.NewWorldStatsRepository(pool)
	sept := stats.WorldFilter{From: at(1, 0), To: at(30, 0)}
	onWorld := func(key string) stats.WorldFilter {
		f := sept
		f.WorldKey = key
		return f
	}
	ours := func(key string) bool { return strings.HasPrefix(key, "itworld") || strings.HasPrefix(key, "it ") }
	worldA := stats.Totals{
		Reservations: 3, Seconds: 4 * 3600, ExpReservations: 1, ExpSeconds: 2 * 3600, Exp: 1000,
		LootReservations: 1, LootSeconds: 3600, Profit: 400000, Loot: 500000, Supplies: 100000,
	}
	worldB := stats.Totals{
		Reservations: 1, Seconds: 3 * 3600, ExpReservations: 1, ExpSeconds: 3 * 3600, Exp: 3000,
		LootReservations: 1, LootSeconds: 1800, Profit: -90000, Loot: 10000, Supplies: 100000,
	}

	t.Run("public worlds: premium guilds with the bot and a world, keyed by the trimmed lower name", func(t *testing.T) {
		// when
		worlds, err := repo.PublicWorlds(ctx)

		// then
		require.NoError(t, err)
		var got []stats.World
		for _, w := range worlds {
			if ours(w.Key) {
				got = append(got, w)
			}
		}
		assert.Equal(t, []stats.World{{Key: "itworlda", Name: "Itworlda"}, {Key: "itworldb", Name: "itworldb"}}, got)
	})

	t.Run("world totals", func(t *testing.T) {
		// when
		all, err1 := repo.WorldTotals(ctx, sept)
		one, err2 := repo.WorldTotals(ctx, onWorld("itworldb"))

		// then
		require.NoError(t, errors.Join(err1, err2))
		var got []stats.WorldRow
		for _, w := range all {
			if ours(w.Key) {
				got = append(got, w)
			}
		}
		assert.Equal(t, []stats.WorldRow{
			{Key: "itworlda", Name: "Itworlda", Guilds: 3, Totals: worldA},
			{Key: "itworldb", Name: "itworldb", Guilds: 1, Totals: worldB},
		}, got)
		assert.Equal(t, []stats.WorldRow{{Key: "itworldb", Name: "itworldb", Guilds: 1, Totals: worldB}}, one)
	})

	t.Run("the guilds of a world, every one listed", func(t *testing.T) {
		// when
		rows, err := repo.WorldGuildTotals(ctx, onWorld("itworlda"))

		// then
		require.NoError(t, err)
		require.Len(t, rows, 3)
		assert.Equal(t, stats.WorldGuildRow{GuildID: "it-worlds-a", Name: "It Alpha Guild", Icon: "abc", Totals: worldA}, rows[0])
		assert.Equal(t, []string{"it-worlds-c", "it-worlds-d"}, []string{rows[1].GuildID, rows[2].GuildID})
		assert.Zero(t, rows[1].Reservations)
	})

	t.Run("spawns group respawn names across guilds and worlds", func(t *testing.T) {
		// given
		query := func(f stats.WorldFilter, key stats.SortKey, asc bool) stats.WorldQuery {
			return stats.WorldQuery{WorldFilter: f, Sort: stats.Sort{Key: key, Asc: asc}}
		}
		minLoot := query(sept, stats.SortProfitPerHour, false)
		minLoot.MinLootSeconds = 2 * 3600
		minExp := query(sept, stats.SortExpPerHour, false)
		minExp.MinExpSeconds = 5 * 3600

		// when
		byWorlds, err1 := repo.SpawnTotals(ctx, query(sept, stats.SortWorlds, false))
		byProfitH, err2 := repo.SpawnTotals(ctx, query(sept, stats.SortProfitPerHour, true))
		onB, err3 := repo.SpawnTotals(ctx, query(onWorld("itworldb"), stats.SortHours, false))
		tooShort, err4 := repo.SpawnTotals(ctx, minLoot)
		longEnough, err5 := repo.SpawnTotals(ctx, minExp)

		// then
		require.NoError(t, errors.Join(err1, err2, err3, err4, err5))
		var keys []string
		for _, r := range byWorlds.Rows {
			if ours(r.Key) {
				keys = append(keys, r.Key)
			}
		}
		assert.Equal(t, []string{"it hero cave", "it banuta"}, keys)
		hero := byWorlds.Rows[0]
		assert.Equal(t, "It Hero Cave", hero.Name)
		assert.Equal(t, 2, hero.Worlds)
		assert.Equal(t, stats.Totals{
			Reservations: 3, Seconds: 6 * 3600, ExpReservations: 2, ExpSeconds: 5 * 3600, Exp: 4000,
			LootReservations: 2, LootSeconds: 5400, Profit: 310000, Loot: 510000, Supplies: 200000,
		}, hero.Totals)
		assert.Equal(t, "it hero cave", byProfitH.Rows[0].Key, "rows without data come last, ascending too")
		require.Len(t, onB.Rows, 1)
		assert.Equal(t, 1, onB.Rows[0].Worlds)
		assert.Equal(t, int64(1), onB.Rows[0].Reservations)
		assert.Empty(t, tooShort.Rows)
		require.Len(t, longEnough.Rows, 1)
		assert.Equal(t, "it hero cave", longEnough.Rows[0].Key)
	})

	t.Run("the respawns of a spawn", func(t *testing.T) {
		// given
		f := sept
		f.SpawnKey = "it hero cave"

		// when
		rows, err := repo.SpawnSpotTotals(ctx, f)

		// then
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Equal(t, "Itworlda", rows[0].World)
		assert.Equal(t, "it-worlds-a", rows[0].GuildID)
		assert.Equal(t, "It Alpha Guild", rows[0].GuildName)
		assert.Equal(t, int64(990201), rows[0].SpotID)
		assert.Equal(t, int64(2), rows[0].Reservations)
		assert.Equal(t, "itworldb", rows[1].WorldKey)
		assert.Equal(t, "it hero cave ", rows[1].SpotName)
		assert.Equal(t, int64(-90000), rows[1].Profit)
	})

	t.Run("spawn name: the most common spelling of public guilds", func(t *testing.T) {
		// when
		hero, err1 := repo.SpawnName(ctx, "it hero cave")
		_, err2 := repo.SpawnName(ctx, "it secret")

		// then
		require.NoError(t, err1)
		assert.Equal(t, "It Hero Cave", hero)
		assert.ErrorIs(t, err2, ports.ErrNotFound, "only guilds without premium, the bot or a world have it")
	})

	t.Run("world character boards", func(t *testing.T) {
		// when
		boards, err := repo.WorldCharacterBoards(ctx, onWorld("itworlda"), 10)
		top1, err2 := repo.WorldCharacterBoards(ctx, onWorld("itworlda"), 1)

		// then
		require.NoError(t, errors.Join(err, err2))
		require.Len(t, boards.ByExp, 1)
		assert.Equal(t, stats.WorldCharacterRow{GuildID: "it-worlds-a", CharacterRow: stats.CharacterRow{
			Key: "alpha", Name: "Alpha", Totals: stats.Totals{
				Reservations: 2, Seconds: 3 * 3600, ExpReservations: 1, ExpSeconds: 2 * 3600, Exp: 1000,
				LootReservations: 1, LootSeconds: 3600, Profit: 200000, Loot: 250000, Supplies: 50000,
			},
		}}, boards.ByExp[0])
		require.Len(t, boards.ByProfit, 2)
		assert.Equal(t, []string{"alpha", "bravo"}, []string{boards.ByProfit[0].Key, boards.ByProfit[1].Key})
		assert.Len(t, top1.ByProfit, 1)
	})
}

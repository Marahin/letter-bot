package sqlc_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/stats/postgresql/sqlc"
	"spot-assistant/internal/ports"
)

const itGuild = "it-stats-guild"

// TestStatsRepository_Queries runs every stats query against a real database. Set LETTER_TEST_DATABASE_URL to
// run it, e.g. postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable.
func TestStatsRepository_Queries(t *testing.T) {
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
	cleanup := func() {
		_, err := pool.Exec(ctx, `DELETE FROM web_reservation WHERE guild_id = $1`, itGuild)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM web_spot WHERE guild_id = $1`, itGuild)
		require.NoError(t, err)
	}
	cleanup()
	t.Cleanup(cleanup)

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	at := func(d, h, m int) time.Time { return time.Date(2026, 9, d, h, m, 0, 0, berlin) }
	_, err = pool.Exec(ctx, `INSERT INTO web_spot (id, name, created_at, guild_id) VALUES
		(990001, 'It Hero Cave', now(), $1), (990002, 'It Dragons', now(), $1)`, itGuild)
	require.NoError(t, err)
	type res struct {
		id           int64
		spot         int64
		user, author string
		start, end   time.Time
	}
	for _, r := range []res{
		{990001, 990001, "u1", "quiet nyx/Storm Quiet", at(1, 10, 0), at(1, 12, 0)},
		{990002, 990001, "u2", "Storm Quiet", at(1, 23, 30), at(2, 1, 30)},
		{990003, 990002, "", "Free Text", at(2, 10, 0), at(2, 11, 0)},
		{990004, 990002, "u1", "quiet nyx / Quiet Nyx", at(3, 10, 0), at(3, 11, 0)},
		{990005, 990002, "u1", "Quiet Nyx", at(3, 20, 0), at(3, 21, 0)},
		{990006, 990001, "u1", "Quiet Nyx", time.Date(2026, 8, 1, 10, 0, 0, 0, berlin), time.Date(2026, 8, 1, 11, 0, 0, 0, berlin)},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO web_reservation (id, author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id)
			VALUES ($1, $2, now(), $3, $4, $5, $6, $7)`, r.id, r.author, r.start, r.end, r.spot, itGuild, r.user)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO reservation_experience (reservation_id, character_key, character_name, gain, status) VALUES
		(990001, 'quiet nyx', 'Quiet Nyx', 1000, 'ok'),
		(990001, 'storm quiet', 'Storm Quiet', NULL, 'no_data'),
		(990002, 'storm quiet', 'Storm Quiet', 500, 'ok'),
		(990004, 'quiet nyx', 'Quiet Nyx', -50, 'ok')`)
	require.NoError(t, err)

	repo, err := sqlc.NewStatsRepository(pool, "Europe/Berlin")
	require.NoError(t, err)
	all := stats.Filter{GuildID: itGuild, From: at(1, 0, 0), To: at(4, 0, 0)}
	every := func(f stats.Filter) stats.Query {
		return stats.Query{Filter: f, Sort: stats.Sort{Key: stats.SortHours}}
	}
	tot := func(res, secs, expRes, expSecs, exp int64) stats.Totals {
		return stats.Totals{Reservations: res, Seconds: secs, ExpReservations: expRes, ExpSeconds: expSecs, Exp: exp}
	}

	t.Run("spot totals", func(t *testing.T) {
		// when
		page, err := repo.SpotTotals(ctx, every(all))

		// then
		require.NoError(t, err)
		assert.Equal(t, 2, page.Total)
		got := map[string]stats.Totals{}
		for _, r := range page.Rows {
			got[r.Name] = r.Totals
		}
		assert.Equal(t, map[string]stats.Totals{
			"It Hero Cave": tot(2, 4*3600, 2, 4*3600, 1500),
			"It Dragons":   tot(3, 3*3600, 1, 3600, -50),
		}, got)
	})

	t.Run("spot totals of one character count only its own experience", func(t *testing.T) {
		// given
		f := all
		f.CharacterKey = "quiet nyx"

		// when
		page, err := repo.SpotTotals(ctx, every(f))

		// then
		require.NoError(t, err)
		got := map[string]stats.Totals{}
		for _, r := range page.Rows {
			got[r.Name] = r.Totals
		}
		assert.Equal(t, map[string]stats.Totals{
			"It Hero Cave": tot(1, 2*3600, 1, 2*3600, 1000),
			"It Dragons":   tot(2, 2*3600, 1, 3600, -50),
		}, got)
	})

	t.Run("player totals leave out free-text authors", func(t *testing.T) {
		// when
		page, err := repo.PlayerTotals(ctx, every(all))

		// then
		require.NoError(t, err)
		got := map[string]stats.PlayerRow{}
		for _, r := range page.Rows {
			got[r.UserID] = r
		}
		require.Len(t, got, 2)
		assert.Equal(t, tot(3, 4*3600, 2, 3*3600, 950), got["u1"].Totals)
		assert.Equal(t, "Quiet Nyx", got["u1"].Name, "the author of the latest reservation, in any range")
		assert.Equal(t, tot(1, 2*3600, 1, 2*3600, 500), got["u2"].Totals)
	})

	t.Run("character totals split and dedupe the author", func(t *testing.T) {
		// when
		page, err := repo.CharacterTotals(ctx, every(all))

		// then
		require.NoError(t, err)
		got := map[string]stats.Totals{}
		names := map[string]string{}
		for _, r := range page.Rows {
			got[r.Key] = r.Totals
			names[r.Key] = r.Name
		}
		assert.Equal(t, map[string]stats.Totals{
			"quiet nyx":   tot(3, 4*3600, 2, 3*3600, 950),
			"storm quiet": tot(2, 4*3600, 1, 2*3600, 500),
			"free text":   tot(1, 3600, 0, 0, 0),
		}, got)
		assert.Equal(t, "Quiet Nyx", names["quiet nyx"], "the spelling of the latest reservation")
	})

	t.Run("character totals of one player and of one character", func(t *testing.T) {
		// given
		byUser, byChar := all, all
		byUser.UserID = "u1"
		byChar.CharacterKey = "storm quiet"

		// when
		userPage, err1 := repo.CharacterTotals(ctx, every(byUser))
		charPage, err2 := repo.CharacterTotals(ctx, every(byChar))

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Len(t, userPage.Rows, 2)
		require.Len(t, charPage.Rows, 1)
		assert.Equal(t, "Storm Quiet", charPage.Rows[0].Name)
		assert.Equal(t, tot(2, 4*3600, 1, 2*3600, 500), charPage.Rows[0].Totals)
	})

	t.Run("totals sort in SQL with no data last and count the rows before the limit", func(t *testing.T) {
		// given
		sorted := func(key stats.SortKey, asc bool, limit int) stats.Query {
			return stats.Query{Filter: all, Sort: stats.Sort{Key: key, Asc: asc}, Limit: limit}
		}
		keys := func(p stats.Page[stats.CharacterRow]) []string {
			var out []string
			for _, r := range p.Rows {
				out = append(out, r.Key)
			}
			return out
		}

		// when
		byExpDesc, err1 := repo.CharacterTotals(ctx, sorted(stats.SortExp, false, 0))
		byExpAsc, err2 := repo.CharacterTotals(ctx, sorted(stats.SortExp, true, 0))
		byNameDesc, err3 := repo.CharacterTotals(ctx, sorted(stats.SortName, false, 2))
		byReservations, err4 := repo.CharacterTotals(ctx, sorted(stats.SortReservations, false, 1))
		spotsByName, err5 := repo.SpotTotals(ctx, sorted(stats.SortName, true, 1))
		playersByName, err6 := repo.PlayerTotals(ctx, sorted(stats.SortName, true, 0))

		// then
		require.NoError(t, errors.Join(err1, err2, err3, err4, err5, err6))
		assert.Equal(t, []string{"quiet nyx", "storm quiet", "free text"}, keys(byExpDesc))
		assert.Equal(t, []string{"storm quiet", "quiet nyx", "free text"}, keys(byExpAsc))
		assert.Equal(t, []string{"storm quiet", "quiet nyx"}, keys(byNameDesc))
		assert.Equal(t, 3, byNameDesc.Total)
		assert.Equal(t, []string{"quiet nyx"}, keys(byReservations))
		require.Len(t, spotsByName.Rows, 1)
		assert.Equal(t, "It Dragons", spotsByName.Rows[0].Name)
		assert.Equal(t, 2, spotsByName.Total)
		require.Len(t, playersByName.Rows, 2)
		assert.Equal(t, "Quiet Nyx", playersByName.Rows[0].Name)
	})

	t.Run("character leaderboards", func(t *testing.T) {
		// when
		boards, err := repo.CharacterLeaderboards(ctx, all, 10, 3*3600)
		top1, err2 := repo.CharacterLeaderboards(ctx, all, 1, 0)
		none, err3 := repo.CharacterLeaderboards(ctx, all, 10, 100*3600)

		// then
		require.NoError(t, errors.Join(err, err2, err3))
		assert.Equal(t, 3, boards.Characters)
		require.Len(t, boards.ByExp, 2)
		assert.Equal(t, "Quiet Nyx", boards.ByExp[0].Name)
		assert.Equal(t, tot(3, 4*3600, 2, 3*3600, 950), boards.ByExp[0].Totals)
		assert.Equal(t, "storm quiet", boards.ByExp[1].Key)
		require.Len(t, boards.ByExpPerHour, 1, "storm quiet has only 2 hours with data")
		assert.Equal(t, "quiet nyx", boards.ByExpPerHour[0].Key)
		require.Len(t, top1.ByExp, 1)
		require.Len(t, top1.ByExpPerHour, 1)
		assert.Equal(t, "quiet nyx", top1.ByExpPerHour[0].Key, "317 exp/h beats 250")
		assert.Equal(t, stats.CharacterBoards{Characters: 3, ByExp: none.ByExp}, none)
	})

	t.Run("daily buckets by the local day of the start", func(t *testing.T) {
		// given
		spotOnly := all
		spotOnly.SpotID = 990001

		// when
		days, err1 := repo.Daily(ctx, all)
		spotDays, err2 := repo.Daily(ctx, spotOnly)

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		require.Len(t, days, 3)
		assert.Equal(t, at(1, 0, 0), days[0].Day)
		assert.Equal(t, tot(2, 4*3600, 2, 4*3600, 1500), days[0].Totals)
		assert.Equal(t, int64(1), days[1].Reservations)
		assert.Equal(t, int64(2), days[2].Reservations)
		require.Len(t, spotDays, 1)
		assert.Equal(t, int64(2), spotDays[0].Reservations)
	})

	t.Run("reservation days", func(t *testing.T) {
		// when
		days, err := repo.ReservationDays(ctx, itGuild, time.Date(2026, 7, 1, 0, 0, 0, 0, berlin), at(30, 0, 0))

		// then
		require.NoError(t, err)
		assert.Equal(t, []time.Time{time.Date(2026, 8, 1, 0, 0, 0, 0, berlin), at(1, 0, 0), at(2, 0, 0), at(3, 0, 0)}, days)
	})

	t.Run("latest player name", func(t *testing.T) {
		// when
		name, err1 := repo.LatestPlayerName(ctx, itGuild, "u2")
		_, err2 := repo.LatestPlayerName(ctx, itGuild, "nobody")

		// then
		require.NoError(t, err1)
		assert.Equal(t, "Storm Quiet", name)
		assert.ErrorIs(t, err2, ports.ErrNotFound)
	})

	t.Run("character reservations, newest first, with the character's own gain", func(t *testing.T) {
		// given
		f := all
		f.CharacterKey = "quiet nyx"

		// when
		rows, err := repo.CharacterReservations(ctx, f, 10)

		// then
		require.NoError(t, err)
		require.Len(t, rows, 3)
		assert.Equal(t, int64(990005), rows[0].ID)
		assert.Equal(t, experience.Status(""), rows[0].Status)
		assert.Nil(t, rows[0].Gain)
		assert.Equal(t, int64(990004), rows[1].ID)
		assert.Equal(t, "It Dragons", rows[1].SpotName)
		require.NotNil(t, rows[1].Gain)
		assert.Equal(t, int64(-50), *rows[1].Gain)
		assert.Equal(t, experience.StatusOK, rows[2].Status)
	})
}

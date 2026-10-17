package sqlc_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/loot"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/loot/postgresql/sqlc"
	reservationsqlc "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
)

const (
	itPremium    = "it-loot-premium"
	itFree       = "it-loot-free"
	itBotAbsent  = "it-loot-absent"
	itHashPrefix = "it-loot-"
	itAuthor     = "it-loot-author"
)

// TestLootRepository_Queries runs the loot queries against a real database. Set
// LETTER_TEST_DATABASE_URL to run it.
func TestLootRepository_Queries(t *testing.T) {
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
	guilds := []string{itPremium, itFree, itBotAbsent}
	cleanup := func() {
		_, err := pool.Exec(ctx, `DELETE FROM loot_sessions WHERE substring(content_hash FROM 1 FOR $1) = $2`, len(itHashPrefix), []byte(itHashPrefix))
		require.NoError(t, err)
		for _, table := range []string{"web_reservation", "web_spot", "guilds"} {
			_, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE guild_id = ANY($1)`, guilds)
			require.NoError(t, err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err = pool.Exec(ctx, `INSERT INTO guilds (guild_id, name, bot_present, premium) VALUES
		($1, 'It Premium', true, true), ($2, 'It Free', true, false), ($3, 'It Absent', false, true)`, itPremium, itFree, itBotAbsent)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO web_spot (id, name, created_at, guild_id) VALUES
		(990101, 'It Hero Cave', now(), $1), (990102, 'It Dragons', now(), $1), (990103, 'It Free Cave', now(), $2), (990104, 'It Absent Cave', now(), $3)`,
		itPremium, itFree, itBotAbsent)
	require.NoError(t, err)

	at := func(h, m int) time.Time { return time.Date(2026, 9, 25, h, m, 0, 0, time.UTC) }
	type res struct {
		id         int64
		spot       int64
		guild      string
		author     string
		start, end time.Time
	}
	for _, r := range []res{
		{990101, 990101, itPremium, "Quiet Nyx /  STORM QUIET ", at(10, 0), at(12, 0)},
		{990102, 990102, itPremium, "Storm Quiet", at(11, 0), at(14, 0)},
		{990103, 990102, itPremium, "Someone Else", at(9, 0), at(11, 0)},
		// Longer than a reservation may be: it starts before the window.
		{990104, 990101, itPremium, "Storm Quiet", at(6, 0), at(10, 30)},
		{990105, 990103, itFree, "Storm Quiet", at(10, 0), at(12, 0)},
		{990106, 990104, itBotAbsent, "Storm Quiet", at(10, 0), at(12, 0)},
		{990107, 990102, itPremium, "Storm Quiet", at(14, 0), at(15, 0)},
		{990108, 990102, itPremium, "Storm Quiet", at(16, 0), at(18, 0)},
		{990109, 990102, itPremium, "Storm Quiet", at(19, 0), at(21, 0)},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO web_reservation (id, author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id)
			VALUES ($1, $2, now(), $3, $4, $5, $6, $7)`, r.id, r.author, r.start, r.end, r.spot, r.guild, itAuthor)
		require.NoError(t, err)
	}
	repo := sqlc.NewLootRepository(pool)
	newSession := func(hash string, from, to time.Time) loot.Session {
		return loot.Session{
			Hash: []byte(itHashPrefix + hash), StartedAt: from, EndedAt: to, Duration: to.Sub(from), PartySize: 2,
			Members: []loot.SessionMember{
				{Member: loot.Member{Name: "Storm Quiet", Balance: 100}, Key: "storm quiet"},
				{Member: loot.Member{Name: "Quiet Nyx", Leader: true, Balance: 50}, Key: "quiet nyx", Position: 1},
			},
			Balance: 150,
		}
	}

	t.Run("a session is stored once, with its first submitter", func(t *testing.T) {
		// given
		s := newSession("dedupe", at(10, 30), at(11, 30))
		s.SubmittedBy = itAuthor
		again := s
		again.SubmittedBy = "someone else"

		// when
		id1, err1 := repo.SaveSession(ctx, s)
		id2, err2 := repo.SaveSession(ctx, again)

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, id1, id2)
		var members int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM loot_session_members WHERE session_id = $1`, id1).Scan(&members))
		assert.Equal(t, 2, members)
		var submitter string
		require.NoError(t, pool.QueryRow(ctx, `SELECT submitted_by FROM loot_sessions WHERE id = $1`, id1).Scan(&submitter))
		assert.Equal(t, itAuthor, submitter)
	})

	t.Run("candidates: premium guilds with the bot, the author's characters, the start window", func(t *testing.T) {
		// when
		got, err := repo.MatchCandidates(ctx, at(10, 30), at(11, 30), []string{"storm quiet"}, 3*time.Hour)

		// then
		require.NoError(t, err)
		ids := make([]int64, len(got))
		for i, c := range got {
			ids[i] = c.ReservationID
		}
		assert.Equal(t, []int64{990102, 990101}, ids, "newest first")
		assert.Equal(t, loot.Candidate{
			ReservationID: 990101, GuildID: itPremium, GuildName: "It Premium", SpotID: 990101, SpotName: "It Hero Cave",
			Author: "Quiet Nyx /  STORM QUIET ", AuthorDiscordID: itAuthor, StartAt: at(10, 0), EndAt: at(12, 0),
		}, normalize(got[1]))
	})

	t.Run("a larger overlap replaces the pick, a tie or a smaller one does not", func(t *testing.T) {
		// given
		first, err := repo.SaveSession(ctx, newSession("first", at(11, 0), at(12, 0)))
		require.NoError(t, err)
		tie, err := repo.SaveSession(ctx, newSession("tie", at(10, 0), at(11, 0)))
		require.NoError(t, err)
		smaller, err := repo.SaveSession(ctx, newSession("smaller", at(10, 0), at(10, 30)))
		require.NoError(t, err)
		larger, err := repo.SaveSession(ctx, newSession("larger", at(10, 0), at(11, 30)))
		require.NoError(t, err)

		// when
		okFirst, err1 := repo.Pick(ctx, loot.Attribution{SessionID: first, ReservationID: 990101, OverlapSeconds: 3600})
		okAgain, err2 := repo.Pick(ctx, loot.Attribution{SessionID: first, ReservationID: 990101, OverlapSeconds: 3600})
		okTie, err3 := repo.Pick(ctx, loot.Attribution{SessionID: tie, ReservationID: 990101, OverlapSeconds: 3600})
		okSmaller, err4 := repo.Pick(ctx, loot.Attribution{SessionID: smaller, ReservationID: 990101, OverlapSeconds: 1800})
		okLarger, err5 := repo.Pick(ctx, loot.Attribution{SessionID: larger, ReservationID: 990101, OverlapSeconds: 5400})

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		require.NoError(t, err4)
		require.NoError(t, err5)
		assert.True(t, okFirst)
		assert.True(t, okAgain, "the same session again still counts")
		assert.False(t, okTie)
		assert.False(t, okSmaller)
		assert.True(t, okLarger)
		assert.Equal(t, larger, pickOf(t, pool, 990101))
	})

	t.Run("a pick moves the session, a lost pick keeps the old one", func(t *testing.T) {
		// given
		s, err := repo.SaveSession(ctx, newSession("moving", at(14, 0), at(14, 50)))
		require.NoError(t, err)
		ok, err := repo.Pick(ctx, loot.Attribution{SessionID: s, ReservationID: 990102, OverlapSeconds: 600})
		require.NoError(t, err)
		require.True(t, ok)
		holder, err := repo.SaveSession(ctx, newSession("holder", at(14, 0), at(15, 0)))
		require.NoError(t, err)
		ok, err = repo.Pick(ctx, loot.Attribution{SessionID: holder, ReservationID: 990107, OverlapSeconds: 3600})
		require.NoError(t, err)
		require.True(t, ok)

		// when
		lost, err1 := repo.Pick(ctx, loot.Attribution{SessionID: s, ReservationID: 990107, OverlapSeconds: 3000})
		moved, err2 := repo.Pick(ctx, loot.Attribution{SessionID: s, ReservationID: 990103, OverlapSeconds: 3000})

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.False(t, lost)
		assert.True(t, moved)
		assert.Equal(t, holder, pickOf(t, pool, 990107))
		assert.Equal(t, s, pickOf(t, pool, 990103))
		assert.Zero(t, pickOf(t, pool, 990102), "the session counts for one reservation")
	})

	t.Run("the author's session wins over a larger overlap, and no one else's replaces it", func(t *testing.T) {
		// given
		fake, err := repo.SaveSession(ctx, newSession("fake", at(16, 0), at(18, 0)))
		require.NoError(t, err)
		own, err := repo.SaveSession(ctx, newSession("own", at(16, 30), at(17, 30)))
		require.NoError(t, err)
		larger, err := repo.SaveSession(ctx, newSession("fake-larger", at(16, 0), at(17, 59)))
		require.NoError(t, err)
		ok, err := repo.Pick(ctx, loot.Attribution{SessionID: fake, ReservationID: 990108, OverlapSeconds: 7200})
		require.NoError(t, err)
		require.True(t, ok)

		// when
		okOwn, err1 := repo.Pick(ctx, loot.Attribution{SessionID: own, ReservationID: 990108, OverlapSeconds: 3600, ByAuthor: true})
		okLarger, err2 := repo.Pick(ctx, loot.Attribution{SessionID: larger, ReservationID: 990108, OverlapSeconds: 7140})
		okOwnAgain, err3 := repo.Pick(ctx, loot.Attribution{SessionID: own, ReservationID: 990108, OverlapSeconds: 3600})

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		assert.True(t, okOwn, "the author's session replaces a larger overlap")
		assert.False(t, okLarger, "someone else's session never replaces the author's")
		assert.True(t, okOwnAgain)
		assert.Equal(t, own, pickOf(t, pool, 990108))
		var byAuthor bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT by_author FROM reservation_loot WHERE reservation_id = 990108`).Scan(&byAuthor))
		assert.True(t, byAuthor, "the same session pasted again by someone else stays the author's")
	})

	t.Run("an author's session is replaced only by a larger overlap of the author", func(t *testing.T) {
		// given
		first, err := repo.SaveSession(ctx, newSession("own-first", at(19, 0), at(20, 0)))
		require.NoError(t, err)
		second, err := repo.SaveSession(ctx, newSession("own-second", at(19, 0), at(20, 30)))
		require.NoError(t, err)
		ok, err := repo.Pick(ctx, loot.Attribution{SessionID: first, ReservationID: 990109, OverlapSeconds: 3600, ByAuthor: true})
		require.NoError(t, err)
		require.True(t, ok)

		// when
		okSecond, err := repo.Pick(ctx, loot.Attribution{SessionID: second, ReservationID: 990109, OverlapSeconds: 5400, ByAuthor: true})

		// then
		require.NoError(t, err)
		assert.True(t, okSecond)
		assert.Equal(t, second, pickOf(t, pool, 990109))
	})

	t.Run("an edit of the spot, the times or the author drops the pick, a save without a change keeps it", func(t *testing.T) {
		// given
		reservations := reservationsqlc.NewReservationRepository(pool)
		booked := reservation.Reservation{ID: 990109, SpotID: 990102, StartAt: at(19, 0), EndAt: at(21, 0), Author: "Storm Quiet", AuthorDiscordID: itAuthor}
		require.NotZero(t, pickOf(t, pool, 990109))

		// when
		errSame := reservations.UpdateReservation(ctx, itPremium, booked)
		kept := pickOf(t, pool, 990109)
		booked.EndAt = at(20, 45)
		errChanged := reservations.UpdateReservation(ctx, itPremium, booked)

		// then
		require.NoError(t, errSame)
		require.NoError(t, errChanged)
		assert.NotZero(t, kept)
		assert.Zero(t, pickOf(t, pool, 990109))
	})

	t.Run("a deleted reservation drops its pick", func(t *testing.T) {
		// when
		_, err := pool.Exec(ctx, `DELETE FROM web_reservation WHERE id = 990103`)

		// then
		require.NoError(t, err)
		assert.Zero(t, pickOf(t, pool, 990103))
	})
}

// pickOf is the session the reservation keeps, 0 for none.
func pickOf(t *testing.T, pool *pgxpool.Pool, reservationID int64) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `SELECT session_id FROM reservation_loot WHERE reservation_id = $1`, reservationID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0
	}
	require.NoError(t, err)
	return id
}

func normalize(c loot.Candidate) loot.Candidate {
	c.StartAt, c.EndAt = c.StartAt.UTC(), c.EndAt.UTC()
	return c
}

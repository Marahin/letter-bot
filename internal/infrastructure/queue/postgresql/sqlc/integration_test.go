package sqlc_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/queue/postgresql/sqlc"
)

const (
	itGuild    = "it-spot-queue"
	itSpot     = int64(990201)
	itArchived = int64(990202)
)

// TestSpotQueueRepository_Queries runs the queue queries against a real
// database. Set LETTER_TEST_DATABASE_URL to run it.
func TestSpotQueueRepository_Queries(t *testing.T) {
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
		// web_reservation keeps its spot; the queue entries go with the guild.
		for _, table := range []string{"web_reservation", "web_spot", "guilds"} {
			_, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE guild_id = $1`, itGuild)
			require.NoError(t, err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	at := func(h, m int) time.Time { return time.Date(2026, 10, 18, h, m, 0, 0, time.UTC) }
	now := at(12, 0).Add(20 * time.Second)
	_, err = pool.Exec(ctx, `INSERT INTO guilds (guild_id, name, bot_present, premium) VALUES ($1, 'It Queue', true, true)`, itGuild)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO web_spot (id, name, created_at, guild_id, archived_at) VALUES
		($1, 'It Queue Cave', now(), $3, NULL), ($2, 'It Queue Archived', now(), $3, now())`, itSpot, itArchived, itGuild)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO web_reservation (id, author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id) VALUES
		(990201, 'Knight', now(), $1, $2, $5, $6, 'k'), (990202, 'Druid', now(), $3, $4, $5, $6, 'd')`,
		at(10, 0), at(12, 0), at(13, 0), at(15, 0), itSpot, itGuild)
	require.NoError(t, err)
	repo := sqlc.NewSpotQueueRepository(pool)
	upsert := func(spotID int64, user string, length time.Duration, expires time.Time) *queue.Entry {
		e, err := repo.UpsertEntry(ctx, queue.Entry{GuildID: itGuild, SpotID: spotID, UserID: user, Length: length, ExpiresAt: expires})
		require.NoError(t, err)
		return e
	}
	tomorrow := now.Add(24 * time.Hour)
	hour := upsert(itSpot, "u1", time.Hour, tomorrow)
	halfHour := upsert(itSpot, "u2", 30*time.Minute, tomorrow)
	upsert(itArchived, "u1", time.Hour, tomorrow)
	upsert(itSpot, "u3", time.Hour, now.Add(-time.Minute))

	// when: the window starts at the minute the first hunt ended
	atEnd, err := repo.SelectGuildCandidates(ctx, itGuild, now, at(12, 0))

	// then: the edge counts, as in booking; archived and expired are left out
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{hour.ID: false, halfHour.ID: false}, freeByID(atEnd))

	// when: the window starts a minute later
	later, err := repo.SelectGuildCandidates(ctx, itGuild, now, at(12, 1))

	// then: an hour reaches the next hunt at 13:00, half an hour does not
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{hour.ID: false, halfHour.ID: true}, freeByID(later))
	assert.Equal(t, "It Queue Cave", later[0].SpotName)

	// when: an offer is claimed twice
	first, err1 := repo.ClaimOffer(ctx, halfHour.ID, now)
	second, err2 := repo.ClaimOffer(ctx, halfHour.ID, now.Add(time.Minute))
	claimed, err3 := repo.GetEntry(ctx, halfHour.ID)

	// then: only the first one counts
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	assert.True(t, first)
	assert.False(t, second)
	require.NotNil(t, claimed.OfferedAt)
	assert.True(t, now.Equal(*claimed.OfferedAt))

	// when: the offer is reset, then queued again with another length
	require.NoError(t, repo.ResetOffers(ctx, []int64{halfHour.ID}))
	again, err := repo.ClaimOffer(ctx, halfHour.ID, now)
	require.NoError(t, err)
	requeued := upsert(itSpot, "u2", 90*time.Minute, tomorrow)

	// then: the same entry, its offer cleared
	assert.True(t, again)
	assert.Equal(t, halfHour.ID, requeued.ID)
	assert.Nil(t, requeued.OfferedAt)
	assert.Equal(t, 90*time.Minute, requeued.Length)

	// when: the member's entries are counted and listed
	others, err1 := repo.CountUserEntries(ctx, itGuild, "u1", itSpot, now)
	all, err2 := repo.CountUserEntries(ctx, itGuild, "u1", 0, now)
	mine, err3 := repo.ListUserEntries(ctx, itGuild, "u1", now)

	// then
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	assert.Equal(t, 1, others)
	assert.Equal(t, 2, all)
	assert.Len(t, mine, 2)

	// when: entries are deleted
	expired, err1 := repo.DeleteExpired(ctx, itGuild, now)
	notMine, err2 := repo.DeleteEntry(ctx, hour.ID, "u2")
	mineDeleted, err3 := repo.DeleteEntry(ctx, hour.ID, "u1")
	bySpot, err4 := repo.DeleteUserSpotEntry(ctx, itGuild, "u2", itSpot)
	_, getErr := repo.GetEntry(ctx, hour.ID)

	// then
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	require.NoError(t, err4)
	assert.Equal(t, int64(1), expired)
	assert.False(t, notMine)
	assert.True(t, mineDeleted)
	assert.True(t, bySpot)
	assert.Error(t, getErr)
}

func freeByID(cs []queue.Candidate) map[int64]bool {
	out := make(map[int64]bool, len(cs))
	for _, c := range cs {
		out[c.ID] = c.Free
	}
	return out
}

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

	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/queue/postgresql/sqlc"
	"spot-assistant/internal/ports"
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
	expiredEntry := upsert(itSpot, "u3", time.Hour, now.Add(-time.Minute))
	due := func(startAt time.Time) []int64 {
		entries, err := repo.SelectDueEntries(ctx, queue.DueQuery{GuildID: itGuild, Now: now, StartAt: startAt})
		require.NoError(t, err)
		ids := make([]int64, 0, len(entries))
		for _, e := range entries {
			ids = append(ids, e.ID)
		}
		return ids
	}
	offeredAt := func(id int64) *time.Time {
		e, err := repo.GetEntry(ctx, id)
		require.NoError(t, err)
		return e.OfferedAt
	}

	// when: the window starts at the minute the first hunt ended
	atEnd := due(at(12, 0))

	// then: the edge counts, as in booking; archived and expired are left out
	assert.Empty(t, atEnd)

	// when: the window starts a minute later
	later, err := repo.SelectDueEntries(ctx, queue.DueQuery{GuildID: itGuild, Now: now, StartAt: at(12, 1)})

	// then: an hour reaches the next hunt at 13:00, half an hour does not
	require.NoError(t, err)
	require.Len(t, later, 1)
	assert.Equal(t, halfHour.ID, later[0].ID)
	assert.Equal(t, "It Queue Cave", later[0].SpotName)

	// when: offers are claimed
	taken, err1 := repo.ClaimOffer(ctx, queue.Claim{EntryID: hour.ID, OfferedAt: now, StartAt: at(12, 1)})
	first, err2 := repo.ClaimOffer(ctx, queue.Claim{EntryID: halfHour.ID, OfferedAt: now, StartAt: at(12, 1)})
	second, err3 := repo.ClaimOffer(ctx, queue.Claim{EntryID: halfHour.ID, OfferedAt: now.Add(time.Minute), StartAt: at(12, 1)})
	gone, err4 := repo.ClaimOffer(ctx, queue.Claim{EntryID: expiredEntry.ID, OfferedAt: now, StartAt: at(16, 0)})

	// then: only the first claim of a free, live entry counts
	require.NoError(t, errors.Join(err1, err2, err3, err4))
	assert.False(t, taken, "the respawn is not free for an hour")
	assert.True(t, first)
	assert.False(t, second)
	assert.False(t, gone, "the entry expired")
	require.NotNil(t, offeredAt(halfHour.ID))
	assert.True(t, now.Equal(*offeredAt(halfHour.ID)))
	assert.Empty(t, due(at(12, 1)), "an offered entry is not due again")

	// when: the taken offers are reset while the respawn is still free
	require.NoError(t, repo.ResetTakenOffers(ctx, itGuild, at(12, 1)))

	// then: the offer stays
	assert.NotNil(t, offeredAt(halfHour.ID))

	// when: the taken offers are reset while the respawn is taken
	require.NoError(t, repo.ResetTakenOffers(ctx, itGuild, at(12, 0)))

	// then: the offer is cleared
	assert.Nil(t, offeredAt(halfHour.ID))

	// when: an offer is claimed and unclaimed, then queued again with another length
	again, err := repo.ClaimOffer(ctx, queue.Claim{EntryID: halfHour.ID, OfferedAt: now, StartAt: at(12, 1)})
	require.NoError(t, err)
	require.NoError(t, repo.ResetOffer(ctx, halfHour.ID, now))
	unclaimed := offeredAt(halfHour.ID)
	_, err = repo.ClaimOffer(ctx, queue.Claim{EntryID: halfHour.ID, OfferedAt: now, StartAt: at(12, 1)})
	require.NoError(t, err)
	requeued := upsert(itSpot, "u2", 90*time.Minute, tomorrow)

	// then: the same entry, its offer cleared
	assert.True(t, again)
	assert.Nil(t, unclaimed)
	assert.Equal(t, halfHour.ID, requeued.ID)
	assert.Nil(t, requeued.OfferedAt)
	assert.Equal(t, 90*time.Minute, requeued.Length)

	// when: a pass claims, /queue queues again and claims meanwhile, then the pass unclaims
	stale, err1 := repo.ClaimOffer(ctx, queue.Claim{EntryID: halfHour.ID, OfferedAt: now, StartAt: at(15, 1)})
	upsert(itSpot, "u2", 90*time.Minute, tomorrow)
	newer, err2 := repo.ClaimOffer(ctx, queue.Claim{EntryID: halfHour.ID, OfferedAt: now.Add(time.Minute), StartAt: at(15, 1)})
	err3 = repo.ResetOffer(ctx, halfHour.ID, now)

	// then: the newer claim stays
	require.NoError(t, errors.Join(err1, err2, err3))
	assert.True(t, stale)
	assert.True(t, newer)
	require.NotNil(t, offeredAt(halfHour.ID))
	assert.True(t, now.Add(time.Minute).Equal(*offeredAt(halfHour.ID)))

	// when: the member's entries are counted and listed
	others, err1 := repo.CountUserEntries(ctx, queue.UserCount{GuildID: itGuild, UserID: "u1", ExceptSpotID: itSpot, Now: now})
	all, err2 := repo.CountUserEntries(ctx, queue.UserCount{GuildID: itGuild, UserID: "u1", Now: now})
	mine, err3 := repo.ListUserEntries(ctx, itGuild, "u1", now)

	// then: the count leaves out the archived respawn, the list does not
	require.NoError(t, errors.Join(err1, err2, err3))
	assert.Equal(t, 0, others)
	assert.Equal(t, 1, all)
	assert.Len(t, mine, 2)

	// when: entries are deleted
	expired, err1 := repo.DeleteExpired(ctx, now)
	notMine, err2 := repo.DeleteEntry(ctx, hour.ID, "u2")
	mineDeleted, err3 := repo.DeleteEntry(ctx, hour.ID, "u1")
	bySpot, err4 := repo.DeleteUserSpotEntry(ctx, itGuild, "u2", itSpot)
	_, getErr := repo.GetEntry(ctx, hour.ID)
	_, expiredErr := repo.GetEntry(ctx, expiredEntry.ID)

	// then
	require.NoError(t, errors.Join(err1, err2, err3, err4))
	assert.GreaterOrEqual(t, expired, int64(1), "other guilds' expired entries go too")
	assert.ErrorIs(t, expiredErr, ports.ErrNotFound)
	assert.False(t, notMine)
	assert.True(t, mineDeleted)
	assert.True(t, bySpot)
	assert.Error(t, getErr)
}

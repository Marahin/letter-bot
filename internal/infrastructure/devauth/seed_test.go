//go:build devauth

package devauth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/db/postgresql"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
)

// TestSeed runs against a real database. Set LETTER_TEST_DATABASE_URL to run it.
func TestSeed(t *testing.T) {
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
		for _, q := range []string{
			`DELETE FROM web_reservation WHERE guild_id = $1`,
			`DELETE FROM web_spot WHERE guild_id = $1`,
		} {
			_, err := pool.Exec(ctx, q, PremiumGuildID)
			require.NoError(t, err)
		}
		_, err := pool.Exec(ctx, `DELETE FROM guilds WHERE guild_id = ANY($1)`, []string{PremiumGuildID, LockedGuildID})
		require.NoError(t, err)
	}
	cleanup()
	t.Cleanup(cleanup)
	configs := guildsqlc.NewGuildConfigRepository(pool)
	roles := guildsqlc.NewGuildRoleRepository(pool)

	// when it runs twice
	require.NoError(t, Seed(ctx, pool, configs, roles, time.Now()))
	require.NoError(t, Seed(ctx, pool, configs, roles, time.Now()))

	// then
	premium, err := configs.Get(ctx, PremiumGuildID)
	require.NoError(t, err)
	assert.True(t, premium.IsPremium())
	assert.True(t, premium.BotPresent)
	assert.Equal(t, []string{ManageRoleID}, premium.ManageRoleIDs)
	locked, err := configs.Get(ctx, LockedGuildID)
	require.NoError(t, err)
	assert.False(t, locked.IsPremium())
	var spots, reservations, gains int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM web_spot WHERE guild_id = $1`, PremiumGuildID).Scan(&spots))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM web_reservation WHERE guild_id = $1`, PremiumGuildID).Scan(&reservations))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reservation_experience e JOIN web_reservation r ON r.id = e.reservation_id
		WHERE r.guild_id = $1 AND e.status = 'ok'`, PremiumGuildID).Scan(&gains))
	assert.Equal(t, len(SpotNames), spots)
	assert.Equal(t, 2*seedDays+2, reservations, "the second run adds nothing")
	assert.Positive(t, gains)
}

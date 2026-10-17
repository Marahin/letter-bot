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
	guilds := []string{PremiumGuildID, LockedGuildID, AlliesGuildID}
	cleanup := func() {
		_, err := pool.Exec(ctx, `DELETE FROM loot_sessions WHERE substring(content_hash FROM 1 FOR $1) = $2`, len(lootHashPrefix), []byte(lootHashPrefix))
		require.NoError(t, err)
		for _, table := range []string{"web_reservation", "web_spot", "guilds_world", "guilds"} {
			_, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE guild_id = ANY($1)`, guilds)
			require.NoError(t, err)
		}
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

	allies, err := configs.Get(ctx, AlliesGuildID)
	require.NoError(t, err)
	assert.True(t, allies.IsPremium())
	var worlds []string
	rows, err := pool.Query(ctx, `SELECT world_name FROM guilds_world WHERE guild_id = ANY($1) ORDER BY world_name`, guilds)
	require.NoError(t, err)
	for rows.Next() {
		var w string
		require.NoError(t, rows.Scan(&w))
		worlds = append(worlds, w)
	}
	assert.Equal(t, []string{AlliesWorld, PremiumWorld}, worlds)
	var alliesReservations, picks, sessions, guests, losses int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM web_reservation WHERE guild_id = $1`, AlliesGuildID).Scan(&alliesReservations))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reservation_loot rl JOIN web_reservation r ON r.id = rl.reservation_id
		WHERE r.guild_id = ANY($1)`, guilds).Scan(&picks))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM loot_sessions WHERE substring(content_hash FROM 1 FOR $1) = $2`,
		len(lootHashPrefix), []byte(lootHashPrefix)).Scan(&sessions))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM loot_session_members WHERE character_key = 'sorcerer delta'`).Scan(&guests))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM loot_sessions WHERE balance < 0 AND substring(content_hash FROM 1 FOR $1) = $2`,
		len(lootHashPrefix), []byte(lootHashPrefix)).Scan(&losses))
	assert.Equal(t, alliesDays, alliesReservations)
	assert.Equal(t, 2*seedDays-2*seedDays/3+alliesDays-alliesDays/3, picks, "two of every three ended reservations of each server; the second run adds nothing")
	assert.Equal(t, picks, sessions)
	assert.Equal(t, sessions, guests)
	assert.Positive(t, losses)
}

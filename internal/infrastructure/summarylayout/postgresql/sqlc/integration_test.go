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

	"spot-assistant/internal/core/dto/summarylayout"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/summarylayout/postgresql/sqlc"
)

const itGuild = "it-summary-layout"

// TestSummaryLayoutRepository_Queries runs the queries against a real database.
// Set LETTER_TEST_DATABASE_URL to run it.
func TestSummaryLayoutRepository_Queries(t *testing.T) {
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
		_, err := pool.Exec(ctx, `DELETE FROM guilds WHERE guild_id = $1`, itGuild)
		require.NoError(t, err)
	}
	cleanup()
	t.Cleanup(cleanup)
	_, err = pool.Exec(ctx, `INSERT INTO guilds (guild_id, name, bot_present, premium) VALUES ($1, 'It Layout', true, true)`, itGuild)
	require.NoError(t, err)
	repo := sqlc.NewSummaryLayoutRepository(pool)
	blocked := time.Date(2026, 10, 18, 13, 0, 0, 0, time.UTC)

	// when: a guild without a layout
	empty, err := repo.GetLayout(ctx, itGuild)

	// then
	require.NoError(t, err)
	assert.Equal(t, &summarylayout.Layout{GuildID: itGuild}, empty)

	// when: the layout is saved twice
	require.NoError(t, repo.SaveLayout(ctx, summarylayout.Layout{GuildID: itGuild, CategoryID: "cat", BlockedUntil: &blocked}))
	require.NoError(t, repo.SaveLayout(ctx, summarylayout.Layout{GuildID: itGuild, CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: "h"}))
	layout, err := repo.GetLayout(ctx, itGuild)

	// then: the second replaced the first
	require.NoError(t, err)
	assert.Equal(t, &summarylayout.Layout{GuildID: itGuild, CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: "h"}, layout)

	// when: region channels are saved, replaced and deleted
	require.NoError(t, repo.SaveRegionChannel(ctx, summarylayout.RegionChannel{GuildID: itGuild, Region: "zao", ChannelID: "c-zao"}))
	require.NoError(t, repo.SaveRegionChannel(ctx, summarylayout.RegionChannel{GuildID: itGuild, Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1", "m2"}, ContentHash: "h"}))
	require.NoError(t, repo.SaveRegionChannel(ctx, summarylayout.RegionChannel{GuildID: itGuild, Region: "zao", ChannelID: "c-zao", RetiredAt: &blocked}))
	rows, err := repo.ListRegionChannels(ctx, itGuild)

	// then
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, summarylayout.RegionChannel{GuildID: itGuild, Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1", "m2"}, ContentHash: "h"}, rows[0])
	require.NotNil(t, rows[1].RetiredAt)
	assert.True(t, blocked.Equal(*rows[1].RetiredAt))
	assert.Equal(t, []string{}, rows[1].MessageIDs)

	// when
	require.NoError(t, repo.DeleteRegionChannel(ctx, itGuild, "zao"))
	rows, err = repo.ListRegionChannels(ctx, itGuild)

	// then
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

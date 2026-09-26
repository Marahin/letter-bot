package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/ports"
)

func newMock(t *testing.T) pgxmock.PgxPoolIface {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func newGuildRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"guild_id", "name", "icon", "owner_id", "bot_present", "premium", "premium_forever",
		"command_channel_id", "summary_channel_id",
		"manage_role_ids", "view_role_ids", "reserve_role_ids", "overbook_role_ids",
		"resync_requested_at", "synced_at", "created_at", "updated_at",
	})
}

func addGuildRow(rows *pgxmock.Rows, id string, syncedAt any) *pgxmock.Rows {
	now := time.Now()
	return rows.AddRow(id, "Celesta", "icon", "owner", true, false, true,
		"cmd", "sum",
		[]string{"m1"}, []string{}, nil, []string{"o1"},
		nil, syncedAt, now, now)
}

func TestGuildConfigRepository_Get(t *testing.T) {
	// given
	mock := newMock(t)
	syncedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM guilds WHERE guild_id = \\$1").
		WithArgs("g1").
		WillReturnRows(addGuildRow(newGuildRows(), "g1", syncedAt))
	repo := NewGuildConfigRepository(mock)

	// when
	cfg, err := repo.Get(context.Background(), "g1")

	// then
	require.NoError(t, err)
	assert.Equal(t, "g1", cfg.GuildID)
	assert.True(t, cfg.IsPremium())
	assert.True(t, cfg.BotPresent)
	assert.Equal(t, "cmd", cfg.CommandChannelID)
	assert.Equal(t, "sum", cfg.SummaryChannelID)
	assert.Equal(t, []string{"m1"}, cfg.ManageRoleIDs)
	assert.Equal(t, []string{}, cfg.ReserveRoleIDs)
	assert.Equal(t, []string{"o1"}, cfg.OverbookRoleIDs)
	assert.Nil(t, cfg.ResyncRequestedAt)
	assert.True(t, syncedAt.Equal(*cfg.SyncedAt))
}

func TestGuildConfigRepository_GetNotFound(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("FROM guilds").WithArgs("g1").WillReturnError(pgx.ErrNoRows)
	repo := NewGuildConfigRepository(mock)

	// when
	cfg, err := repo.Get(context.Background(), "g1")

	// then
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ports.ErrNotFound)
}

func TestGuildConfigRepository_ListByIDs(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("guild_id = ANY").
		WithArgs([]string{"g1", "g2"}).
		WillReturnRows(addGuildRow(addGuildRow(newGuildRows(), "g1", nil), "g2", nil))
	repo := NewGuildConfigRepository(mock)

	// when
	cfgs, err := repo.ListByIDs(context.Background(), []string{"g1", "g2"})
	empty, emptyErr := repo.ListByIDs(context.Background(), nil)

	// then
	require.NoError(t, err)
	assert.Len(t, cfgs, 2)
	require.NoError(t, emptyErr)
	assert.Empty(t, empty)
}

func TestGuildConfigRepository_ListPresentByIDs(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("guild_id = ANY.*AND bot_present").
		WithArgs([]string{"g1", "g2"}).
		WillReturnRows(addGuildRow(newGuildRows(), "g1", nil))
	mock.ExpectQuery("AND bot_present").WithArgs(anyArgs(1)...).WillReturnError(errors.New("boom"))
	repo := NewGuildConfigRepository(mock)

	// when
	cfgs, err := repo.ListPresentByIDs(context.Background(), []string{"g1", "g2"})
	failed, failedErr := repo.ListPresentByIDs(context.Background(), []string{"g1"})
	empty, emptyErr := repo.ListPresentByIDs(context.Background(), nil)

	// then
	require.NoError(t, err)
	assert.Len(t, cfgs, 1)
	assert.Error(t, failedErr)
	assert.Empty(t, failed)
	require.NoError(t, emptyErr)
	assert.Empty(t, empty)
}

func TestGuildConfigRepository_ListErrors(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("guild_id = ANY").WithArgs(anyArgs(1)...).WillReturnError(errors.New("boom"))
	mock.ExpectQuery("FROM guilds ORDER BY bot_present DESC").WillReturnError(errors.New("boom"))
	mock.ExpectQuery("resync_requested_at IS NOT NULL").WillReturnError(errors.New("boom"))
	repo := NewGuildConfigRepository(mock)

	// when
	byIDs, byIDsErr := repo.ListByIDs(context.Background(), []string{"g1"})
	all, allErr := repo.ListAll(context.Background())
	resync, resyncErr := repo.ListResyncRequested(context.Background())

	// then
	assert.Error(t, byIDsErr)
	assert.Empty(t, byIDs)
	assert.Error(t, allErr)
	assert.Empty(t, all)
	assert.Error(t, resyncErr)
	assert.Empty(t, resync)
}

func TestGuildConfigRepository_ListAll(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("FROM guilds ORDER BY").WillReturnRows(addGuildRow(newGuildRows(), "g1", nil))
	repo := NewGuildConfigRepository(mock)

	// when
	cfgs, err := repo.ListAll(context.Background())

	// then
	require.NoError(t, err)
	assert.Len(t, cfgs, 1)
}

func TestGuildConfigRepository_UpsertPresence(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("ON CONFLICT \\(guild_id\\) DO UPDATE").
		WithArgs("g1", "Celesta", "icon", "owner").
		WillReturnRows(addGuildRow(newGuildRows(), "g1", nil))
	mock.ExpectQuery("ON CONFLICT \\(guild_id\\) DO UPDATE").
		WithArgs(anyArgs(4)...).
		WillReturnError(errors.New("boom"))
	repo := NewGuildConfigRepository(mock)

	// when
	cfg, err := repo.UpsertPresence(context.Background(), "g1", "Celesta", "icon", "owner")
	failed, failedErr := repo.UpsertPresence(context.Background(), "g1", "Celesta", "icon", "owner")

	// then
	require.NoError(t, err)
	assert.True(t, cfg.PremiumForever)
	assert.Nil(t, failed)
	assert.Error(t, failedErr)
}

func TestGuildConfigRepository_MarkAbsentExcept(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectExec("SET bot_present = false").
		WithArgs([]string{"g1"}, int64(2), int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 3))
	mock.ExpectExec("SET bot_present = false").
		WithArgs([]string{}, int64(1), int64(0)).
		WillReturnError(errors.New("boom"))
	repo := NewGuildConfigRepository(mock)

	// when
	err := repo.MarkAbsentExcept(context.Background(), 1, 2, []string{"g1"})
	nilErr := repo.MarkAbsentExcept(context.Background(), 0, 1, nil)

	// then
	assert.NoError(t, err)
	assert.Error(t, nilErr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGuildConfigRepository_Setters(t *testing.T) {
	syncStartedAt := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		sql  string
		args []any
		call func(repo *GuildConfigRepository) error
	}{
		"bot present": {
			sql:  "SET bot_present = \\$1",
			args: []any{false, "g1"},
			call: func(r *GuildConfigRepository) error { return r.SetBotPresent(context.Background(), "g1", false) },
		},
		"premium": {
			sql:  "SET premium = \\$1",
			args: []any{true, "g1"},
			call: func(r *GuildConfigRepository) error { return r.SetPremium(context.Background(), "g1", true) },
		},
		"channels": {
			sql:  "SET command_channel_id",
			args: []any{"c1", "c2", "g1"},
			call: func(r *GuildConfigRepository) error { return r.SetChannels(context.Background(), "g1", "c1", "c2") },
		},
		"manage ranks": {
			sql:  "SET manage_role_ids",
			args: []any{[]string{"r1"}, "g1"},
			call: func(r *GuildConfigRepository) error {
				return r.SetRoleIDs(context.Background(), "g1", guildconfig.RoleKindManage, []string{"r1"})
			},
		},
		"view ranks": {
			sql:  "SET view_role_ids",
			args: []any{[]string{"r1"}, "g1"},
			call: func(r *GuildConfigRepository) error {
				return r.SetRoleIDs(context.Background(), "g1", guildconfig.RoleKindView, []string{"r1"})
			},
		},
		"reserve ranks": {
			sql:  "SET reserve_role_ids",
			args: []any{[]string{"r1"}, "g1"},
			call: func(r *GuildConfigRepository) error {
				return r.SetRoleIDs(context.Background(), "g1", guildconfig.RoleKindReserve, []string{"r1"})
			},
		},
		"overbook ranks nil stored as empty": {
			sql:  "SET overbook_role_ids",
			args: []any{[]string{}, "g1"},
			call: func(r *GuildConfigRepository) error {
				return r.SetRoleIDs(context.Background(), "g1", guildconfig.RoleKindOverbook, nil)
			},
		},
		"request resync": {
			sql:  "SET resync_requested_at = now\\(\\)",
			args: []any{"g1"},
			call: func(r *GuildConfigRepository) error { return r.RequestResync(context.Background(), "g1") },
		},
		"mark synced": {
			sql:  "resync_requested_at <= \\$1::timestamptz THEN NULL",
			args: []any{pgtype.Timestamptz{Time: syncStartedAt, Valid: true}, "g1"},
			call: func(r *GuildConfigRepository) error { return r.MarkSynced(context.Background(), "g1", syncStartedAt) },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			mock := newMock(t)
			mock.ExpectExec(tc.sql).WithArgs(tc.args...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			mock.ExpectExec(tc.sql).WithArgs(tc.args...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			repo := NewGuildConfigRepository(mock)

			// when
			okErr := tc.call(repo)
			missingErr := tc.call(repo)

			// then
			assert.NoError(t, okErr)
			assert.ErrorIs(t, missingErr, ports.ErrNotFound)
		})
	}
}

func TestGuildConfigRepository_SetRoleIDsUnknownKind(t *testing.T) {
	// given
	mock := newMock(t)
	repo := NewGuildConfigRepository(mock)

	// when
	err := repo.SetRoleIDs(context.Background(), "g1", guildconfig.RoleKind("admin"), []string{"r1"})

	// then
	assert.Error(t, err)
}

func TestGuildConfigRepository_ListResyncRequested(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("resync_requested_at IS NOT NULL").
		WillReturnRows(pgxmock.NewRows([]string{"guild_id"}).AddRow("g1").AddRow("g2"))
	repo := NewGuildConfigRepository(mock)

	// when
	ids, err := repo.ListResyncRequested(context.Background())

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"g1", "g2"}, ids)
}

func TestGuildChannelRepository_Replace(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM guild_channels").WithArgs("g1").WillReturnResult(pgxmock.NewResult("DELETE", 3))
	mock.ExpectExec("INSERT INTO guild_channels").
		WithArgs("g1", []string{"c1", "cat"}, []string{"letter", "Bot"}, []int32{0, 4}, []string{"cat", ""}, []int32{2, 0}).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))
	mock.ExpectCommit()
	repo := NewGuildChannelRepository(mock)

	// when
	err := repo.Replace(context.Background(), "g1", []*discord.Channel{
		{ID: "c1", Name: "letter", Type: discord.ChannelTypeGuildText, ParentID: "cat", Position: 2},
		{ID: "cat", Name: "Bot", Type: discord.ChannelTypeGuildCategory},
	})

	// then
	assert.NoError(t, err)
}

func TestGuildChannelRepository_ReplaceWithNoChannelsOnlyDeletes(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM guild_channels").WithArgs("g1").WillReturnResult(pgxmock.NewResult("DELETE", 3))
	mock.ExpectCommit()
	repo := NewGuildChannelRepository(mock)

	// when
	err := repo.Replace(context.Background(), "g1", nil)

	// then
	assert.NoError(t, err)
}

func TestGuildChannelRepository_ReplaceRollsBackOnError(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM guild_channels").WithArgs("g1").WillReturnResult(pgxmock.NewResult("DELETE", 3))
	mock.ExpectExec("INSERT INTO guild_channels").WithArgs(anyArgs(6)...).WillReturnError(errors.New("boom"))
	mock.ExpectRollback()
	repo := NewGuildChannelRepository(mock)

	// when
	err := repo.Replace(context.Background(), "g1", []*discord.Channel{{ID: "c1"}})

	// then
	assert.Error(t, err)
}

func TestGuildChannelRepository_ReplaceBeginError(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin().WillReturnError(errors.New("boom"))
	repo := NewGuildChannelRepository(mock)

	// when
	err := repo.Replace(context.Background(), "g1", nil)

	// then
	assert.Error(t, err)
}

func TestGuildChannelRepository_ListByTypes(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("type = ANY").
		WithArgs("g1", []int32{0, 5}).
		WillReturnRows(pgxmock.NewRows([]string{"channel_id", "name", "type", "parent_id", "position"}).
			AddRow("c1", "letter", int32(0), "cat", int32(2)))
	mock.ExpectQuery("FROM guild_channels").WithArgs("g2", []int32{0}).WillReturnError(errors.New("boom"))
	repo := NewGuildChannelRepository(mock)

	// when
	channels, err := repo.ListByTypes(context.Background(), "g1", []discord.ChannelType{discord.ChannelTypeGuildText, discord.ChannelTypeGuildNews})
	failed, failedErr := repo.ListByTypes(context.Background(), "g2", []discord.ChannelType{discord.ChannelTypeGuildText})

	// then
	require.NoError(t, err)
	assert.Equal(t, []*discord.Channel{{ID: "c1", Name: "letter", Type: discord.ChannelTypeGuildText, ParentID: "cat", Position: 2}}, channels)
	assert.Error(t, failedErr)
	assert.Empty(t, failed)
}

func TestGuildRoleRepository_Replace(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM guild_roles").WithArgs("g1").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec("INSERT INTO guild_roles").
		WithArgs("g1", []string{"r1"}, []string{"Postman"}, []int32{255}, []int32{3}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM guild_roles").WithArgs("g1").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectCommit()
	repo := NewGuildRoleRepository(mock)

	// when
	err := repo.Replace(context.Background(), "g1", []*role.Role{{ID: "r1", Name: "Postman", Color: 255, Position: 3}})
	emptyErr := repo.Replace(context.Background(), "g1", nil)

	// then
	assert.NoError(t, err)
	assert.NoError(t, emptyErr)
}

func TestGuildRoleRepository_ReplaceDeleteError(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM guild_roles").WithArgs("g1").WillReturnError(errors.New("boom"))
	mock.ExpectRollback()
	repo := NewGuildRoleRepository(mock)

	// when
	err := repo.Replace(context.Background(), "g1", nil)

	// then
	assert.Error(t, err)
}

func TestGuildRoleRepository_List(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("FROM guild_roles").
		WithArgs("g1").
		WillReturnRows(pgxmock.NewRows([]string{"role_id", "name", "color", "position"}).
			AddRow("r2", "Leader", int32(0), int32(5)).
			AddRow("r1", "Postman", int32(255), int32(1)))
	mock.ExpectQuery("FROM guild_roles").WithArgs("g2").WillReturnError(errors.New("boom"))
	repo := NewGuildRoleRepository(mock)

	// when
	roles, err := repo.List(context.Background(), "g1")
	failed, failedErr := repo.List(context.Background(), "g2")

	// then
	require.NoError(t, err)
	assert.Equal(t, []*role.Role{
		{ID: "r2", Name: "Leader", Position: 5},
		{ID: "r1", Name: "Postman", Color: 255, Position: 1},
	}, roles)
	assert.Error(t, failedErr)
	assert.Empty(t, failed)
}

package sqlc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/infrastructure/db/postgresql"
)

type DBTXWrapper interface {
	DBTX

	Begin(ctx context.Context) (pgx.Tx, error)
}

type GuildConfigRepository struct {
	q *Queries
}

func NewGuildConfigRepository(db DBTX) *GuildConfigRepository {
	return &GuildConfigRepository{q: New(db)}
}

func (r *GuildConfigRepository) Get(ctx context.Context, guildID string) (*guildconfig.Config, error) {
	res, err := r.q.SelectGuild(ctx, guildID)
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return mapGuild(res), nil
}

func (r *GuildConfigRepository) ListByIDs(ctx context.Context, guildIDs []string) ([]*guildconfig.Config, error) {
	if len(guildIDs) == 0 {
		return []*guildconfig.Config{}, nil
	}

	res, err := r.q.SelectGuildsByIDs(ctx, guildIDs)
	if err != nil {
		return []*guildconfig.Config{}, err
	}

	return collections.PoorMansMap(res, mapGuild), nil
}

func (r *GuildConfigRepository) ListAll(ctx context.Context) ([]*guildconfig.Config, error) {
	res, err := r.q.SelectAllGuilds(ctx)
	if err != nil {
		return []*guildconfig.Config{}, err
	}

	return collections.PoorMansMap(res, mapGuild), nil
}

func (r *GuildConfigRepository) UpsertPresence(ctx context.Context, guildID, name, icon, ownerID string) (*guildconfig.Config, error) {
	res, err := r.q.UpsertGuildPresence(ctx, UpsertGuildPresenceParams{
		GuildID: guildID,
		Name:    name,
		Icon:    icon,
		OwnerID: ownerID,
	})
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return mapGuild(res), nil
}

func (r *GuildConfigRepository) SetBotPresent(ctx context.Context, guildID string, present bool) error {
	return postgresql.RowsAffected(r.q.SetGuildBotPresent(ctx, SetGuildBotPresentParams{BotPresent: present, GuildID: guildID}))
}

func (r *GuildConfigRepository) MarkAbsentExcept(ctx context.Context, shardID, shardCount int, presentIDs []string) error {
	if presentIDs == nil {
		presentIDs = []string{}
	}
	return r.q.MarkGuildsAbsentExcept(ctx, MarkGuildsAbsentExceptParams{
		PresentIds: presentIDs,
		ShardCount: int64(shardCount),
		ShardID:    int64(shardID),
	})
}

func (r *GuildConfigRepository) SetPremium(ctx context.Context, guildID string, premium bool) error {
	return postgresql.RowsAffected(r.q.SetGuildPremium(ctx, SetGuildPremiumParams{Premium: premium, GuildID: guildID}))
}

func (r *GuildConfigRepository) SetChannels(ctx context.Context, guildID, commandChannelID, summaryChannelID string) error {
	return postgresql.RowsAffected(r.q.SetGuildChannels(ctx, SetGuildChannelsParams{
		CommandChannelID: commandChannelID,
		SummaryChannelID: summaryChannelID,
		GuildID:          guildID,
	}))
}

// SetRoleIDs switches over one query per kind, because sqlc cannot parametrize a column name.
func (r *GuildConfigRepository) SetRoleIDs(ctx context.Context, guildID string, kind guildconfig.RoleKind, roleIDs []string) error {
	if roleIDs == nil {
		roleIDs = []string{}
	}

	switch kind {
	case guildconfig.RoleKindManage:
		return postgresql.RowsAffected(r.q.SetGuildManageRoleIDs(ctx, SetGuildManageRoleIDsParams{RoleIds: roleIDs, GuildID: guildID}))
	case guildconfig.RoleKindView:
		return postgresql.RowsAffected(r.q.SetGuildViewRoleIDs(ctx, SetGuildViewRoleIDsParams{RoleIds: roleIDs, GuildID: guildID}))
	case guildconfig.RoleKindReserve:
		return postgresql.RowsAffected(r.q.SetGuildReserveRoleIDs(ctx, SetGuildReserveRoleIDsParams{RoleIds: roleIDs, GuildID: guildID}))
	case guildconfig.RoleKindOverbook:
		return postgresql.RowsAffected(r.q.SetGuildOverbookRoleIDs(ctx, SetGuildOverbookRoleIDsParams{RoleIds: roleIDs, GuildID: guildID}))
	}

	return fmt.Errorf("unknown role kind %q", kind)
}

func (r *GuildConfigRepository) RequestResync(ctx context.Context, guildID string) error {
	return postgresql.RowsAffected(r.q.RequestGuildResync(ctx, guildID))
}

func (r *GuildConfigRepository) ListResyncRequested(ctx context.Context) ([]string, error) {
	res, err := r.q.SelectResyncRequestedGuildIDs(ctx)
	if err != nil {
		return []string{}, err
	}

	return res, nil
}

func (r *GuildConfigRepository) MarkSynced(ctx context.Context, guildID string, startedAt time.Time) error {
	return postgresql.RowsAffected(r.q.MarkGuildSynced(ctx, MarkGuildSyncedParams{
		StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
		GuildID:   guildID,
	}))
}

type GuildChannelRepository struct {
	q  *Queries
	db DBTXWrapper
}

func NewGuildChannelRepository(db DBTXWrapper) *GuildChannelRepository {
	return &GuildChannelRepository{q: New(db), db: db}
}

func (r *GuildChannelRepository) Replace(ctx context.Context, guildID string, channels []*discord.Channel) error {
	params := InsertGuildChannelsParams{
		GuildID:    guildID,
		ChannelIds: make([]string, len(channels)),
		Names:      make([]string, len(channels)),
		Types:      make([]int32, len(channels)),
		ParentIds:  make([]string, len(channels)),
		Positions:  make([]int32, len(channels)),
	}
	for i, ch := range channels {
		params.ChannelIds[i] = ch.ID
		params.Names[i] = ch.Name
		params.Types[i] = int32(ch.Type)
		params.ParentIds[i] = ch.ParentID
		params.Positions[i] = int32(ch.Position)
	}

	return inTx(ctx, r.db, func(qtx *Queries) error {
		if err := qtx.DeleteGuildChannels(ctx, guildID); err != nil {
			return err
		}
		if len(channels) == 0 {
			return nil
		}
		return qtx.InsertGuildChannels(ctx, params)
	})
}

func (r *GuildChannelRepository) List(ctx context.Context, guildID string) ([]*discord.Channel, error) {
	res, err := r.q.SelectGuildChannels(ctx, guildID)
	if err != nil {
		return []*discord.Channel{}, err
	}

	return collections.PoorMansMap(res, func(row SelectGuildChannelsRow) *discord.Channel {
		return &discord.Channel{
			ID:       row.ChannelID,
			Name:     row.Name,
			Type:     discord.ChannelType(row.Type),
			ParentID: row.ParentID,
			Position: int(row.Position),
		}
	}), nil
}

type GuildRoleRepository struct {
	q  *Queries
	db DBTXWrapper
}

func NewGuildRoleRepository(db DBTXWrapper) *GuildRoleRepository {
	return &GuildRoleRepository{q: New(db), db: db}
}

func (r *GuildRoleRepository) Replace(ctx context.Context, guildID string, roles []*role.Role) error {
	params := InsertGuildRolesParams{
		GuildID:   guildID,
		RoleIds:   make([]string, len(roles)),
		Names:     make([]string, len(roles)),
		Colors:    make([]int32, len(roles)),
		Positions: make([]int32, len(roles)),
	}
	for i, rl := range roles {
		params.RoleIds[i] = rl.ID
		params.Names[i] = rl.Name
		params.Colors[i] = int32(rl.Color)
		params.Positions[i] = int32(rl.Position)
	}

	return inTx(ctx, r.db, func(qtx *Queries) error {
		if err := qtx.DeleteGuildRoles(ctx, guildID); err != nil {
			return err
		}
		if len(roles) == 0 {
			return nil
		}
		return qtx.InsertGuildRoles(ctx, params)
	})
}

func (r *GuildRoleRepository) List(ctx context.Context, guildID string) ([]*role.Role, error) {
	res, err := r.q.SelectGuildRoles(ctx, guildID)
	if err != nil {
		return []*role.Role{}, err
	}

	return collections.PoorMansMap(res, func(row SelectGuildRolesRow) *role.Role {
		return &role.Role{
			ID:       row.RoleID,
			Name:     row.Name,
			Color:    int(row.Color),
			Position: int(row.Position),
		}
	}), nil
}

func inTx(ctx context.Context, db DBTXWrapper, fn func(qtx *Queries) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer errors.ExecuteAndIgnoreErrorF(tx.Rollback, ctx)

	if err := fn(New(tx)); err != nil {
		return postgresql.MapError(err)
	}

	return tx.Commit(ctx)
}

func mapGuild(g Guild) *guildconfig.Config {
	return &guildconfig.Config{
		GuildID:           g.GuildID,
		Name:              g.Name,
		Icon:              g.Icon,
		OwnerID:           g.OwnerID,
		BotPresent:        g.BotPresent,
		Premium:           g.Premium,
		PremiumForever:    g.PremiumForever,
		CommandChannelID:  g.CommandChannelID,
		SummaryChannelID:  g.SummaryChannelID,
		ManageRoleIDs:     nonNil(g.ManageRoleIds),
		ViewRoleIDs:       nonNil(g.ViewRoleIds),
		ReserveRoleIDs:    nonNil(g.ReserveRoleIds),
		OverbookRoleIDs:   nonNil(g.OverbookRoleIds),
		ResyncRequestedAt: timePtr(g.ResyncRequestedAt),
		SyncedAt:          timePtr(g.SyncedAt),
		CreatedAt:         g.CreatedAt.Time,
		UpdatedAt:         g.UpdatedAt.Time,
	}
}

func nonNil(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

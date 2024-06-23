package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/guild"
)

type DBTXWrapper interface {
	DBTX

	Begin(ctx context.Context) (pgx.Tx, error)
}

type GuildRepository struct {
	q   *Queries
	db  DBTXWrapper
	log *zap.SugaredLogger
}

func NewGuildRepository(db DBTXWrapper) *GuildRepository {
	return &GuildRepository{
		q:  New(db),
		db: db,
	}
}

func (t *GuildRepository) WithLogger(log *zap.SugaredLogger) *GuildRepository {
	t.log = log.With(
		"layer", "infrastructure",
		"name", "GuildRepository")

	return t
}

func (t *GuildRepository) CreateGuild(ctx context.Context, guildID string, guildName string) (*guild.Guild, error) {
	params := CreateGuildParams{
		GuildID:   guildID,
		GuildName: guildName,
	}

	g, err := t.q.CreateGuild(ctx, params)
	if err != nil {
		return nil, err
	}

	return &guild.Guild{
		InternalID: g.ID,
		ID:         g.GuildID,
		Name:       g.GuildName,
	}, nil
}

func (t *GuildRepository) SelectGuilds(ctx context.Context) ([]*guild.Guild, error) {
	t.log.Info("SelectGuilds1")
	rows, err := t.q.SelectGuilds(ctx)
	if err != nil {
		return nil, err
	}
	t.log.With("rows_count", len(rows)).Info("SelectGuilds2")

	guilds := make([]*guild.Guild, len(rows))
	for index, row := range rows {
		guilds[index] = &guild.Guild{
			InternalID: row.Guild.ID,
			ID:         row.Guild.GuildID,
			Name:       row.Guild.GuildName,
		}
	}

	return guilds, nil
}

package sqlc

import (
	"context"
	"spot-assistant/internal/core/dto/guildsworld"
)

type WorldNameRepository struct {
	q *Queries
}

func NewWorldNameRepository(db DBTX) *WorldNameRepository {
	return &WorldNameRepository{
		q: New(db),
	}
}

func (repo *WorldNameRepository) UpsertGuildWorld(ctx context.Context, guildID string, worldName string) error {
	return repo.q.UpsertGuildWorld(ctx, UpsertGuildWorldParams{
		GuildID:   guildID,
		WorldName: worldName,
	})
}

func (repo *WorldNameRepository) SelectGuildWorld(ctx context.Context, guildID string) (*guildsworld.GuildsWorld, error) {
	res, err := repo.q.SelectGuildWorld(ctx, guildID)
	if err != nil {
		return nil, err
	}
	return &guildsworld.GuildsWorld{
		ID:        res.ID,
		GuildID:   res.GuildID,
		WorldName: res.WorldName,
		CreatedAt: res.CreatedAt.Time,
	}, nil
}

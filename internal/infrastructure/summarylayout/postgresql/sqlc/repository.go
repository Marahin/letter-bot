package sqlc

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/core/dto/summarylayout"
)

// SummaryLayoutRepository implements ports.SummaryLayoutRepository.
type SummaryLayoutRepository struct {
	q *Queries
}

func NewSummaryLayoutRepository(db DBTX) *SummaryLayoutRepository {
	return &SummaryLayoutRepository{q: New(db)}
}

func (r *SummaryLayoutRepository) GetLayout(ctx context.Context, guildID string) (*summarylayout.Layout, error) {
	row, err := r.q.SelectSummaryLayout(ctx, guildID)
	if errors.Is(err, pgx.ErrNoRows) {
		return &summarylayout.Layout{GuildID: guildID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &summarylayout.Layout{
		GuildID:        row.GuildID,
		CategoryID:     row.CategoryID,
		IndexChannelID: row.IndexChannelID,
		IndexMessageID: row.IndexMessageID,
		IndexHash:      row.IndexHash,
		BlockedUntil:   timePtr(row.BlockedUntil),
	}, nil
}

func (r *SummaryLayoutRepository) SaveLayout(ctx context.Context, l summarylayout.Layout) error {
	return r.q.UpsertSummaryLayout(ctx, UpsertSummaryLayoutParams{
		GuildID:        l.GuildID,
		CategoryID:     l.CategoryID,
		IndexChannelID: l.IndexChannelID,
		IndexMessageID: l.IndexMessageID,
		IndexHash:      l.IndexHash,
		BlockedUntil:   timestamptz(l.BlockedUntil),
	})
}

func (r *SummaryLayoutRepository) ListRegionChannels(ctx context.Context, guildID string) ([]summarylayout.RegionChannel, error) {
	rows, err := r.q.SelectRegionChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := make([]summarylayout.RegionChannel, 0, len(rows))
	for _, row := range rows {
		out = append(out, summarylayout.RegionChannel{
			GuildID:     row.GuildID,
			Region:      row.Region,
			ChannelID:   row.ChannelID,
			MessageIDs:  nonNil(row.MessageIds),
			ContentHash: row.ContentHash,
			RetiredAt:   timePtr(row.RetiredAt),
		})
	}
	return out, nil
}

func (r *SummaryLayoutRepository) SaveRegionChannel(ctx context.Context, c summarylayout.RegionChannel) error {
	return r.q.UpsertRegionChannel(ctx, UpsertRegionChannelParams{
		GuildID:     c.GuildID,
		Region:      c.Region,
		ChannelID:   c.ChannelID,
		MessageIds:  nonNil(c.MessageIDs),
		ContentHash: c.ContentHash,
		RetiredAt:   timestamptz(c.RetiredAt),
	})
}

func (r *SummaryLayoutRepository) DeleteRegionChannel(ctx context.Context, guildID, region string) error {
	return r.q.DeleteRegionChannel(ctx, DeleteRegionChannelParams{GuildID: guildID, Region: region})
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

func timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

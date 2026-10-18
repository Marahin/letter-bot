package sqlc

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/ports"
)

// SpotQueueRepository implements ports.SpotQueueRepository.
type SpotQueueRepository struct {
	q *Queries
}

func NewSpotQueueRepository(db DBTX) *SpotQueueRepository {
	return &SpotQueueRepository{q: New(db)}
}

func (r *SpotQueueRepository) UpsertEntry(ctx context.Context, e queue.Entry) (*queue.Entry, error) {
	row, err := r.q.UpsertQueueEntry(ctx, UpsertQueueEntryParams{
		GuildID:       e.GuildID,
		SpotID:        e.SpotID,
		UserID:        e.UserID,
		LengthMinutes: int32(e.Length / time.Minute),
		ExpiresAt:     timestamptz(e.ExpiresAt),
	})
	if err != nil {
		return nil, err
	}
	out := entry(row, e.SpotName)
	return &out, nil
}

func (r *SpotQueueRepository) CountUserEntries(ctx context.Context, c queue.UserCount) (int, error) {
	n, err := r.q.CountUserQueueEntries(ctx, CountUserQueueEntriesParams{
		GuildID:      c.GuildID,
		UserID:       c.UserID,
		ExceptSpotID: c.ExceptSpotID,
		Now:          timestamptz(c.Now),
	})
	return int(n), err
}

func (r *SpotQueueRepository) GetEntry(ctx context.Context, id int64) (*queue.Entry, error) {
	row, err := r.q.SelectQueueEntry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := entry(row.SpotQueueEntry, row.SpotName)
	return &out, nil
}

func (r *SpotQueueRepository) ListUserEntries(ctx context.Context, guildID, userID string, now time.Time) ([]queue.Entry, error) {
	rows, err := r.q.SelectUserQueueEntries(ctx, SelectUserQueueEntriesParams{GuildID: guildID, UserID: userID, Now: timestamptz(now)})
	if err != nil {
		return nil, err
	}
	out := make([]queue.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entry(row.SpotQueueEntry, row.SpotName))
	}
	return out, nil
}

func (r *SpotQueueRepository) DeleteEntry(ctx context.Context, id int64, userID string) (bool, error) {
	n, err := r.q.DeleteQueueEntry(ctx, DeleteQueueEntryParams{ID: id, UserID: userID})
	return n > 0, err
}

func (r *SpotQueueRepository) DeleteUserSpotEntry(ctx context.Context, guildID, userID string, spotID int64) (bool, error) {
	n, err := r.q.DeleteUserSpotQueueEntry(ctx, DeleteUserSpotQueueEntryParams{GuildID: guildID, UserID: userID, SpotID: spotID})
	return n > 0, err
}

func (r *SpotQueueRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	return r.q.DeleteExpiredQueueEntries(ctx, timestamptz(now))
}

func (r *SpotQueueRepository) SelectDueEntries(ctx context.Context, q queue.DueQuery) ([]queue.Entry, error) {
	rows, err := r.q.SelectDueQueueEntries(ctx, SelectDueQueueEntriesParams{
		GuildID: q.GuildID,
		Now:     timestamptz(q.Now),
		StartAt: timestamptz(q.StartAt),
	})
	if err != nil {
		return nil, err
	}
	out := make([]queue.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entry(row.SpotQueueEntry, row.SpotName))
	}
	return out, nil
}

func (r *SpotQueueRepository) ClaimOffer(ctx context.Context, c queue.Claim) (bool, error) {
	n, err := r.q.ClaimQueueOffer(ctx, ClaimQueueOfferParams{
		OfferedAt: timestamptz(c.OfferedAt),
		ID:        c.EntryID,
		StartAt:   timestamptz(c.StartAt),
	})
	return n > 0, err
}

func (r *SpotQueueRepository) ResetTakenOffers(ctx context.Context, guildID string, startAt time.Time) error {
	_, err := r.q.ResetTakenQueueOffers(ctx, ResetTakenQueueOffersParams{GuildID: guildID, StartAt: timestamptz(startAt)})
	return err
}

func (r *SpotQueueRepository) ResetOffer(ctx context.Context, id int64) error {
	return r.q.ResetQueueOffer(ctx, id)
}

func entry(row SpotQueueEntry, spotName string) queue.Entry {
	e := queue.Entry{
		ID:        row.ID,
		GuildID:   row.GuildID,
		SpotID:    row.SpotID,
		SpotName:  spotName,
		UserID:    row.UserID,
		Length:    time.Duration(row.LengthMinutes) * time.Minute,
		CreatedAt: row.CreatedAt.Time,
		ExpiresAt: row.ExpiresAt.Time,
	}
	if row.OfferedAt.Valid {
		at := row.OfferedAt.Time
		e.OfferedAt = &at
	}
	return e
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

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
	out := entry(row.ID, row.GuildID, row.SpotID, row.UserID, "", row.LengthMinutes, row.CreatedAt, row.ExpiresAt, row.OfferedAt)
	out.SpotName = e.SpotName
	return &out, nil
}

func (r *SpotQueueRepository) CountUserEntries(ctx context.Context, guildID, userID string, exceptSpotID int64, now time.Time) (int, error) {
	n, err := r.q.CountUserQueueEntries(ctx, CountUserQueueEntriesParams{
		GuildID:      guildID,
		UserID:       userID,
		ExceptSpotID: exceptSpotID,
		Now:          timestamptz(now),
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
	out := entry(row.ID, row.GuildID, row.SpotID, row.UserID, row.SpotName, row.LengthMinutes, row.CreatedAt, row.ExpiresAt, row.OfferedAt)
	return &out, nil
}

func (r *SpotQueueRepository) ListUserEntries(ctx context.Context, guildID, userID string, now time.Time) ([]queue.Entry, error) {
	rows, err := r.q.SelectUserQueueEntries(ctx, SelectUserQueueEntriesParams{GuildID: guildID, UserID: userID, Now: timestamptz(now)})
	if err != nil {
		return nil, err
	}
	out := make([]queue.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entry(row.ID, row.GuildID, row.SpotID, row.UserID, row.SpotName, row.LengthMinutes, row.CreatedAt, row.ExpiresAt, row.OfferedAt))
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

func (r *SpotQueueRepository) DeleteExpired(ctx context.Context, guildID string, now time.Time) (int64, error) {
	return r.q.DeleteExpiredQueueEntries(ctx, DeleteExpiredQueueEntriesParams{GuildID: guildID, Now: timestamptz(now)})
}

func (r *SpotQueueRepository) SelectGuildCandidates(ctx context.Context, guildID string, now, startAt time.Time) ([]queue.Candidate, error) {
	rows, err := r.q.SelectGuildQueueCandidates(ctx, SelectGuildQueueCandidatesParams{
		StartAt: timestamptz(startAt),
		GuildID: guildID,
		Now:     timestamptz(now),
	})
	if err != nil {
		return nil, err
	}
	out := make([]queue.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, queue.Candidate{
			Entry: entry(row.ID, row.GuildID, row.SpotID, row.UserID, row.SpotName, row.LengthMinutes, row.CreatedAt, row.ExpiresAt, row.OfferedAt),
			Free:  row.Free,
		})
	}
	return out, nil
}

func (r *SpotQueueRepository) ClaimOffer(ctx context.Context, id int64, at time.Time) (bool, error) {
	n, err := r.q.ClaimQueueOffer(ctx, ClaimQueueOfferParams{OfferedAt: timestamptz(at), ID: id})
	return n > 0, err
}

func (r *SpotQueueRepository) ResetOffers(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.q.ResetQueueOffers(ctx, ids)
}

func entry(id int64, guildID string, spotID int64, userID, spotName string, lengthMinutes int32, createdAt, expiresAt, offeredAt pgtype.Timestamptz) queue.Entry {
	e := queue.Entry{
		ID:        id,
		GuildID:   guildID,
		SpotID:    spotID,
		SpotName:  spotName,
		UserID:    userID,
		Length:    time.Duration(lengthMinutes) * time.Minute,
		CreatedAt: createdAt.Time,
		ExpiresAt: expiresAt.Time,
	}
	if offeredAt.Valid {
		at := offeredAt.Time
		e.OfferedAt = &at
	}
	return e
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

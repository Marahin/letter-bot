package sqlc

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	commonerrors "spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/loot"
	"spot-assistant/internal/infrastructure/db/postgresql"
)

type DBTXWrapper interface {
	DBTX

	Begin(ctx context.Context) (pgx.Tx, error)
}

// LootRepository implements ports.LootSessionRepository.
type LootRepository struct {
	q  *Queries
	db DBTXWrapper
}

func NewLootRepository(db DBTXWrapper) *LootRepository {
	return &LootRepository{q: New(db), db: db}
}

func (r *LootRepository) SaveSession(ctx context.Context, s loot.Session) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer commonerrors.ExecuteAndIgnoreErrorF(ctx, tx.Rollback)
	qtx := New(tx)

	id, err := qtx.InsertSession(ctx, InsertSessionParams{
		ContentHash:     s.Hash,
		StartedAt:       ts(s.StartedAt),
		EndedAt:         ts(s.EndedAt),
		DurationSeconds: int32(s.Duration / time.Second),
		Members:         int32(s.PartySize),
		Loot:            s.Loot,
		Supplies:        s.Supplies,
		Balance:         s.Balance,
		Damage:          s.Damage,
		Healing:         s.Healing,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Stored before: the conflict returns no row.
		id, err = qtx.SessionIDByHash(ctx, s.Hash)
		if err != nil {
			return 0, postgresql.MapError(err)
		}
		return id, tx.Commit(ctx)
	}
	if err != nil {
		return 0, postgresql.MapError(err)
	}

	p := InsertMembersParams{SessionID: id}
	for _, m := range s.Members {
		p.Keys = append(p.Keys, m.Key)
		p.Names = append(p.Names, m.Name)
		p.Positions = append(p.Positions, int32(m.Position))
		p.Leaders = append(p.Leaders, m.Leader)
		p.Loots = append(p.Loots, m.Loot)
		p.Supplies = append(p.Supplies, m.Supplies)
		p.Balances = append(p.Balances, m.Balance)
		p.Damages = append(p.Damages, m.Damage)
		p.Healings = append(p.Healings, m.Healing)
	}
	if err := qtx.InsertMembers(ctx, p); err != nil {
		return 0, postgresql.MapError(err)
	}
	return id, tx.Commit(ctx)
}

func (r *LootRepository) MatchCandidates(ctx context.Context, from, to time.Time, keys []string, maxLen time.Duration) ([]loot.Candidate, error) {
	rows, err := r.q.MatchCandidates(ctx, MatchCandidatesParams{StartFrom: ts(from.Add(-maxLen)), ToT: ts(to), FromT: ts(from), Keys: keys})
	if err != nil {
		return nil, err
	}
	out := make([]loot.Candidate, len(rows))
	for i, row := range rows {
		out[i] = loot.Candidate{
			ReservationID: row.ID,
			GuildID:       row.GuildID,
			GuildName:     row.GuildName,
			SpotID:        row.SpotID,
			SpotName:      row.SpotName,
			Author:        row.Author,
			StartAt:       row.StartAt.Time,
			EndAt:         row.EndAt.Time,
		}
	}
	return out, nil
}

// Pick takes the session off its other reservation first (the unique session_id would refuse
// the insert otherwise), and rolls that back when the reservation keeps its own session.
func (r *LootRepository) Pick(ctx context.Context, sessionID, reservationID, overlapSeconds int64) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer commonerrors.ExecuteAndIgnoreErrorF(ctx, tx.Rollback)
	qtx := New(tx)

	if err := qtx.DeleteOtherPicks(ctx, DeleteOtherPicksParams{SessionID: sessionID, ReservationID: reservationID}); err != nil {
		return false, postgresql.MapError(err)
	}
	_, err = qtx.PickSession(ctx, PickSessionParams{ReservationID: reservationID, SessionID: sessionID, OverlapSeconds: int32(overlapSeconds)})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, postgresql.MapError(err)
	}
	return true, tx.Commit(ctx)
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

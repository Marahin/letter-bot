package sqlc

import (
	"context"
	"errors"
	"fmt"
	"math"
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

var errOutOfRange = errors.New("out of the int32 range")

func (r *LootRepository) SaveSession(ctx context.Context, s loot.Session) (int64, error) {
	duration, err := toInt32(int64(s.Duration / time.Second))
	if err != nil {
		return 0, fmt.Errorf("duration: %w", err)
	}
	partySize, err := toInt32(int64(s.PartySize))
	if err != nil {
		return 0, fmt.Errorf("party size: %w", err)
	}
	p := InsertMembersParams{}
	for _, m := range s.Members {
		position, err := toInt32(int64(m.Position))
		if err != nil {
			return 0, fmt.Errorf("position: %w", err)
		}
		p.Keys = append(p.Keys, m.Key)
		p.Names = append(p.Names, m.Name)
		p.Positions = append(p.Positions, position)
		p.Leaders = append(p.Leaders, m.Leader)
		p.Loots = append(p.Loots, m.Loot)
		p.Supplies = append(p.Supplies, m.Supplies)
		p.Balances = append(p.Balances, m.Balance)
		p.Damages = append(p.Damages, m.Damage)
		p.Healings = append(p.Healings, m.Healing)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer commonerrors.ExecuteAndIgnoreErrorF(ctx, tx.Rollback)
	qtx := New(tx)

	id, err := qtx.InsertSession(ctx, InsertSessionParams{
		ContentHash:     s.Hash,
		SubmittedBy:     s.SubmittedBy,
		StartedAt:       ts(s.StartedAt),
		EndedAt:         ts(s.EndedAt),
		DurationSeconds: duration,
		Members:         partySize,
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

	p.SessionID = id
	if err := qtx.InsertMembers(ctx, p); err != nil {
		return 0, postgresql.MapError(err)
	}
	return id, tx.Commit(ctx)
}

func (r *LootRepository) MatchCandidates(ctx context.Context, from, to time.Time, keys []string, maxLen time.Duration) ([]loot.Candidate, error) {
	rows, err := r.q.MatchCandidates(ctx, MatchCandidatesParams{StartFrom: ts(from.Add(-maxLen)), ToT: ts(to), FromT: ts(from), Keys: keys})
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	out := make([]loot.Candidate, len(rows))
	for i, row := range rows {
		out[i] = loot.Candidate{
			ReservationID:   row.ID,
			GuildID:         row.GuildID,
			GuildName:       row.GuildName,
			SpotID:          row.SpotID,
			SpotName:        row.SpotName,
			Author:          row.Author,
			AuthorDiscordID: row.AuthorDiscordID,
			StartAt:         row.StartAt.Time,
			EndAt:           row.EndAt.Time,
		}
	}
	return out, nil
}

// Pick takes the session off its other reservation first (the unique session_id would refuse
// the insert otherwise), and rolls that back when the reservation keeps its own session.
func (r *LootRepository) Pick(ctx context.Context, a loot.Attribution) (bool, error) {
	overlap, err := toInt32(a.OverlapSeconds)
	if err != nil {
		return false, fmt.Errorf("overlap: %w", err)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer commonerrors.ExecuteAndIgnoreErrorF(ctx, tx.Rollback)
	qtx := New(tx)

	if err := qtx.DeleteOtherPicks(ctx, DeleteOtherPicksParams{SessionID: a.SessionID, ReservationID: a.ReservationID}); err != nil {
		return false, postgresql.MapError(err)
	}
	_, err = qtx.PickSession(ctx, PickSessionParams{ReservationID: a.ReservationID, SessionID: a.SessionID, OverlapSeconds: overlap, ByAuthor: a.ByAuthor})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, postgresql.MapError(err)
	}
	return true, tx.Commit(ctx)
}

func toInt32(v int64) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, errOutOfRange
	}
	return int32(v), nil
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

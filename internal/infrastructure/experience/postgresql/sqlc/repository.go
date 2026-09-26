package sqlc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/ports"
)

type DBTXWrapper interface {
	DBTX

	Begin(ctx context.Context) (pgx.Tx, error)
}

// ExperienceRepository implements ports.ExperienceRepository.
type ExperienceRepository struct {
	q  *Queries
	db DBTXWrapper
}

func NewExperienceRepository(db DBTXWrapper) *ExperienceRepository {
	return &ExperienceRepository{q: New(db), db: db}
}

func (r *ExperienceRepository) ListTrackedWorlds(ctx context.Context) ([]string, error) {
	return r.q.ListTrackedWorlds(ctx)
}

// trackedLookback bounds start_at so the (guild_id, start_at) index serves the scan; reservations last hours, not days.
const trackedLookback = 24 * time.Hour

func (r *ExperienceRepository) ListTrackedCharacterKeys(ctx context.Context, world string, since time.Time) ([]string, error) {
	return r.q.ListTrackedCharacterKeys(ctx, ListTrackedCharacterKeysParams{
		World:     world,
		StartFrom: ts(since.Add(-trackedLookback)),
		Since:     ts(since),
	})
}

func (r *ExperienceRepository) LatestSnapshots(ctx context.Context, world string, keys []string) (map[string]experience.Snapshot, error) {
	out := map[string]experience.Snapshot{}
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := r.q.LatestSnapshots(ctx, LatestSnapshotsParams{World: world, Keys: keys})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.CharacterKey] = mapSnapshot(row)
	}
	return out, nil
}

func (r *ExperienceRepository) SaveRun(ctx context.Context, result experience.RunResult) error {
	run := result.Run
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer errors.ExecuteAndIgnoreErrorF(tx.Rollback, ctx)
	qtx := New(tx)

	if len(result.Inserted) > 0 {
		params := InsertSnapshotsParams{
			World:          run.World,
			CharacterKeys:  make([]string, len(result.Inserted)),
			CharacterNames: make([]string, len(result.Inserted)),
			Levels:         make([]int32, len(result.Inserted)),
			Experiences:    make([]int64, len(result.Inserted)),
			Vocations:      make([]string, len(result.Inserted)),
			ObservedAt:     ts(run.ObservedAt),
		}
		for i, s := range result.Inserted {
			params.CharacterKeys[i] = s.CharacterKey
			params.CharacterNames[i] = s.CharacterName
			params.Levels[i] = int32(s.Level)
			params.Experiences[i] = s.Experience
			params.Vocations[i] = s.Vocation
		}
		if err := qtx.InsertSnapshots(ctx, params); err != nil {
			return postgresql.MapError(err)
		}
	}
	if len(result.SeenIDs) > 0 {
		if err := qtx.TouchSnapshots(ctx, TouchSnapshotsParams{SeenAt: ts(run.ObservedAt), Ids: result.SeenIDs}); err != nil {
			return postgresql.MapError(err)
		}
	}
	if err := qtx.InsertRun(ctx, InsertRunParams{
		World:      run.World,
		ObservedAt: ts(run.ObservedAt),
		FetchedAt:  ts(run.FetchedAt),
		Pages:      int32(run.Pages),
		Rows:       int32(run.Rows),
	}); err != nil {
		return postgresql.MapError(err)
	}

	return tx.Commit(ctx)
}

func (r *ExperienceRepository) FirstRunObservedAt(ctx context.Context, world string) (time.Time, error) {
	res, err := r.q.FirstRunObservedAt(ctx, world)
	if err != nil {
		return time.Time{}, postgresql.MapError(err)
	}
	if !res.Valid {
		return time.Time{}, ports.ErrNotFound
	}
	return res.Time, nil
}

func (r *ExperienceRepository) FirstRunObservedAtOrAfter(ctx context.Context, world string, t time.Time) (time.Time, error) {
	res, err := r.q.FirstRunObservedAtOrAfter(ctx, FirstRunObservedAtOrAfterParams{World: world, T: ts(t)})
	if err != nil {
		return time.Time{}, postgresql.MapError(err)
	}
	return res.Time, nil
}

func (r *ExperienceRepository) SnapshotAtOrBefore(ctx context.Context, world, key string, t time.Time) (*experience.Snapshot, error) {
	res, err := r.q.SnapshotAtOrBefore(ctx, SnapshotAtOrBeforeParams{World: world, CharacterKey: key, T: ts(t)})
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	s := mapSnapshot(res)
	return &s, nil
}

func (r *ExperienceRepository) FirstSnapshotAfter(ctx context.Context, world, key string, from, to time.Time) (*experience.Snapshot, error) {
	res, err := r.q.FirstSnapshotAfter(ctx, FirstSnapshotAfterParams{World: world, CharacterKey: key, FromT: ts(from), ToT: ts(to)})
	if err != nil {
		return nil, postgresql.MapError(err)
	}
	s := mapSnapshot(res)
	return &s, nil
}

func (r *ExperienceRepository) PendingReservations(ctx context.Context, world string, startFrom, endFrom, endTo time.Time) ([]experience.PendingReservation, error) {
	rows, err := r.q.PendingReservations(ctx, PendingReservationsParams{
		World:     world,
		StartFrom: ts(startFrom),
		EndFrom:   ts(endFrom),
		EndTo:     ts(endTo),
	})
	if err != nil {
		return nil, err
	}
	return collections.PoorMansMap(rows, func(row PendingReservationsRow) experience.PendingReservation {
		return experience.PendingReservation{ID: row.ID, Author: row.Author, StartAt: row.StartAt.Time, EndAt: row.EndAt.Time}
	}), nil
}

func (r *ExperienceRepository) InsertReservationExperience(ctx context.Context, rows []experience.ReservationExperience) error {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	params := InsertReservationExperienceParams{
		ReservationIds:   make([]int64, n),
		CharacterKeys:    make([]string, n),
		CharacterNames:   make([]string, n),
		StartExperiences: make([]int64, n),
		HasStarts:        make([]bool, n),
		EndExperiences:   make([]int64, n),
		HasEnds:          make([]bool, n),
		Gains:            make([]int64, n),
		Statuses:         make([]string, n),
	}
	for i, row := range rows {
		params.ReservationIds[i] = row.ReservationID
		params.CharacterKeys[i] = row.CharacterKey
		params.CharacterNames[i] = row.CharacterName
		params.StartExperiences[i], params.HasStarts[i] = deref(row.StartExperience)
		params.EndExperiences[i], params.HasEnds[i] = deref(row.EndExperience)
		params.Gains[i], _ = deref(row.Gain)
		params.Statuses[i] = string(row.Status)
	}
	return postgresql.MapError(r.q.InsertReservationExperience(ctx, params))
}

func (r *ExperienceRepository) SnapshotHistory(ctx context.Context, world, key string, from, to time.Time) ([]experience.Snapshot, error) {
	rows, err := r.q.SnapshotHistory(ctx, SnapshotHistoryParams{World: world, CharacterKey: key, FromT: ts(from), ToT: ts(to)})
	if err != nil {
		return nil, err
	}
	return collections.PoorMansMap(rows, mapSnapshot), nil
}

func mapSnapshot(row HighscoreSnapshot) experience.Snapshot {
	return experience.Snapshot{
		ID:            row.ID,
		World:         row.World,
		CharacterKey:  row.CharacterKey,
		CharacterName: row.CharacterName,
		Level:         int(row.Level),
		Experience:    row.Experience,
		Vocation:      row.Vocation,
		ObservedAt:    row.ObservedAt.Time,
		LastSeenAt:    row.LastSeenAt.Time,
	}
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func deref(v *int64) (int64, bool) {
	if v == nil {
		return 0, false
	}
	return *v, true
}

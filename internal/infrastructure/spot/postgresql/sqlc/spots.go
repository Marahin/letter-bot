package sqlc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/infrastructure/db/postgresql"
)

type SpotRepository struct {
	q *Queries
}

func NewSpotRepository(db DBTX) *SpotRepository {
	return &SpotRepository{
		q: New(db),
	}
}

func (repo *SpotRepository) SelectGuildSpots(ctx context.Context, guildID string, includeArchived bool) ([]*spot.Spot, error) {
	res, err := repo.q.SelectGuildSpots(ctx, SelectGuildSpotsParams{GuildID: guildID, IncludeArchived: includeArchived})
	if err != nil {
		return []*spot.Spot{}, err
	}

	return collections.PoorMansMap(res, mapWebSpot), nil
}

func (repo *SpotRepository) SelectGuildSpotByName(ctx context.Context, guildID string, name string) (*spot.Spot, error) {
	res, err := repo.q.SelectGuildSpotByName(ctx, SelectGuildSpotByNameParams{GuildID: guildID, Name: name})
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return mapWebSpot(res), nil
}

func (repo *SpotRepository) SelectGuildSpotsLike(ctx context.Context, guildID string, namePattern string) ([]*spot.Spot, error) {
	res, err := repo.q.SelectGuildSpotsLike(ctx, SelectGuildSpotsLikeParams{GuildID: guildID, NamePattern: postgresql.EscapeLike(namePattern)})
	if err != nil {
		return []*spot.Spot{}, err
	}

	return collections.PoorMansMap(res, mapWebSpot), nil
}

func (repo *SpotRepository) SelectGuildSpotByID(ctx context.Context, guildID string, id int64) (*spot.Spot, error) {
	res, err := repo.q.SelectGuildSpotByID(ctx, SelectGuildSpotByIDParams{ID: id, GuildID: guildID})
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return mapWebSpot(res), nil
}

func (repo *SpotRepository) InsertSpot(ctx context.Context, guildID string, name string) (*spot.Spot, error) {
	res, err := repo.q.InsertSpot(ctx, InsertSpotParams{Name: name, GuildID: guildID})
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return mapWebSpot(res), nil
}

func (repo *SpotRepository) RenameSpot(ctx context.Context, guildID string, id int64, name string) error {
	return postgresql.RowsAffected(repo.q.RenameSpot(ctx, RenameSpotParams{Name: name, ID: id, GuildID: guildID}))
}

func (repo *SpotRepository) ArchiveSpot(ctx context.Context, guildID string, id int64) error {
	return postgresql.RowsAffected(repo.q.ArchiveSpot(ctx, ArchiveSpotParams{ID: id, GuildID: guildID}))
}

func (repo *SpotRepository) RestoreSpot(ctx context.Context, guildID string, id int64) error {
	return postgresql.RowsAffected(repo.q.RestoreSpot(ctx, RestoreSpotParams{ID: id, GuildID: guildID}))
}

func (repo *SpotRepository) DeleteSpot(ctx context.Context, guildID string, id int64) error {
	return postgresql.RowsAffected(repo.q.DeleteSpot(ctx, DeleteSpotParams{ID: id, GuildID: guildID}))
}

func (repo *SpotRepository) CountSpotReservations(ctx context.Context, guildID string, id int64) (int64, error) {
	return repo.q.CountSpotReservations(ctx, CountSpotReservationsParams{SpotID: id, GuildID: guildID})
}

func (repo *SpotRepository) SelectGuildSpotReservationCounts(ctx context.Context, guildID string) (map[int64]spot.ReservationCounts, error) {
	rows, err := repo.q.SelectGuildSpotReservationCounts(ctx, guildID)
	if err != nil {
		return nil, err
	}

	counts := make(map[int64]spot.ReservationCounts, len(rows))
	for _, r := range rows {
		counts[r.SpotID] = spot.ReservationCounts{Total: r.Total, Upcoming: r.Upcoming}
	}
	return counts, nil
}

func (repo *SpotRepository) InsertSpotsIgnoreDuplicates(ctx context.Context, guildID string, names []string) (int64, error) {
	if len(names) == 0 {
		return 0, nil
	}

	return repo.q.InsertSpotsIgnoreDuplicates(ctx, InsertSpotsIgnoreDuplicatesParams{GuildID: guildID, Names: names})
}

func mapWebSpot(s WebSpot) *spot.Spot {
	return &spot.Spot{
		ID:         s.ID,
		Name:       s.Name,
		CreatedAt:  s.CreatedAt.Time,
		GuildID:    s.GuildID.String,
		ArchivedAt: timePtr(s.ArchivedAt),
	}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

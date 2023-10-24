package sqlc

import (
	"context"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/spot"
)

type SpotRepository struct {
	q *Queries
}

func NewSpotRepository(db DBTX) *SpotRepository {
	return &SpotRepository{
		q: New(db),
	}
}

func (repo *SpotRepository) SelectAllSpots(ctx context.Context) ([]*spot.Spot, error) {
	res, err := repo.q.SelectAllSpots(ctx)
	if err != nil {
		return []*spot.Spot{}, err
	}

	return collections.PoorMansMap(res, func(s WebSpot) *spot.Spot {
		return &spot.Spot{
			ID:        s.ID,
			Name:      s.Name,
			CreatedAt: s.CreatedAt.Time,
		}
	}), nil
}

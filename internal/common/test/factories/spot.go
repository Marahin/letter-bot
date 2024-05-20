package factories

import (
	"time"

	"spot-assistant/internal/core/dto/spot"
)

// CreateSpot creates a sample spot.
func CreateSpot() *spot.Spot {
	return &spot.Spot{
		ID:        0,
		Name:      "sample-spot-name",
		CreatedAt: time.Now(),
	}
}

package reservationforms

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

// AmbiguousSpotError means the typed name is part of more than one respawn name.
type AmbiguousSpotError struct {
	Query      string
	Candidates []*spot.Spot
}

func (e *AmbiguousSpotError) Error() string {
	return fmt.Sprintf("%d respawns match %q", len(e.Candidates), e.Query)
}

// resolveSpot finds the active respawn with the name, or the only one whose name
// contains it.
func (s *Service) resolveSpot(ctx context.Context, guildID, name string) (*spot.Spot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, booking.ErrSpotNotFound
	}
	sp, err := s.spots.SelectGuildSpotByName(ctx, guildID, name)
	if err == nil {
		return sp, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return nil, fmt.Errorf("select spot by name: %w", err)
	}
	candidates, err := s.spots.SelectGuildSpotsLike(ctx, guildID, name)
	if err != nil {
		return nil, fmt.Errorf("select spots like: %w", err)
	}
	switch len(candidates) {
	case 0:
		return nil, booking.ErrSpotNotFound
	case 1:
		return candidates[0], nil
	default:
		return nil, &AmbiguousSpotError{Query: name, Candidates: candidates}
	}
}

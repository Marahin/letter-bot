// Package spots manages the respawn list of a guild: add, rename, remove
// (delete or archive), restore and the default list import.
package spots

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

// MaxNameLength is the web_spot.name column limit, in characters.
const MaxNameLength = 120

var (
	ErrNameEmpty     = errors.New("spot name is empty")
	ErrNameTooLong   = errors.New("spot name is too long")
	ErrDuplicateName = errors.New("an active spot has this name")
	ErrNotFound      = errors.New("spot not found")
)

// Service implements ports.SpotService.
type Service struct {
	spots    ports.SpotRepository
	notifier ports.BotNotifier
	log      *zap.SugaredLogger
}

func New(spots ports.SpotRepository, notifier ports.BotNotifier, log *zap.SugaredLogger) *Service {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Service{spots: spots, notifier: notifier, log: log}
}

// List returns the spots of one tab that match the query, and the size of both tabs.
func (s *Service) List(ctx context.Context, guildID string, filter spot.ListFilter) (*spot.List, error) {
	all, err := s.spots.SelectGuildSpots(ctx, guildID, true)
	if err != nil {
		return nil, fmt.Errorf("select spots: %w", err)
	}
	counts, err := s.spots.SelectGuildSpotReservationCounts(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("count spot reservations: %w", err)
	}

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	list := &spot.List{Spots: []spot.Listed{}}
	for _, sp := range all {
		if sp.IsArchived() {
			list.ArchivedCount++
		} else {
			list.ActiveCount++
		}
		if sp.IsArchived() != filter.Archived {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(sp.Name), query) {
			continue
		}
		list.Spots = append(list.Spots, spot.Listed{Spot: *sp, Reservations: counts[sp.ID]})
	}
	return list, nil
}

func (s *Service) Create(ctx context.Context, guildID, name string) (*spot.Spot, error) {
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}
	created, err := s.spots.InsertSpot(ctx, guildID, name)
	if err != nil {
		return nil, mapRepoError(err)
	}
	s.summaryChanged(ctx, guildID)
	return created, nil
}

func (s *Service) Rename(ctx context.Context, guildID string, id int64, name string) error {
	name, err := normalizeName(name)
	if err != nil {
		return err
	}
	if err := s.spots.RenameSpot(ctx, guildID, id, name); err != nil {
		return mapRepoError(err)
	}
	s.summaryChanged(ctx, guildID)
	return nil
}

// Remove tries the delete first: the repository refuses it while any reservation
// points at the spot, which also covers a booking made a moment ago and the old
// rows of other guilds. The spot is then archived instead.
func (s *Service) Remove(ctx context.Context, guildID string, id int64) (spot.RemoveOutcome, error) {
	current, err := s.spots.SelectGuildSpotByID(ctx, guildID, id)
	if err != nil {
		return "", mapRepoError(err)
	}

	err = s.spots.DeleteSpot(ctx, guildID, id)
	if err == nil {
		s.summaryChanged(ctx, guildID)
		return spot.RemoveDeleted, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return "", fmt.Errorf("delete spot: %w", err)
	}

	if current.IsArchived() {
		return spot.RemoveArchived, nil
	}
	if err := s.spots.ArchiveSpot(ctx, guildID, id); err != nil {
		return "", mapRepoError(err)
	}
	s.summaryChanged(ctx, guildID)
	return spot.RemoveArchived, nil
}

// Restore returns ErrDuplicateName when an active spot took the name meanwhile.
func (s *Service) Restore(ctx context.Context, guildID string, id int64) error {
	if err := s.spots.RestoreSpot(ctx, guildID, id); err != nil {
		return mapRepoError(err)
	}
	s.summaryChanged(ctx, guildID)
	return nil
}

func (s *Service) ImportDefaults(ctx context.Context, guildID string) (int64, error) {
	added, err := s.spots.InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames)
	if err != nil {
		return 0, fmt.Errorf("import default spots: %w", err)
	}
	if added > 0 {
		s.summaryChanged(ctx, guildID)
	}
	return added, nil
}

// summaryChanged is best-effort: the bot refreshes the summary on its next tick anyway.
func (s *Service) summaryChanged(ctx context.Context, guildID string) {
	if err := s.notifier.SummaryChanged(ctx, guildID); err != nil {
		s.log.Warnw("notify bot of spot change", "guild_id", guildID, "error", err)
	}
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", ErrNameEmpty
	case utf8.RuneCountInString(name) > MaxNameLength:
		return "", ErrNameTooLong
	}
	return name, nil
}

func mapRepoError(err error) error {
	switch {
	case errors.Is(err, ports.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, ports.ErrDuplicate):
		return ErrDuplicateName
	}
	return err
}

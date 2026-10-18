// Package spots manages the respawn list of a guild: add, edit (name, region,
// image), remove (delete or archive), restore and the default list import.
package spots

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/regions"
	"spot-assistant/internal/ports"
)

const (
	// MaxNameLength is the web_spot.name column limit, in characters.
	MaxNameLength = 120
	// MaxImageURLLength is the web_spot.image_url check constraint.
	MaxImageURLLength = 2048
)

var (
	ErrNameEmpty       = errors.New("spot name is empty")
	ErrNameTooLong     = errors.New("spot name is too long")
	ErrDuplicateName   = errors.New("an active spot has this name")
	ErrNotFound        = errors.New("spot not found")
	ErrUnknownRegion   = errors.New("unknown region")
	ErrImageURLTooLong = errors.New("image link is too long")
	ErrImageURLInvalid = errors.New("image link is not an http or https URL")
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
	filter.Query = strings.TrimSpace(filter.Query)
	spots, err := s.spots.SelectGuildSpotList(ctx, guildID, filter)
	if err != nil {
		return nil, fmt.Errorf("select spots: %w", err)
	}
	active, archived, err := s.spots.CountGuildSpots(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("count spots: %w", err)
	}
	if spots == nil {
		spots = []spot.Listed{}
	}
	return &spot.List{Spots: spots, ActiveCount: active, ArchivedCount: archived}, nil
}

func (s *Service) Create(ctx context.Context, guildID string, f spot.Fields) (*spot.Spot, error) {
	f, err := normalizeFields(f)
	if err != nil {
		return nil, err
	}
	created, err := s.spots.InsertSpot(ctx, guildID, f)
	if err != nil {
		return nil, mapRepoError(err)
	}
	s.summaryChanged(ctx, guildID)
	return created, nil
}

func (s *Service) Update(ctx context.Context, guildID string, id int64, f spot.Fields) error {
	f, err := normalizeFields(f)
	if err != nil {
		return err
	}
	if err := s.spots.UpdateSpot(ctx, guildID, id, f); err != nil {
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
	defaultRegions := make([]string, len(DefaultNames))
	for i, name := range DefaultNames {
		defaultRegions[i] = regions.DefaultSpotRegion[name]
	}
	added, err := s.spots.InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames, defaultRegions)
	if err != nil {
		return 0, fmt.Errorf("import default spots: %w", err)
	}
	if added > 0 {
		s.summaryChanged(ctx, guildID)
	}
	return added, nil
}

// summaryChanged is best-effort: the bot refreshes every summary when its listener connects again.
func (s *Service) summaryChanged(ctx context.Context, guildID string) {
	if err := s.notifier.SummaryChanged(ctx, guildID); err != nil {
		s.log.Warnw("notify bot of spot change", "guild_id", guildID, "error", err)
	}
}

func normalizeFields(f spot.Fields) (spot.Fields, error) {
	name, err := normalizeName(f.Name)
	if err != nil {
		return spot.Fields{}, err
	}
	regionKey := strings.TrimSpace(f.Region)
	if regionKey != "" && !regions.Valid(regionKey) {
		return spot.Fields{}, ErrUnknownRegion
	}
	image, err := NormalizeImageURL(f.ImageURL)
	if err != nil {
		return spot.Fields{}, err
	}
	return spot.Fields{Name: name, Region: regionKey, ImageURL: image}, nil
}

// NormalizeImageURL trims the link and accepts "" or an absolute http or https URL.
func NormalizeImageURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if utf8.RuneCountInString(raw) > MaxImageURLLength {
		return "", ErrImageURLTooLong
	}
	if strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return "", ErrImageURLInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", ErrImageURLInvalid
	}
	return raw, nil
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

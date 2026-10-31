package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/world"
)

type HighscoreAPI interface {
	// GetHighscoresPage returns one page (1-based) of the world's experience highscores, all vocations.
	GetHighscoresPage(ctx context.Context, worldName string, page int) (*world.HighscorePage, error)
}

type CharacterAPI interface {
	// GetCharacter returns ErrCharacterNotFound for an unknown name.
	GetCharacter(ctx context.Context, name string) (*character.Character, error)
}

type ExperienceRepository interface {
	// ListTrackedWorlds returns the distinct worlds of premium guilds.
	ListTrackedWorlds(ctx context.Context) ([]string, error)
	// ListTrackedCharacterKeys returns the character keys in the authors of the world's reservations that end at or after since.
	ListTrackedCharacterKeys(ctx context.Context, worldName string, since time.Time) ([]string, error)
	// LatestSnapshots returns the newest snapshot of each given character that has one, by key.
	LatestSnapshots(ctx context.Context, worldName string, keys []string) (map[string]experience.Snapshot, error)
	// SaveRun writes the new snapshots, moves last_seen_at of the seen ones and records the run, in one transaction.
	SaveRun(ctx context.Context, result experience.RunResult) error
	// FirstRunObservedAt returns ErrNotFound when the world has no run.
	FirstRunObservedAt(ctx context.Context, worldName string) (time.Time, error)
	// FirstRunObservedAtOrAfter returns ErrNotFound when no run was observed at or after t.
	FirstRunObservedAtOrAfter(ctx context.Context, worldName string, t time.Time) (time.Time, error)
	// SnapshotAtOrBefore returns the newest snapshot observed at or before t, or ErrNotFound.
	SnapshotAtOrBefore(ctx context.Context, worldName, key string, t time.Time) (*experience.Snapshot, error)
	// FirstSnapshotAfter returns the oldest snapshot observed in (from, to], or ErrNotFound.
	FirstSnapshotAfter(ctx context.Context, worldName, key string, from, to time.Time) (*experience.Snapshot, error)
	// PendingReservations returns reservations of the world's premium guilds with start_at >= startFrom and
	// end_at in [endFrom, endTo] that have no reservation_experience rows.
	PendingReservations(ctx context.Context, worldName string, startFrom, endFrom, endTo time.Time) ([]experience.PendingReservation, error)
	// InsertReservationExperience ignores rows that already exist.
	InsertReservationExperience(ctx context.Context, rows []experience.ReservationExperience) error
	// SnapshotHistory returns the character's snapshots observed in [from, to], oldest first.
	SnapshotHistory(ctx context.Context, worldName, key string, from, to time.Time) ([]experience.Snapshot, error)
}

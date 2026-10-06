// Package experience holds the highscore snapshot and reservation experience models.
package experience

import (
	"strings"
	"time"
)

// Snapshot is one experience value of a character. The value was seen by every run
// observed from ObservedAt to LastSeenAt.
type Snapshot struct {
	ID            int64
	World         string
	CharacterKey  string
	CharacterName string
	Level         int
	Experience    int64
	Vocation      string
	ObservedAt    time.Time
	LastSeenAt    time.Time
}

// Run is one complete read of a world's experience highscores.
type Run struct {
	World      string
	ObservedAt time.Time
	FetchedAt  time.Time
	Pages      int
	Rows       int
}

// RunResult is what one run writes: new snapshots, and snapshots seen again with the same value.
type RunResult struct {
	Run      Run
	Inserted []Snapshot
	SeenIDs  []int64
}

// PendingReservation is an ended reservation that has no experience rows yet.
type PendingReservation struct {
	ID      int64
	Author  string
	StartAt time.Time
	EndAt   time.Time
}

type Status string

const (
	StatusOK     Status = "ok"
	StatusNoData Status = "no_data"
)

// ReservationExperience is the gain of one character during one reservation. Values are nil when
// Status is StatusNoData. Gain can be negative after a death.
type ReservationExperience struct {
	ReservationID   int64
	CharacterKey    string
	CharacterName   string
	StartExperience *int64
	EndExperience   *int64
	Gain            *int64
	Status          Status
}

// CharacterKey normalizes a character name the same way the SQL does: lower(btrim(name)).
func CharacterKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Characters splits a reservation author ("A/B/C") into unique characters, keeping the first spelling.
func Characters(author string) []Character {
	var out []Character
	seen := map[string]bool{}
	for _, part := range strings.Split(author, "/") {
		key := CharacterKey(part)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Character{Key: key, Name: strings.TrimSpace(part)})
	}
	return out
}

type Character struct {
	Key  string
	Name string
}

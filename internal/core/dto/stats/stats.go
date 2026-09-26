// Package stats holds the reservation and experience statistics models.
package stats

import (
	"time"

	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/spot"
)

// Range is a span of whole local days: [From, To) on start_at, with one midnight per day in Days.
type Range struct {
	From time.Time
	To   time.Time
	Days []time.Time
}

// Filter selects the reservations of one guild that start in [From, To).
type Filter struct {
	GuildID string
	From    time.Time
	To      time.Time
	// SpotID 0 means every respawn.
	SpotID int64
	// UserID "" means every player.
	UserID string
	// CharacterKey "" means every character. When set, experience counts only this character.
	CharacterKey string
}

// Totals sums reservations and their experience. Only reservations with experience data
// (at least one character with status ok) count in ExpReservations and ExpSeconds.
type Totals struct {
	Reservations    int64
	Seconds         int64
	ExpReservations int64
	ExpSeconds      int64
	Exp             int64
}

func (t Totals) Hours() float64 { return float64(t.Seconds) / 3600 }

func (t Totals) ExpHours() float64 { return float64(t.ExpSeconds) / 3600 }

func (t Totals) HasExp() bool { return t.ExpReservations > 0 }

// ExpTotal is nil when no reservation has experience data.
func (t Totals) ExpTotal() *int64 {
	if !t.HasExp() {
		return nil
	}
	v := t.Exp
	return &v
}

// ExpPerHour divides the experience by the hours of the reservations with data. It is nil without data.
func (t Totals) ExpPerHour() *float64 {
	if !t.HasExp() || t.ExpSeconds <= 0 {
		return nil
	}
	v := float64(t.Exp) / t.ExpHours()
	return &v
}

func (t Totals) Add(o Totals) Totals {
	return Totals{
		Reservations:    t.Reservations + o.Reservations,
		Seconds:         t.Seconds + o.Seconds,
		ExpReservations: t.ExpReservations + o.ExpReservations,
		ExpSeconds:      t.ExpSeconds + o.ExpSeconds,
		Exp:             t.Exp + o.Exp,
	}
}

type SpotRow struct {
	SpotID   int64
	Name     string
	Archived bool
	Totals
}

// PlayerRow is one Discord user. Name is the author text of their latest reservation.
type PlayerRow struct {
	UserID string
	Name   string
	Totals
}

// CharacterRow is one character of the reservation authors. Name is its latest spelling.
type CharacterRow struct {
	Key  string
	Name string
	Totals
}

// Day is the totals of the reservations that start on one local day (midnight).
type Day struct {
	Day time.Time
	Totals
}

// CharacterReservation is one reservation of a character with the character's own experience.
// Status "" means the experience is not computed yet.
type CharacterReservation struct {
	ID       int64
	SpotID   int64
	SpotName string
	Author   string
	StartAt  time.Time
	EndAt    time.Time
	Status   experience.Status
	Gain     *int64
}

type SortKey string

const (
	SortName         SortKey = "name"
	SortReservations SortKey = "reservations"
	SortHours        SortKey = "hours"
	SortExp          SortKey = "exp"
	SortExpPerHour   SortKey = "exp_h"
)

// SortKeys lists every SortKey in column order.
var SortKeys = []SortKey{SortName, SortReservations, SortHours, SortExp, SortExpPerHour}

type Sort struct {
	Key SortKey
	Asc bool
}

// Query orders the totals rows of Filter by Sort and keeps the first Limit (0 = every row).
// Rows without the sort figure come last in both directions; ties go by name.
type Query struct {
	Filter
	Sort  Sort
	Limit int
}

// Page is the first rows of a query and the number of rows without the limit.
type Page[T any] struct {
	Rows  []T
	Total int
}

// CharacterBoards are the overview's character leaderboards and the number of characters.
type CharacterBoards struct {
	ByExp        []CharacterRow
	ByExpPerHour []CharacterRow
	Characters   int
}

// Leaderboards are the top rows of the overview.
type Leaderboards struct {
	PlayersByHours         []PlayerRow
	CharactersByExp        []CharacterRow
	CharactersByExpPerHour []CharacterRow
}

type Overview struct {
	Range        Range
	Totals       Totals
	Spots        int
	Players      int
	Characters   int
	Daily        []Day
	TopSpots     []SpotRow
	Leaderboards Leaderboards
}

type SpotDetail struct {
	Spot       *spot.Spot
	Range      Range
	Totals     Totals
	Daily      []Day
	Players    Page[PlayerRow]
	Characters Page[CharacterRow]
}

type PlayerDetail struct {
	UserID     string
	Name       string
	Range      Range
	Totals     Totals
	Daily      []Day
	Spots      Page[SpotRow]
	Characters Page[CharacterRow]
}

// ProfileSource says what the character page got from TibiaData.
type ProfileSource string

const (
	ProfileFound       ProfileSource = "found"
	ProfileNotFound    ProfileSource = "not_found"
	ProfileUnavailable ProfileSource = "unavailable"
)

// HistoryPoint is the last known level and experience of a character on one local day.
type HistoryPoint struct {
	Day        time.Time
	Level      int
	Experience int64
}

type CharacterProfile struct {
	Key       string
	Name      string
	World     string
	Source    ProfileSource
	Character *character.Character
	Range     Range
	History   []HistoryPoint
	Totals    Totals
	Daily     []Day
	Spots     Page[SpotRow]
	Recent    []CharacterReservation
}

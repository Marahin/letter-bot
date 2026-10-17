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

// Totals sums reservations, their experience and their loot. Only reservations with experience
// data (at least one character with status ok) count in ExpReservations and ExpSeconds; only
// reservations with a loot session count in LootReservations. LootSeconds is session time, not
// booked time.
type Totals struct {
	Reservations    int64
	Seconds         int64
	ExpReservations int64
	ExpSeconds      int64
	Exp             int64

	LootReservations int64
	LootSeconds      int64
	Profit           int64
	Loot             int64
	Supplies         int64
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

func (t Totals) HasLoot() bool { return t.LootReservations > 0 }

func (t Totals) LootHours() float64 { return float64(t.LootSeconds) / 3600 }

// ProfitTotal is the summed balance, nil when no reservation has a loot session.
func (t Totals) ProfitTotal() *int64 { return t.lootFigure(t.Profit) }

func (t Totals) LootTotal() *int64 { return t.lootFigure(t.Loot) }

func (t Totals) SuppliesTotal() *int64 { return t.lootFigure(t.Supplies) }

// ProfitPerHour divides the profit by the session hours. It is nil without data.
func (t Totals) ProfitPerHour() *float64 {
	if !t.HasLoot() || t.LootSeconds <= 0 {
		return nil
	}
	v := float64(t.Profit) / t.LootHours()
	return &v
}

func (t Totals) lootFigure(v int64) *int64 {
	if !t.HasLoot() {
		return nil
	}
	return &v
}

func (t Totals) Add(o Totals) Totals {
	return Totals{
		Reservations:     t.Reservations + o.Reservations,
		Seconds:          t.Seconds + o.Seconds,
		ExpReservations:  t.ExpReservations + o.ExpReservations,
		ExpSeconds:       t.ExpSeconds + o.ExpSeconds,
		Exp:              t.Exp + o.Exp,
		LootReservations: t.LootReservations + o.LootReservations,
		LootSeconds:      t.LootSeconds + o.LootSeconds,
		Profit:           t.Profit + o.Profit,
		Loot:             t.Loot + o.Loot,
		Supplies:         t.Supplies + o.Supplies,
	}
}

type SpotRow struct {
	Totals

	SpotID   int64
	Name     string
	Archived bool
}

// PlayerRow is one Discord user. Name is the author text of their latest reservation.
type PlayerRow struct {
	Totals

	UserID string
	Name   string
}

// CharacterRow is one character of the reservation authors. Name is its latest spelling.
type CharacterRow struct {
	Totals

	Key  string
	Name string
}

// Day is the totals of the reservations that start on one local day (midnight).
type Day struct {
	Totals

	Day time.Time
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
	SortName          SortKey = "name"
	SortReservations  SortKey = "reservations"
	SortHours         SortKey = "hours"
	SortExp           SortKey = "exp"
	SortExpPerHour    SortKey = "exp_h"
	SortProfit        SortKey = "profit"
	SortProfitPerHour SortKey = "profit_h"
	// SortWorlds orders spawns by the number of worlds that booked them.
	SortWorlds SortKey = "worlds"
)

// SortKeys lists every SortKey of the guild tables in column order.
var SortKeys = []SortKey{SortName, SortReservations, SortHours, SortExp, SortExpPerHour, SortProfit, SortProfitPerHour}

// SpawnSortKeys lists every SortKey of the spawn directory in column order.
var SpawnSortKeys = []SortKey{SortName, SortWorlds, SortReservations, SortHours, SortExpPerHour, SortProfitPerHour}

type Sort struct {
	Key SortKey
	Asc bool
}

// Query orders the totals rows of Filter by Sort and keeps the first Limit (0 = every row).
// Rows without the sort figure come last in both directions; ties go by name,
// or by id when the sort is not by name.
type Query struct {
	Filter

	Sort  Sort
	Limit int
	// MinLootSeconds keeps rows with less session time out (0 = every row). Only SpotTotals reads it.
	MinLootSeconds int64
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
	ByProfit     []CharacterRow
	Characters   int
}

// Leaderboards are the top rows of the overview.
type Leaderboards struct {
	PlayersByHours         []PlayerRow
	CharactersByExp        []CharacterRow
	CharactersByExpPerHour []CharacterRow
	SpotsByProfitPerHour   []SpotRow
	CharactersByProfit     []CharacterRow
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

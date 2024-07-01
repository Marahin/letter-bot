package summary

import (
	"time"
)

type Summary struct {
	PreMessage   string
	Chart        []byte
	URL          string
	Title        string
	Footer       string
	Description  string
	Ledger       Ledger
	LegendValues []LegendValue
}

type Ledger []LedgerEntry

type LedgerEntry struct {
	Spot string

	Bookings []*Booking
}

type Booking struct {
	Author          string
	AuthorDiscordID string
	StartAt         time.Time
	EndAt           time.Time
}

// LegendValue is a container for label (Legend) and float64 value (Value)
type LegendValue struct {
	Legend string
	Value  float64
}

type PrivateSummaryRequest struct {
	UserID    int64
	GuildID   int64
	SpotNames []string
}

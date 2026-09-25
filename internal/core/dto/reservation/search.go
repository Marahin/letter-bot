package reservation

import "time"

// SearchScope limits a reservation search by time relative to now.
type SearchScope string

const (
	ScopeAll      SearchScope = "all"
	ScopeUpcoming SearchScope = "upcoming"
	ScopePast     SearchScope = "past"
)

// SearchFilter selects the reservations of one guild. Nil and empty fields do not filter.
type SearchFilter struct {
	GuildID         string
	SpotID          *int64
	Author          string
	AuthorDiscordID string
	From            *time.Time
	To              *time.Time
	Scope           SearchScope
	Limit           int
	Offset          int
}

// KnownAuthor is a Discord user who booked before, with the latest author text they used.
type KnownAuthor struct {
	AuthorDiscordID string
	Author          string
}

package spot

import "time"

type Spot struct {
	Name       string
	ID         int64
	CreatedAt  time.Time
	GuildID    string
	ArchivedAt *time.Time
}

func (s Spot) IsArchived() bool {
	return s.ArchivedAt != nil
}

// ReservationCounts are the past and upcoming reservations of one spot. Upcoming
// includes the reservation in progress.
type ReservationCounts struct {
	Total    int64
	Upcoming int64
}

// Listed is a spot on the respawn list, with its reservation counts.
type Listed struct {
	Spot

	Reservations ReservationCounts
}

// ListFilter selects one tab of the respawn list. Query matches a part of the
// name, case-insensitive.
type ListFilter struct {
	Archived bool
	Query    string
}

// List is one tab of the respawn list and the size of both tabs.
type List struct {
	Spots         []Listed
	ActiveCount   int
	ArchivedCount int
}

// Total counts the active and archived spots.
func (l List) Total() int {
	return l.ActiveCount + l.ArchivedCount
}

// RemoveOutcome says whether Remove deleted a spot or archived it.
type RemoveOutcome string

const (
	RemoveDeleted  RemoveOutcome = "deleted"
	RemoveArchived RemoveOutcome = "archived"
)

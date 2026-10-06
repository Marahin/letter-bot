package reservation

import (
	"time"

	"spot-assistant/internal/core/permission"
)

// Actor is the signed-in web user acting on a guild's reservations.
type Actor struct {
	UserID string
	// Name is the author text of the actor's own reservations (the member display name).
	Name string
	Caps permission.Capabilities
}

// Draft is the form content of a new or changed reservation. The author fields
// count only for a manager; everyone else books as themselves.
type Draft struct {
	SpotID          int64
	StartAt         time.Time
	EndAt           time.Time
	Author          string
	AuthorDiscordID string
	Overbook        bool
}

// Page is one page of a reservation search.
type Page struct {
	Items   []*ReservationWithSpot
	Total   int64
	Page    int
	Pages   int
	PerPage int
}

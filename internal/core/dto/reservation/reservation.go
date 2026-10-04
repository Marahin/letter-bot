package reservation

import (
	"fmt"
	"time"

	stringsHelper "spot-assistant/internal/common/strings"
)

type Reservation struct {
	ID              int64
	Author          string
	CreatedAt       time.Time
	StartAt         time.Time
	EndAt           time.Time
	SpotID          int64
	GuildID         string
	AuthorDiscordID string
}

// ClippedOrRemovedReservation holds both original reservation and
// a slice holding outcome of overbooking the original reservation.
// Used when other reservation clips or removes the original one.
type ClippedOrRemovedReservation struct {
	New      []*Reservation
	Original *Reservation
}

type Spot struct {
	ID   int64
	Name string
}

type ReservationWithSpot struct {
	Reservation
	Spot
}

// Label is the text that names the reservation in the unbook autocomplete.
func (r *ReservationWithSpot) Label() string {
	return fmt.Sprintf("%s - %s %s", r.StartAt.Format(stringsHelper.DcLongTimeFormat), r.EndAt.Format(stringsHelper.DcLongTimeFormat), r.Spot.Name)
}

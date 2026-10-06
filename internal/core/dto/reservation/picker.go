package reservation

import "time"

// AutoWindow asks for the start window that holds the chosen start.
const AutoWindow = -1

// RespawnPicker is step 1 of the bot's booking wizard: the member's usual
// respawns and one page of the active respawns, in groups by name.
type RespawnPicker struct {
	Usual  []UsualRespawn
	Groups []RespawnGroup
	Page   int
	Pages  int
}

// UsualRespawn is a respawn the member booked often (Bookings > 0), or one that
// is popular on the server (Bookings = 0).
type UsualRespawn struct {
	Spot     Spot
	Bookings int64
}

// RespawnGroup is one select of respawns. From and To are the name prefixes of
// the first and the last respawn, e.g. "Ka" and "Ko".
type RespawnGroup struct {
	From  string
	To    string
	Spots []Spot
}

// TimeChoice is what the member picked in the wizard so far.
type TimeChoice struct {
	SpotID int64
	// ReservationID is the reservation to edit, 0 for a booking.
	ReservationID int64
	// Window is the shown start window, or AutoWindow.
	Window int
	// Now starts the reservation at the current minute; StartAt is then ignored.
	Now     bool
	StartAt time.Time
	Length  time.Duration
}

func (c TimeChoice) HasStart() bool {
	return c.Now || !c.StartAt.IsZero()
}

func (c TimeChoice) Complete() bool {
	return c.HasStart() && c.Length > 0
}

// TimePicker is step 2 of the wizard: the start and the length of one respawn.
type TimePicker struct {
	Spot Spot
	// Editing is the reservation to edit, nil for a booking.
	Editing *ReservationWithSpot
	// Ongoing means Editing has started: only the length can change.
	Ongoing bool
	// Choice has the window resolved.
	Choice  TimeChoice
	Windows int
	Starts  []StartOption
	Lengths []LengthOption
	// Booked are the respawn's reservations in the shown window, by start.
	Booked []*Reservation
}

// StartOption is one start in the list. Now, Keep and Current are special
// options; a plain one is a slot of the grid.
type StartOption struct {
	StartAt time.Time
	Now     bool
	// Keep is the start of an ongoing reservation.
	Keep bool
	// Current is the start of the edited reservation, off the grid.
	Current  bool
	Selected bool
	// BookedBy is the reservation that holds this start, if any.
	BookedBy *Reservation
}

type LengthOption struct {
	Length   time.Duration
	Current  bool
	Selected bool
}

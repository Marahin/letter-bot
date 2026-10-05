package reservation

// Form is the text a member typed into a Discord form.
type Form struct {
	Spot    string
	StartAt string
	EndAt   string
}

// FormOutcome is the result of a booking or an edit from a Discord form.
type FormOutcome struct {
	Draft    Draft
	SpotName string
	// Overbooked are the reservations a successful overbook shortened or removed.
	Overbooked []*ClippedOrRemovedReservation
	// Conflicts are the reservations that stopped the booking or the edit.
	Conflicts []*Reservation
	// CanOverbook means the member may repeat the booking with Draft.Overbook.
	CanOverbook bool
}

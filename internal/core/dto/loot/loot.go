// Package loot holds the Party Hunt sessions counted from the Loot Calculator.
package loot

import "time"

// Member is one party member as the analyser reports them.
type Member struct {
	Name     string
	Leader   bool
	Loot     int64
	Supplies int64
	Balance  int64
	Damage   int64
	Healing  int64
}

// Submission is a parsed analyser text. From and To are as printed ("2026-09-25, 20:14:39"),
// empty when the paste has no session line. SubmittedBy is the signed-in submitter's Discord
// user id, "" when anonymous.
type Submission struct {
	From        string
	To          string
	Members     []Member
	SubmittedBy string
}

// SessionMember is a Member with its character key and paste position.
type SessionMember struct {
	Member

	Key      string
	Position int
}

// Session is a stored loot session. The party figures and PartySize cover every pasted line, as
// the split does; Members keeps one line per character.
type Session struct {
	ID          int64
	Hash        []byte
	SubmittedBy string
	StartedAt   time.Time
	EndedAt     time.Time
	Duration    time.Duration
	Members     []SessionMember
	PartySize   int
	Loot        int64
	Supplies    int64
	Balance     int64
	Damage      int64
	Healing     int64
}

// Candidate is a reservation a session may belong to.
type Candidate struct {
	ReservationID   int64
	GuildID         string
	GuildName       string
	SpotID          int64
	SpotName        string
	Author          string
	AuthorDiscordID string
	StartAt         time.Time
	EndAt           time.Time
}

// Attribution gives a session to a reservation. ByAuthor is set when the reservation's author
// submitted the session: such a session is never replaced by someone else's.
type Attribution struct {
	SessionID      int64
	ReservationID  int64
	OverlapSeconds int64
	ByAuthor       bool
}

// Status says what happened to a submitted session.
type Status string

const (
	// StatusCounted: the session is stored and counts for Match.
	StatusCounted Status = "counted"
	// StatusUnticked: the user opted out; nothing is written.
	StatusUnticked Status = "unticked"
	// StatusNoTime: the paste has no usable session line; nothing is written.
	StatusNoTime Status = "no_time"
	// StatusFuture: the session ends in the future; nothing is written.
	StatusFuture Status = "future"
	// StatusOutOfRange: a figure, the party size or the length is out of bounds; nothing is written.
	StatusOutOfRange Status = "out_of_range"
	// StatusUnmatched: the session is stored, but no reservation matches it.
	StatusUnmatched Status = "unmatched"
	// StatusOutmatched: Match keeps another session (its author's, or one with a larger overlap).
	StatusOutmatched Status = "outmatched"
)

// Outcome is the result of a submission. Match is set for counted and outmatched.
type Outcome struct {
	Status Status
	Match  *Candidate
}

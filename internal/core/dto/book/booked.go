package book

import "time"

// BookedSource is where a booking was made.
type BookedSource int

const (
	// SourceDiscord is a button or the booking wizard in Discord.
	SourceDiscord BookedSource = iota + 1
	// SourceWeb is the web panel.
	SourceWeb
	// SourceSlash is the /book command, which already replies in public.
	SourceSlash
)

// Booked is a new reservation, announced to the guild.
type Booked struct {
	GuildID string
	SpotID  int64
	// SpotName is optional. The bot looks the name up when it is empty.
	SpotName        string
	StartAt         time.Time
	EndAt           time.Time
	Author          string
	AuthorDiscordID string
	// ActorID is the Discord id of the member who booked. It differs from
	// AuthorDiscordID when a manager booked for somebody else.
	ActorID string
	// Overbooked is the number of reservations the booking shortened or removed.
	Overbooked int
	Source     BookedSource
}

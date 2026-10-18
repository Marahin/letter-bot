// Package queue holds the models of the respawn queue: a member waits for a
// respawn to be free, and the bot offers it to them.
package queue

import "time"

// Entry is one member waiting for one respawn of a guild.
type Entry struct {
	ID       int64
	GuildID  string
	SpotID   int64
	SpotName string
	UserID   string
	// Length is the hunt the member wants: the respawn counts as free when it is
	// free for this long.
	Length    time.Duration
	CreatedAt time.Time
	ExpiresAt time.Time
	// OfferedAt is when the bot offered the free respawn. It is cleared when the
	// respawn is taken again, so the next free window gets a new offer.
	OfferedAt *time.Time
}

// Candidate is a live entry of an active respawn, and whether the respawn is
// free for the entry's length now.
type Candidate struct {
	Entry

	Free bool
}

// JoinOutcome is the entry after a member queued. FreeNow means the respawn was
// free, and the entry got its offer at once.
type JoinOutcome struct {
	Entry   Entry
	FreeNow bool
}

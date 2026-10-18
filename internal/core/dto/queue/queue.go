// Package queue holds the models of the respawn queue: a member waits for a
// respawn to be free, and the bot offers it to them.
package queue

import (
	"time"

	"spot-assistant/internal/core/dto/reservation"
)

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

// JoinRequest queues the actor for a respawn and a hunt of Length. A zero
// Length is the default length.
type JoinRequest struct {
	GuildID string
	Actor   reservation.Actor
	SpotID  int64
	Length  time.Duration
}

// JoinOutcome is the entry after a member queued. FreeNow means the respawn was
// free, and the entry got its offer at once.
type JoinOutcome struct {
	Entry   Entry
	FreeNow bool
}

// UserCount selects the member's live entries of active respawns in the guild,
// except the one for ExceptSpotID.
type UserCount struct {
	GuildID      string
	UserID       string
	ExceptSpotID int64
	Now          time.Time
}

// DueQuery selects the guild's live entries of active respawns that have no
// offer and whose respawn is free from StartAt.
type DueQuery struct {
	GuildID string
	Now     time.Time
	StartAt time.Time
}

// Claim sets the offer time of an entry, when its respawn is free from StartAt.
type Claim struct {
	EntryID   int64
	OfferedAt time.Time
	StartAt   time.Time
}

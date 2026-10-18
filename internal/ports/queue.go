package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
)

// SpotQueueRepository stores the members who wait for a respawn to be free.
type SpotQueueRepository interface {
	// UpsertEntry adds the member's entry for the respawn, or replaces its
	// length and expiry and clears its offer. It sets ID and CreatedAt.
	UpsertEntry(ctx context.Context, e queue.Entry) (*queue.Entry, error)
	// CountUserEntries counts the member's live entries in the guild, except the
	// one for exceptSpotID.
	CountUserEntries(ctx context.Context, guildID, userID string, exceptSpotID int64, now time.Time) (int, error)
	// GetEntry returns the entry with its respawn name, or ErrNotFound.
	GetEntry(ctx context.Context, id int64) (*queue.Entry, error)
	// ListUserEntries returns the member's live entries in the guild, oldest first.
	ListUserEntries(ctx context.Context, guildID, userID string, now time.Time) ([]queue.Entry, error)
	// DeleteEntry deletes the member's entry and reports whether it existed.
	DeleteEntry(ctx context.Context, id int64, userID string) (bool, error)
	// DeleteUserSpotEntry deletes the member's entry for the respawn and reports whether it existed.
	DeleteUserSpotEntry(ctx context.Context, guildID, userID string, spotID int64) (bool, error)
	// DeleteExpired deletes the guild's entries that expired by now.
	DeleteExpired(ctx context.Context, guildID string, now time.Time) (int64, error)
	// SelectGuildCandidates returns the guild's live entries of active respawns,
	// oldest first. An entry is free when no reservation of its respawn overlaps
	// [startAt, startAt + length], edges included, as in booking.
	SelectGuildCandidates(ctx context.Context, guildID string, now, startAt time.Time) ([]queue.Candidate, error)
	// ClaimOffer sets the offer time of an entry without one, and reports whether it did.
	ClaimOffer(ctx context.Context, id int64, at time.Time) (bool, error)
	// ResetOffers clears the offer time of the entries.
	ResetOffers(ctx context.Context, ids []int64) error
}

// SpotQueueService is the respawn queue of the bot: a member waits for a taken
// respawn, and gets an offer to book it when it is free.
type SpotQueueService interface {
	// Join queues the actor for the respawn and a hunt of the length. It returns
	// booking.ErrReserveNotAllowed, booking.ErrSpotNotFound,
	// booking.ErrSpotArchived, queue.ErrInvalidLength and queue.ErrQueueFull.
	Join(ctx context.Context, guildID string, actor reservation.Actor, spotID int64, length time.Duration) (*queue.JoinOutcome, error)
	// Leave deletes the member's entry. It returns ErrNotFound when it is gone.
	Leave(ctx context.Context, userID string, entryID int64) error
	// Mine returns the member's live entries in the guild.
	Mine(ctx context.Context, guildID, userID string) ([]queue.Entry, error)
	// Entry returns the entry, or ErrNotFound.
	Entry(ctx context.Context, id int64) (*queue.Entry, error)
	// DueOffers deletes the expired entries, clears the offers of respawns that
	// were taken again, and claims an offer for each entry whose respawn is
	// free. It returns the claimed entries, for the caller to send.
	DueOffers(ctx context.Context, guildID string) ([]queue.Entry, error)
	// Unclaim clears the offer of an entry whose offer could not be sent, so a
	// later pass tries again.
	Unclaim(ctx context.Context, entryID int64) error
	// CheckOffer tells whether the member may still use the offer made at
	// offeredAt: queue.ErrOfferNotYours or queue.ErrOfferExpired.
	CheckOffer(e *queue.Entry, userID string, offeredAt time.Time) error
	// Book books the entry's respawn from now for its length, through the
	// normal booking, and removes the entry. Errors as CheckOffer, ErrNotFound,
	// and the errors of ReservationFormService.Book.
	Book(ctx context.Context, actor reservation.Actor, entryID int64, offeredAt time.Time) (*reservation.FormOutcome, error)
	// Fulfil removes the member's entry for a respawn they booked.
	Fulfil(ctx context.Context, guildID, userID string, spotID int64) error
}

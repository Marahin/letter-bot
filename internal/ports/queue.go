package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
)

// SpotQueueFulfiller removes the queue entry of a member who booked the respawn.
type SpotQueueFulfiller interface {
	// DeleteUserSpotEntry deletes the member's entry for the respawn and reports whether it existed.
	DeleteUserSpotEntry(ctx context.Context, guildID, userID string, spotID int64) (bool, error)
}

// SpotQueueRepository stores the members who wait for a respawn to be free.
// "Free" means that no reservation of the respawn overlaps [start, start +
// length], edges included, as in booking.
type SpotQueueRepository interface {
	SpotQueueFulfiller

	// UpsertEntry adds the member's entry for the respawn, or replaces its
	// length and expiry and clears its offer. It sets ID and CreatedAt.
	UpsertEntry(ctx context.Context, e queue.Entry) (*queue.Entry, error)
	CountUserEntries(ctx context.Context, c queue.UserCount) (int, error)
	// GetEntry returns the entry with its respawn name, or ErrNotFound.
	GetEntry(ctx context.Context, id int64) (*queue.Entry, error)
	// ListUserEntries returns the member's live entries in the guild, oldest first.
	ListUserEntries(ctx context.Context, guildID, userID string, now time.Time) ([]queue.Entry, error)
	// DeleteEntry deletes the member's entry and reports whether it existed.
	DeleteEntry(ctx context.Context, id int64, userID string) (bool, error)
	// DeleteExpired deletes the entries of all guilds that expired by now.
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
	// SelectDueEntries returns the matching entries, oldest first.
	SelectDueEntries(ctx context.Context, q queue.DueQuery) ([]queue.Entry, error)
	// ClaimOffer sets the offer time of a live entry without one whose respawn
	// is free, and reports whether it did.
	ClaimOffer(ctx context.Context, c queue.Claim) (bool, error)
	// ResetTakenOffers clears the offer time of the guild's entries whose
	// respawn is not free from startAt.
	ResetTakenOffers(ctx context.Context, guildID string, startAt time.Time) error
	// ResetOffer clears the offer time of the entry while it is still offeredAt.
	ResetOffer(ctx context.Context, id int64, offeredAt time.Time) error
}

// SpotQueueService is the respawn queue of the bot: a member waits for a taken
// respawn, and gets an offer to book it when it is free.
type SpotQueueService interface {
	// Join queues the actor. It returns booking.ErrReserveNotAllowed,
	// booking.ErrSpotNotFound, booking.ErrSpotArchived, queue.ErrInvalidLength
	// and queue.ErrQueueFull.
	Join(ctx context.Context, req queue.JoinRequest) (*queue.JoinOutcome, error)
	// Leave deletes the member's entry. It returns ErrNotFound when it is gone.
	Leave(ctx context.Context, userID string, entryID int64) error
	// Mine returns the member's live entries in the guild.
	Mine(ctx context.Context, guildID, userID string) ([]queue.Entry, error)
	// Entry returns the entry, or ErrNotFound.
	Entry(ctx context.Context, id int64) (*queue.Entry, error)
	// DueOffers deletes the expired entries, clears the offers of respawns that
	// were taken again, and returns the entries without an offer whose respawn
	// is free. The caller claims each one with ClaimOffer just before it sends it.
	DueOffers(ctx context.Context, guildID string) ([]queue.Entry, error)
	// ClaimOffer sets the entry's offer time to now, when the entry has no
	// offer and its respawn is still free. It returns the offer time and
	// whether it claimed the offer.
	ClaimOffer(ctx context.Context, entryID int64) (time.Time, bool, error)
	// Unclaim clears the offer claimed at offeredAt of an entry whose offer
	// could not be sent, so a later pass tries again. A newer claim stays.
	Unclaim(ctx context.Context, entryID int64, offeredAt time.Time) error
	// OfferUsable tells whether the offer made at offeredAt can still book:
	// nil or queue.ErrOfferExpired. It does not check the member.
	OfferUsable(e *queue.Entry, offeredAt time.Time) error
	// Book books the entry's respawn from now for its length, through the
	// normal booking, which also removes the entry. It returns
	// queue.ErrOfferNotYours when the actor did not queue, the errors of
	// OfferUsable, ErrNotFound, and the errors of ReservationFormService.Book.
	Book(ctx context.Context, actor reservation.Actor, entryID int64, offeredAt time.Time) (*reservation.FormOutcome, error)
}

package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/loot"
)

type LootSessionRepository interface {
	// SaveSession stores the session and its members. A session with the same hash returns the
	// stored id and writes nothing.
	SaveSession(ctx context.Context, s loot.Session) (int64, error)
	// MatchCandidates returns the reservations of premium guilds with the bot present that overlap
	// [from, to), start after from-maxLen, and have one of keys among their author characters.
	MatchCandidates(ctx context.Context, from, to time.Time, keys []string, maxLen time.Duration) ([]loot.Candidate, error)
	// Pick attributes the session to the reservation unless the reservation keeps a session with at
	// least the same overlap. On success the session leaves any other reservation. It reports
	// whether the session counts for the reservation.
	Pick(ctx context.Context, sessionID, reservationID, overlapSeconds int64) (bool, error)
}

type LootSessionService interface {
	// Record stores a Loot Calculator session (unless count is false) and attributes it to a
	// reservation. The outcome says what happened; an error is a storage failure.
	Record(ctx context.Context, s loot.Submission, count bool) (loot.Outcome, error)
}

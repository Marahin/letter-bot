package ports

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound means the row does not exist, or does not belong to the given guild.
	ErrNotFound = errors.New("not found")

	// ErrDuplicate means a unique constraint rejected the write.
	ErrDuplicate = errors.New("duplicate")

	// ErrConflict means the write overlaps an existing reservation.
	ErrConflict = errors.New("conflicting reservation")
)

var (
	// ErrUpstreamUnavailable means an external service did not answer or rate-limited us. Retry later.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")

	// ErrUnauthorized means Discord refused the user's stored OAuth token. The user must sign in again.
	ErrUnauthorized = errors.New("oauth token refused")
)

// ErrCharacterNotFound means TibiaData knows no character with that name. It also matches ErrNotFound.
var ErrCharacterNotFound = fmt.Errorf("character %w", ErrNotFound)

package ports

import "errors"

var (
	// ErrNotFound means the row does not exist, or does not belong to the given guild.
	ErrNotFound = errors.New("not found")

	// ErrDuplicate means a unique constraint rejected the write.
	ErrDuplicate = errors.New("duplicate")

	// ErrConflict means the write overlaps an existing reservation.
	ErrConflict = errors.New("conflicting reservation")
)

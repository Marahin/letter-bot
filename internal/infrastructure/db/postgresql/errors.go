package postgresql

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"spot-assistant/internal/ports"
)

const (
	uniqueViolation    = "23505"
	exclusionViolation = "23P01"
)

// MapError translates driver errors into the ports sentinel errors, keeping the original as context.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolation:
			return fmt.Errorf("%w: %s", ports.ErrDuplicate, pgErr.Message)
		case exclusionViolation:
			return fmt.Errorf("%w: %s", ports.ErrConflict, pgErr.Message)
		}
	}

	return err
}

// RowsAffected turns "no row changed" into ports.ErrNotFound.
func RowsAffected(n int64, err error) error {
	if err != nil {
		return MapError(err)
	}
	if n == 0 {
		return ports.ErrNotFound
	}
	return nil
}

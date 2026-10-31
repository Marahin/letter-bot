package postgresql

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/ports"
)

func TestMapError(t *testing.T) {
	other := errors.New("boom")
	cases := map[string]struct {
		input    error
		expected error
	}{
		"nil":       {nil, nil},
		"no rows":   {pgx.ErrNoRows, ports.ErrNotFound},
		"unique":    {&pgconn.PgError{Code: "23505"}, ports.ErrDuplicate},
		"exclusion": {&pgconn.PgError{Code: "23P01"}, ports.ErrConflict},
		"other pg":  {&pgconn.PgError{Code: "42P01"}, nil},
		"other":     {other, other},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			result := MapError(tc.input)

			// then
			switch {
			case tc.input == nil:
				assert.NoError(t, result)
			case tc.expected == nil:
				assert.Equal(t, tc.input, result)
			default:
				assert.ErrorIs(t, result, tc.expected)
			}
		})
	}
}

func TestRowsAffected(t *testing.T) {
	// when / then
	assert.NoError(t, RowsAffected(1, nil))
	assert.ErrorIs(t, RowsAffected(0, nil), ports.ErrNotFound)
	assert.ErrorIs(t, RowsAffected(0, &pgconn.PgError{Code: "23505"}), ports.ErrDuplicate)
}

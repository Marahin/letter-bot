package postgresql

import (
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestDsn(t *testing.T) {
	// given: ensure defaults by setting known default values
	t.Setenv("DATABASE_HOST", "db")
	t.Setenv("DATABASE_PORT", "5432")
	t.Setenv("DATABASE_USER", "postgres")
	t.Setenv("DATABASE_PASSWORD", "postgres")
	t.Setenv("DATABASE_NAME", "name")

	// when
	output := Dsn()

	// Then
	snaps.MatchSnapshot(t, output)
}

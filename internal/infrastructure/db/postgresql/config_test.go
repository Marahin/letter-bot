package postgresql

import (
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestDsn(t *testing.T) {
	// when
	output := Dsn()

	// Then
	snaps.MatchSnapshot(t, output)
}

package postgresql

import (
	"os"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDsn(t *testing.T) {
	// given: ensure defaults by setting known default values
	t.Setenv("DATABASE_HOST", "db")
	t.Setenv("DATABASE_PORT", "5432")
	t.Setenv("DATABASE_USER", "postgres")
	t.Setenv("DATABASE_PASSWORD", "postgres")
	t.Setenv("DATABASE_NAME", "name")

	// when
	s, err := LoadConfig()
	require.NoError(t, err)
	output := s.DSN()

	// Then
	snaps.MatchSnapshot(t, output)
}

func TestDSN_UsesSSL(t *testing.T) {
	// given
	s := Specification{Host: "h", Port: 1, User: "u", Password: "p", Name: "n", SSL: "require"}

	// when
	dsn := s.DSN()

	// then
	assert.Equal(t, "host=h port=1 user=u password=p dbname=n sslmode=require", dsn)
}

func TestLoadConfig_Defaults(t *testing.T) {
	// given
	for _, name := range []string{"HOST", "PORT", "USER", "PASSWORD", "NAME", "SSL"} {
		t.Setenv("DATABASE_"+name, "")
		require.NoError(t, os.Unsetenv("DATABASE_"+name))
	}

	// when
	s, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Equal(t, Specification{Host: "db", Port: 5432, User: "postgres", Password: "postgres", Name: "name", SSL: "disable"}, s)
}

func TestLoadConfig_RejectsABadPort(t *testing.T) {
	// given
	t.Setenv("DATABASE_PORT", "x")

	// when
	_, err := LoadConfig()

	// then
	assert.Error(t, err)
}

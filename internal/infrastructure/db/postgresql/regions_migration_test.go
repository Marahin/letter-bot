package postgresql

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/regions"
)

var regionValue = regexp.MustCompile(`^\s*\('((?:[^']|'')*)', '([a-z0-9-]+)'\),?$`)

// The migration placed the default respawns once; ImportDefaults places them on
// each import. Both must agree, or an imported guild differs from a migrated one.
func TestMigrations_SpotRegionsMatchTheDefaultMap(t *testing.T) {
	// given
	body := readMigration(t, "20261018120000_spot_region_image.sql")
	parsed := map[string]string{}

	// when
	for line := range strings.Lines(body) {
		m := regionValue.FindStringSubmatch(strings.TrimRight(line, "\n"))
		if m == nil {
			continue
		}
		name := strings.ReplaceAll(m[1], "''", "'")
		_, dup := parsed[name]
		require.False(t, dup, "duplicate %q", name)
		parsed[name] = m[2]
	}

	// then
	assert.Equal(t, regions.DefaultSpotRegion, parsed)
}

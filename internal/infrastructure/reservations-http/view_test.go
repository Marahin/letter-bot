package reservationshttp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormView_ZoneNameFollowsTheStart(t *testing.T) {
	// given a process zone with DST, and a winter "now"
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	local := time.Local
	time.Local = berlin
	t.Cleanup(func() { time.Local = local })
	winter := time.Date(2026, 1, 15, 12, 0, 0, 0, berlin)

	// then the summer start names summer time, and an unreadable start falls back to now
	assert.Equal(t, "CEST", (&formView{Start: "2026-07-01T20:00"}).zoneName(winter))
	assert.Equal(t, "CET", (&formView{Start: "2026-12-01T20:00"}).zoneName(winter))
	assert.Equal(t, "CET", (&formView{Start: "soon"}).zoneName(winter))
}

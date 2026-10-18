package summarylayout

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLayout_Blocked(t *testing.T) {
	// given
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	later, earlier := now.Add(time.Minute), now.Add(-time.Minute)

	// when / then
	assert.False(t, Layout{}.Blocked(now))
	assert.True(t, Layout{BlockedUntil: &later}.Blocked(now))
	assert.False(t, Layout{BlockedUntil: &earlier}.Blocked(now))
}

func TestRegionChannel_Retired(t *testing.T) {
	// given
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	// when / then
	assert.False(t, RegionChannel{}.Retired())
	assert.True(t, RegionChannel{RetiredAt: &at}.Retired())
}

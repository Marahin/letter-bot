package regions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatalog_KeysAreUniqueAndFitTheChannelRules(t *testing.T) {
	// given
	seen := map[string]bool{Other.Key: true}

	// when / then
	assert.LessOrEqual(t, len(Catalog)+1, 50, "a Discord category holds at most 50 channels")
	for _, r := range Catalog {
		assert.False(t, seen[r.Key], "duplicate key %q", r.Key)
		seen[r.Key] = true
		assert.Regexp(t, KeyPattern, r.Key)
		assert.NotEmpty(t, r.Name)
		assert.LessOrEqual(t, len(ChannelName(r)), 100, "Discord channel names have at most 100 characters")
	}
	assert.Regexp(t, KeyPattern, Other.Key)
}

func TestLookupOfValid(t *testing.T) {
	// when
	portHope, found := Lookup("port-hope")
	_, unknown := Lookup("atlantis")
	other, otherFound := Lookup("other")

	// then
	assert.True(t, found)
	assert.Equal(t, "Port Hope & Tiquanda", portHope.Name)
	assert.False(t, unknown)
	assert.True(t, otherFound)
	assert.Equal(t, Other, other)

	assert.Equal(t, portHope, Of("port-hope"))
	assert.Equal(t, Other, Of(""))
	assert.Equal(t, Other, Of("atlantis"))

	assert.True(t, Valid("port-hope"))
	assert.False(t, Valid(""))
	assert.False(t, Valid("other"), "Other is stored as no region")
	assert.False(t, Valid("atlantis"))
}

func TestIndex_KeepsTheCatalogOrderWithOtherLast(t *testing.T) {
	assert.Equal(t, 0, Index(Catalog[0].Key))
	assert.Equal(t, len(Catalog)-1, Index(Catalog[len(Catalog)-1].Key))
	assert.Equal(t, len(Catalog), Index(Other.Key))
	assert.Equal(t, len(Catalog), Index("atlantis"))
}

func TestChannelName(t *testing.T) {
	assert.Equal(t, "letter-port-hope", ChannelName(Of("port-hope")))
	assert.Equal(t, "letter-other", ChannelName(Other))
}

func TestDefaultSpotRegion_EveryRegionHasMoreThanOneSpot(t *testing.T) {
	// given
	counts := map[string]int{}

	// when
	for _, key := range DefaultSpotRegion {
		counts[key]++
	}

	// then
	for key, n := range counts {
		assert.True(t, Valid(key), key)
		assert.Greater(t, n, 1, "region %s would be a one-respawn channel", key)
	}
}

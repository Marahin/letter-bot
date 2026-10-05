//go:build devauth

package devauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsers_HaveUniqueIDs(t *testing.T) {
	// given
	seen := map[string]bool{}

	// then
	for _, u := range Users {
		assert.False(t, seen[u.ID], "duplicate id %s", u.ID)
		seen[u.ID] = true
		assert.NotEmpty(t, u.Username)
		assert.NotEmpty(t, u.Label)
	}
}

func TestByID(t *testing.T) {
	// when
	manager, ok := ByID("700000000000000001")
	_, missing := ByID("1")

	// then
	assert.True(t, ok)
	assert.Equal(t, "dev-manager", manager.Username)
	assert.True(t, manager.memberOf(PremiumGuildID))
	assert.False(t, missing)
}

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

func TestIsDevBaseURL(t *testing.T) {
	for baseURL, want := range map[string]bool{
		"http://localhost:8080":          true,
		"http://127.0.0.1:8080":          true,
		"http://[::1]:8080":              true,
		"HTTP://LOCALHOST":               true,
		"https://tibialoot.com":          false,
		"http://localhost.attacker.test": false,
		"http://[::1":                    false,
	} {
		t.Run(baseURL, func(t *testing.T) {
			// when
			got := IsDevBaseURL(baseURL)

			// then
			assert.Equal(t, want, got)
		})
	}
}

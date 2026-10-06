package guildconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_IsPremium(t *testing.T) {
	cases := map[string]struct {
		cfg      Config
		expected bool
	}{
		"not premium":     {Config{}, false},
		"premium":         {Config{Premium: true}, true},
		"premium forever": {Config{PremiumForever: true}, true},
		"both":            {Config{Premium: true, PremiumForever: true}, true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			result := tc.cfg.IsPremium()

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConfig_RoleIDs(t *testing.T) {
	// given
	cfg := Config{
		ManageRoleIDs:   []string{"m"},
		ViewRoleIDs:     []string{"v"},
		ReserveRoleIDs:  []string{"r"},
		OverbookRoleIDs: []string{"o"},
	}

	// when / then
	assert.Equal(t, []string{"m"}, cfg.RoleIDs(RoleKindManage))
	assert.Equal(t, []string{"v"}, cfg.RoleIDs(RoleKindView))
	assert.Equal(t, []string{"r"}, cfg.RoleIDs(RoleKindReserve))
	assert.Equal(t, []string{"o"}, cfg.RoleIDs(RoleKindOverbook))
	assert.Nil(t, cfg.RoleIDs(RoleKind("other")))
}

func TestRoleKind_Valid(t *testing.T) {
	for _, kind := range RoleKinds {
		assert.True(t, kind.Valid(), kind)
	}
	assert.False(t, RoleKind("admin").Valid())
	assert.False(t, RoleKind("").Valid())
}

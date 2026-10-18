package access

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/permission"
)

func TestGuildMember_DisplayName(t *testing.T) {
	cases := map[string]struct {
		member   GuildMember
		expected string
	}{
		"nick first":           {GuildMember{Nick: "Nyx", GlobalName: "Quiet", Username: "q"}, "Nyx"},
		"global name":          {GuildMember{GlobalName: "Quiet", Username: "q"}, "Quiet"},
		"username as fallback": {GuildMember{Username: "q"}, "q"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			result := tc.member.DisplayName()

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGuildAccess_Allows(t *testing.T) {
	// given
	viewer := GuildAccess{Caps: permission.Capabilities{View: true}}
	reserver := GuildAccess{Caps: permission.Capabilities{View: true, Reserve: true}}
	manager := GuildAccess{Caps: permission.Capabilities{View: true, Reserve: true, Manage: true}}
	admin := GuildAccess{Caps: permission.Capabilities{Admin: true, View: true, Reserve: true, Manage: true}}

	// when / then
	assert.True(t, viewer.Allows(TierView))
	assert.False(t, viewer.Allows(TierReserve))
	assert.True(t, reserver.Allows(TierReserve))
	assert.False(t, reserver.Allows(TierManage))
	assert.True(t, manager.Allows(TierManage))
	assert.False(t, manager.Allows(TierAdmin))
	assert.True(t, admin.Allows(TierAdmin))
	assert.False(t, admin.Allows(Tier(0)))
}

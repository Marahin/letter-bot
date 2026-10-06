package permission

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
)

func TestResolve(t *testing.T) {
	postman := &role.Role{ID: "postman", Name: "Postman"}
	other := &role.Role{ID: "other", Name: "Other"}
	guildRoles := []*role.Role{nil, other, postman}
	configured := guildconfig.Config{
		ManageRoleIDs:   []string{"manage"},
		ViewRoleIDs:     []string{"view"},
		ReserveRoleIDs:  []string{"reserve"},
		OverbookRoleIDs: []string{"overbook"},
	}

	tests := []struct {
		name    string
		cfg     guildconfig.Config
		subject Subject
		want    Capabilities
	}{
		{
			name:    "no ranks configured: everyone may view and reserve",
			cfg:     guildconfig.Config{},
			subject: Subject{RoleIDs: []string{"other"}, GuildRoles: guildRoles},
			want:    Capabilities{View: true, Reserve: true},
		},
		{
			name:    "no ranks configured: Postman may overbook",
			cfg:     guildconfig.Config{},
			subject: Subject{RoleIDs: []string{"postman"}, GuildRoles: guildRoles},
			want:    Capabilities{View: true, Reserve: true, Overbook: true},
		},
		{
			name:    "admin gets everything",
			cfg:     configured,
			subject: Subject{IsAdmin: true},
			want:    Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true},
		},
		{
			name:    "manage rank gets everything but admin",
			cfg:     configured,
			subject: Subject{RoleIDs: []string{"manage"}},
			want:    Capabilities{Manage: true, View: true, Reserve: true, Overbook: true},
		},
		{
			name:    "view rank only views",
			cfg:     configured,
			subject: Subject{RoleIDs: []string{"view"}},
			want:    Capabilities{View: true},
		},
		{
			name:    "reserve rank views and reserves",
			cfg:     configured,
			subject: Subject{RoleIDs: []string{"reserve"}},
			want:    Capabilities{View: true, Reserve: true},
		},
		{
			name:    "overbook rank without reserve rank cannot reserve",
			cfg:     configured,
			subject: Subject{RoleIDs: []string{"overbook"}},
			want:    Capabilities{Overbook: true},
		},
		{
			name:    "configured overbook ranks replace the Postman role",
			cfg:     configured,
			subject: Subject{RoleIDs: []string{"reserve", "postman"}, GuildRoles: guildRoles},
			want:    Capabilities{View: true, Reserve: true},
		},
		{
			name:    "member without a rank gets nothing",
			cfg:     configured,
			subject: Subject{RoleIDs: []string{"other"}, GuildRoles: guildRoles},
			want:    Capabilities{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given / when
			got := Resolve(tt.cfg, tt.subject)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

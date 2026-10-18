package access

import (
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/permission"
)

// UserGuild is one guild from the signed-in user's own Discord guild list.
type UserGuild struct {
	ID   string
	Name string
	Icon string
	// Admin is true when the user owns the guild or holds the Administrator permission.
	Admin bool
}

// GuildMember is the signed-in user's member record in one guild.
type GuildMember struct {
	Nick       string
	GlobalName string
	Username   string
	RoleIDs    []string
}

// DisplayName returns the nick, then the global name, then the username.
func (m GuildMember) DisplayName() string {
	switch {
	case m.Nick != "":
		return m.Nick
	case m.GlobalName != "":
		return m.GlobalName
	}
	return m.Username
}

// Tier is the access level a guild page asks for.
type Tier int

const (
	TierView Tier = iota + 1
	TierReserve
	TierManage
	TierAdmin
)

// GuildAccess is what the signed-in user may do in one stored guild.
type GuildAccess struct {
	Config guildconfig.Config
	Caps   permission.Capabilities
}

// Allows reports whether the capabilities reach the tier. An unknown tier allows nothing.
func (a GuildAccess) Allows(t Tier) bool {
	switch t {
	case TierView:
		return a.Caps.View
	case TierReserve:
		return a.Caps.Reserve
	case TierManage:
		return a.Caps.Manage
	case TierAdmin:
		return a.Caps.Admin
	}
	return false
}

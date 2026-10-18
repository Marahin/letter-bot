package permission

import (
	"slices"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
)

// Subject is the member whose capabilities are resolved.
type Subject struct {
	// IsAdmin is true for the guild owner and for members with the Administrator permission.
	IsAdmin bool
	RoleIDs []string
	// GuildRoles are all roles of the guild. They are used to find the legacy Postman role by name.
	GuildRoles []*role.Role
}

type Capabilities struct {
	Admin    bool
	Manage   bool
	View     bool
	Reserve  bool
	Overbook bool
}

// Resolve applies the rank rules of the guild. The bot and the web use the same rules.
func Resolve(cfg guildconfig.Config, s Subject) Capabilities {
	c := Capabilities{Admin: s.IsAdmin}
	c.Manage = c.Admin || holdsAny(s.RoleIDs, cfg.ManageRoleIDs)
	c.Reserve = c.Manage || len(cfg.ReserveRoleIDs) == 0 || holdsAny(s.RoleIDs, cfg.ReserveRoleIDs)
	c.View = c.Manage || c.Reserve || holdsAny(s.RoleIDs, cfg.ViewRoleIDs)
	if len(cfg.OverbookRoleIDs) > 0 {
		c.Overbook = c.Manage || holdsAny(s.RoleIDs, cfg.OverbookRoleIDs)
	} else {
		c.Overbook = c.Manage || holdsRoleNamed(s.RoleIDs, s.GuildRoles, discord.PrivilegedRole)
	}

	return c
}

func holdsAny(held, wanted []string) bool {
	for _, id := range wanted {
		if slices.Contains(held, id) {
			return true
		}
	}
	return false
}

func holdsRoleNamed(held []string, guildRoles []*role.Role, name string) bool {
	for _, r := range guildRoles {
		if r != nil && r.Name == name && slices.Contains(held, r.ID) {
			return true
		}
	}
	return false
}

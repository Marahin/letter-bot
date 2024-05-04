package factories

import "spot-assistant/internal/core/dto/guild"

// CreateGuild creates a sample guild.
func CreateGuild() *guild.Guild {
	return &guild.Guild{
		ID:    "sample-guild-id",
		Name:  "sample-guild-name",
		Roles: nil,
	}
}

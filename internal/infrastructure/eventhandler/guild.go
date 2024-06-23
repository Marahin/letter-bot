package eventhandler

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/guild"
)

func (a *Handler) OnGuildCreate(guild *guild.Guild) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	g, err := a.guildSrv.CreateGuild(ctx, guild.ID, guild.Name)
	if err != nil {
		a.log.With("guild_id", guild.ID, "guild_name", guild.Name).Warn("OnGuildCreate: ", err)
		return
	}

	a.log.With("guild_id", g.ID, "guild_internal_id", g.InternalID, "guild_name", g.Name).Info("guild created")
}

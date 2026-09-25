package communication

import (
	"context"
	"errors"
	"strconv"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/ports"

	"spot-assistant/internal/core/dto/summary"
)

// @TODO: get rid of methods that suggest implementation:
// FindChannelByName
// OpenDM

func (a *Adapter) SendGuildSummary(guild *guild.Guild, summary *summary.Summary) error {
	summaryChannel, err := a.summaryChannel(guild)
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(guild, summaryChannel, summary)
}

// summaryChannel returns the configured summary channel, or the legacy #letter-summary channel when none is set.
func (a *Adapter) summaryChannel(g *guild.Guild) (*discord.Channel, error) {
	cfg, err := a.guildConfigs.Get(context.Background(), g.ID)
	if err != nil && !errors.Is(err, ports.ErrNotFound) {
		return nil, err
	}
	if cfg != nil && cfg.SummaryChannelID != "" {
		return a.bot.FindChannelById(g, cfg.SummaryChannelID)
	}

	return a.bot.FindChannelByName(g, discord.SummaryChannel)
}

func (a *Adapter) SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error {
	dmChannel, err := a.bot.OpenDM(&member.Member{ID: strconv.FormatInt(request.UserID, 10)})
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(nil, dmChannel, summary)
}

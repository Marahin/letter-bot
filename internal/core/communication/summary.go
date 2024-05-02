package communication

import (
	"strconv"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"

	"spot-assistant/internal/core/dto/summary"
)

// @TODO: get rid of methods that suggest implementation:
// FindChannelByName
// OpenDM

func (a *Adapter) SendGuildSummary(guild *guild.Guild, summary *summary.Summary) error {
	summaryChannel, err := a.bot.FindChannelByName(guild, discord.SummaryChannel)
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(guild, summaryChannel, summary)
}

func (a *Adapter) SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error {
	dmChannel, err := a.bot.OpenDM(&member.Member{ID: strconv.FormatInt(request.UserID, 10)})
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(nil, dmChannel, summary)
}

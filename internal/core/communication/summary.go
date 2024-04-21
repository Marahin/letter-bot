package communication

import (
	"strconv"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) SendGuildSummary(guild *discord.Guild, summary *summary.Summary) error {
	summaryChannel, err := a.bot.FindChannelByName(guild, "letter-summary")
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(guild, summaryChannel, summary)
}

func (a *Adapter) SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error {
	dmChannel, err := a.bot.OpenDM(&discord.Member{ID: strconv.FormatInt(request.UserID, 10)})
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(nil, dmChannel, summary)
}

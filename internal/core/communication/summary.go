package communication

import (
	"strconv"

	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) SendPrivateSummary(request summary.PrivateSummaryRequest, sum *summary.Summary) error {
	dmChannel, err := a.bot.OpenDM(&member.Member{ID: strconv.FormatInt(request.UserID, 10)})
	if err != nil {
		return err
	}

	return a.bot.SendLetterMessage(nil, dmChannel, sum)
}

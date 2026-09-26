package communication

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/summary"
)

func TestAdapter_SendPrivateSummary(t *testing.T) {
	// given
	assert := assert.New(t)
	var nilptrGuild *guild.Guild
	dmChannel := &discord.Channel{}
	request := summary.PrivateSummaryRequest{
		UserID: 123,
	}
	summary := &summary.Summary{}
	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("OpenDM", &member.Member{ID: strconv.FormatInt(request.UserID, 10)}).Return(dmChannel, nil).Once()
	botOperations.On("SendLetterMessage", nilptrGuild, dmChannel, summary).Return(nil).Once()
	adapter := NewAdapter(botOperations, nil)

	// when
	err := adapter.SendPrivateSummary(request, summary)

	// assert
	assert.Nil(err)
	botOperations.AssertExpectations(t)
}

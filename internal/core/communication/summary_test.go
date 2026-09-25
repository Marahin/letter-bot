package communication

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/ports"
)

func TestAdapter_SendGuildSummary_FallsBackToLegacyChannel(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &guild.Guild{ID: "g1"}
	summary := &summary.Summary{}
	summaryCh := &discord.Channel{}
	memberOperations := mocks.NewMockMemberRepository(t)
	guildConfigs := mocks.NewMockGuildConfigRepository(t)
	guildConfigs.On("Get", mocks.ContextMock, "g1").Return(&guildconfig.Config{GuildID: "g1"}, nil).Once()
	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("FindChannelByName", guild, discord.SummaryChannel).Return(summaryCh, nil).Once()
	botOperations.On("SendLetterMessage", guild, summaryCh, summary).Return(nil).Once()
	adapter := NewAdapter(botOperations, memberOperations, guildConfigs)

	// when
	err := adapter.SendGuildSummary(guild, summary)

	// assert
	assert.Nil(err)
	botOperations.AssertExpectations(t)
}

func TestAdapter_SendGuildSummary_FallsBackToLegacyChannelForUnknownGuild(t *testing.T) {
	// given
	guild := &guild.Guild{ID: "g1"}
	summary := &summary.Summary{}
	summaryCh := &discord.Channel{}
	guildConfigs := mocks.NewMockGuildConfigRepository(t)
	guildConfigs.On("Get", mocks.ContextMock, "g1").Return(nil, ports.ErrNotFound).Once()
	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("FindChannelByName", guild, discord.SummaryChannel).Return(summaryCh, nil).Once()
	botOperations.On("SendLetterMessage", guild, summaryCh, summary).Return(nil).Once()
	adapter := NewAdapter(botOperations, nil, guildConfigs)

	// when
	err := adapter.SendGuildSummary(guild, summary)

	// then
	assert.NoError(t, err)
}

func TestAdapter_SendGuildSummary_UsesConfiguredChannel(t *testing.T) {
	// given
	guild := &guild.Guild{ID: "g1"}
	summary := &summary.Summary{}
	summaryCh := &discord.Channel{ID: "c9"}
	guildConfigs := mocks.NewMockGuildConfigRepository(t)
	guildConfigs.On("Get", mocks.ContextMock, "g1").Return(&guildconfig.Config{GuildID: "g1", SummaryChannelID: "c9"}, nil).Once()
	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("FindChannelById", guild, "c9").Return(summaryCh, nil).Once()
	botOperations.On("SendLetterMessage", guild, summaryCh, summary).Return(nil).Once()
	adapter := NewAdapter(botOperations, nil, guildConfigs)

	// when
	err := adapter.SendGuildSummary(guild, summary)

	// then
	assert.NoError(t, err)
}

func TestAdapter_SendGuildSummary_ConfigError(t *testing.T) {
	// given
	guild := &guild.Guild{ID: "g1"}
	guildConfigs := mocks.NewMockGuildConfigRepository(t)
	guildConfigs.On("Get", mocks.ContextMock, "g1").Return(nil, errors.New("db down")).Once()
	adapter := NewAdapter(mocks.NewMockBotPort(t), nil, guildConfigs)

	// when
	err := adapter.SendGuildSummary(guild, &summary.Summary{})

	// then
	assert.EqualError(t, err, "db down")
}

func TestAdapter_SendGuildSummary_ChannelNotFound(t *testing.T) {
	// given
	guild := &guild.Guild{ID: "g1"}
	guildConfigs := mocks.NewMockGuildConfigRepository(t)
	guildConfigs.On("Get", mocks.ContextMock, "g1").Return(&guildconfig.Config{GuildID: "g1", SummaryChannelID: "gone"}, nil).Once()
	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("FindChannelById", guild, "gone").Return(nil, errors.New("not found")).Once()
	adapter := NewAdapter(botOperations, nil, guildConfigs)

	// when
	err := adapter.SendGuildSummary(guild, &summary.Summary{})

	// then
	assert.EqualError(t, err, "not found")
}

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
	adapter := NewAdapter(botOperations, nil, nil)

	// when
	err := adapter.SendPrivateSummary(request, summary)

	// assert
	assert.Nil(err)
	botOperations.AssertExpectations(t)

}

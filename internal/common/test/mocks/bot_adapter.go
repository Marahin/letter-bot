package mocks

import (
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/ports"
)

type MockBot struct {
	mock.Mock
}

func (m *MockBot) Run() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockBot) SendDMOverbookedNotification(mem *member.Member, req book.BookRequest, res *reservation.ClippedOrRemovedReservation) error {
	args := m.Called(mem, req, res)
	return args.Error(0)
}

func (m *MockBot) GetMemberByGuildAndId(g *guild.Guild, memberId string) (*member.Member, error) {
	args := m.Called(g, memberId)
	return args.Get(0).(*member.Member), args.Error(1)
}

func (m *MockBot) WithEventHandler(handler ports.APIPort) ports.BotPort {
	args := m.Called(handler)
	return args.Get(0).(*MockBot)
}

func (m *MockBot) ChannelMessages(g *guild.Guild, ch *discord.Channel, limit int) ([]*discord.Message, error) {
	args := m.Called(g, ch, limit)
	return args.Get(0).([]*discord.Message), args.Error(1)
}

func (m *MockBot) CleanChannel(g *guild.Guild, channel *discord.Channel) error {
	args := m.Called(g, channel)
	return args.Error(0)
}

func (m *MockBot) EnsureChannel(g *guild.Guild) error {
	args := m.Called(g)
	return args.Error(0)
}

func (m *MockBot) FindChannelByName(g *guild.Guild, channelName string) (*discord.Channel, error) {
	args := m.Called(g, channelName)

	return args.Get(0).(*discord.Channel), args.Error(1)
}

func (m *MockBot) EnsureRoles(g *guild.Guild) error {
	args := m.Called(g)
	return args.Error(0)
}

func (m *MockBot) GetGuilds() []*guild.Guild {
	args := m.Called()
	return args.Get(0).([]*guild.Guild)
}

func (m *MockBot) SendLetterMessage(g *guild.Guild, ch *discord.Channel, sum *summary.Summary) error {
	args := m.Called(g, ch, sum)
	return args.Error(0)
}

func (m *MockBot) RegisterCommands(g *guild.Guild) error {
	args := m.Called(g)
	return args.Error(0)
}

func (m *MockBot) MemberHasRole(g *guild.Guild, mem *member.Member, roleName string) bool {
	args := m.Called(g, mem, roleName)
	return args.Bool(0)
}

func (m *MockBot) OpenDM(mem *member.Member) (*discord.Channel, error) {
	args := m.Called(mem)
	return args.Get(0).(*discord.Channel), args.Error(1)
}

func (m *MockBot) StartTicking() {
	m.Called()
}

func (m *MockBot) SendChannelMessage(g *guild.Guild, ch *discord.Channel, content string) error {
	args := m.Called(g, ch, content)
	return args.Error(0)
}

func (m *MockBot) SendDM(mem *member.Member, msg string) error {
	args := m.Called(mem, msg)
	return args.Error(0)
}

func (m *MockBot) GetMember(g *guild.Guild, memberID string) (*member.Member, error) {
	args := m.Called(g, memberID)
	return args.Get(0).(*member.Member), args.Error(1)
}

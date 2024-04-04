package mocks

import (
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"
)

type MockBot struct {
	mock.Mock
}

func (m *MockBot) ChannelMessages(g *discord.Guild, ch *discord.Channel, limit int) ([]*discord.Message, error) {
	args := m.Called(g, ch, limit)
	return args.Get(0).([]*discord.Message), args.Error(1)
}

func (m *MockBot) CleanChannel(g *discord.Guild, channel *discord.Channel) error {
	args := m.Called(g, channel)
	return args.Error(0)
}

func (m *MockBot) EnsureChannel(g *discord.Guild) error {
	args := m.Called(g)
	return args.Error(0)
}

func (m *MockBot) FindChannel(g *discord.Guild, channelName string) (*discord.Channel, error) {
	args := m.Called(g, channelName)

	return args.Get(0).(*discord.Channel), args.Error(1)
}

func (m *MockBot) EnsureRoles(g *discord.Guild) error {
	args := m.Called(g)
	return args.Error(0)
}

func (m *MockBot) GetGuilds() []*discord.Guild {
	args := m.Called()
	return args.Get(0).([]*discord.Guild)
}

func (m *MockBot) SendLetterMessage(g *discord.Guild, ch *discord.Channel, sum *summary.Summary) error {
	args := m.Called(g, ch, sum)
	return args.Error(0)
}

func (m *MockBot) RegisterCommands(g *discord.Guild) error {
	args := m.Called(g)
	return args.Error(0)
}

func (m *MockBot) MemberHasRole(g *discord.Guild, mem *discord.Member, roleName string) bool {
	args := m.Called(g, mem, roleName)
	return args.Bool(0)
}

func (m *MockBot) OpenDM(mem *discord.Member) (*discord.Channel, error) {
	args := m.Called(mem)
	return args.Get(0).(*discord.Channel), args.Error(1)
}

func (m *MockBot) StartTicking() {
	m.Called()
}

func (m *MockBot) SendChannelMessage(g *discord.Guild, ch *discord.Channel, content string) error {
	args := m.Called(g, ch, content)
	return args.Error(0)
}

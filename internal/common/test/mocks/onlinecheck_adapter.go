package mocks

import (
	"spot-assistant/internal/core/dto/summary"

	"github.com/stretchr/testify/mock"
)

type MockOnlineCheckService struct {
	mock.Mock
}

func (m *MockOnlineCheckService) IsOnline(guildID, characterName string) bool {
	args := m.Called(guildID, characterName)
	return args.Bool(0)
}

func (m *MockOnlineCheckService) PlayerStatus(guildID, characterName string) summary.OnlineStatus {
	args := m.Called(guildID, characterName)
	return args.Get(0).(summary.OnlineStatus)
}

func (m *MockOnlineCheckService) RefreshOnlinePlayers(guildID string) error {
	args := m.Called(guildID)
	return args.Error(0)
}

func (m *MockOnlineCheckService) IsConfigured() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockOnlineCheckService) TryRefresh(guildID string) {
	m.Called(guildID)
}

func (m *MockOnlineCheckService) ConfigureWorldName(guildID, world string) {
	m.Called(guildID, world)
}

package mocks

import "github.com/stretchr/testify/mock"

type MockOnlineCheckService struct {
	mock.Mock
}

func (m *MockOnlineCheckService) IsOnline(characterName string) bool {
	args := m.Called(characterName)
	return args.Bool(0)
}

func (m *MockOnlineCheckService) RefreshOnlinePlayers() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockOnlineCheckService) IsConfigured() bool {
	args := m.Called()
	return args.Bool(0)
}

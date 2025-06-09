package mocks

import "github.com/stretchr/testify/mock"

type MockOnlineCheckService struct {
	mock.Mock
}

func (m *MockOnlineCheckService) IsOnline(characterName string) bool {
	args := m.Called(characterName)
	return args.Bool(0)
}

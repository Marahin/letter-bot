package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/core/dto/guild"
)

type MockGuildRepository struct {
	mock.Mock
}

func (m *MockGuildRepository) SelectGuilds(ctx context.Context) ([]*guild.Guild, error) {
	args := m.Called(ctx)

	return args.Get(0).([]*guild.Guild), args.Error(1)
}

func (m *MockGuildRepository) CreateGuild(ctx context.Context, id, name string) (*guild.Guild, error) {
	args := m.Called(ctx, id, name)

	return args.Get(0).(*guild.Guild), args.Error(1)
}

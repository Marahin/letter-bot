package mocks

import (
	"context"
	"spot-assistant/internal/core/dto/guildsworld"
)

type MockWorldNameRepository struct {
	UpsertCalled  bool
	UpsertErr     error
	UpsertGuildID string
	UpsertWorld   string
	SelectWorld   *guildsworld.GuildsWorld
	SelectErr     error
}

func (m *MockWorldNameRepository) UpsertGuildWorld(ctx context.Context, guildID string, worldName string) error {
	m.UpsertCalled = true
	m.UpsertGuildID = guildID
	m.UpsertWorld = worldName
	if m.UpsertErr != nil {
		return m.UpsertErr
	}
	return nil
}

func (m *MockWorldNameRepository) SelectGuildWorld(ctx context.Context, guildID string) (*guildsworld.GuildsWorld, error) {
	if m.SelectErr != nil {
		return nil, m.SelectErr
	}
	if m.SelectWorld != nil {
		return m.SelectWorld, nil
	}
	return nil, nil
}

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/core/dto/spot"
)

type MockSpotRepo struct {
	mock.Mock
}

func (a *MockSpotRepo) SelectAllSpots(ctx context.Context) ([]*spot.Spot, error) {
	args := a.Called(ctx)
	return args.Get(0).([]*spot.Spot), args.Error(1)
}

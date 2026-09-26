package worldapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/ports"
)

func newCached(t *testing.T) (*CachedCharacters, *mocks.MockCharacterAPI, *time.Time) {
	inner := mocks.NewMockCharacterAPI(t)
	c := NewCachedCharacters(inner)
	clock := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return clock }
	return c, inner, &clock
}

func TestCachedCharacters_ServesAHitUntilTheTTLEnds(t *testing.T) {
	// given
	c, inner, clock := newCached(t)
	inner.EXPECT().GetCharacter(mock.Anything, "Quiet Nyx").Return(&character.Character{Name: "Quiet Nyx", Level: 1}, nil).Once()

	// when
	first, err1 := c.GetCharacter(context.Background(), "Quiet Nyx")
	second, err2 := c.GetCharacter(context.Background(), " quiet nyx ")

	// then
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Same(t, first, second)

	// given
	*clock = clock.Add(CharacterCacheTTL)
	inner.EXPECT().GetCharacter(mock.Anything, "Quiet Nyx").Return(&character.Character{Name: "Quiet Nyx", Level: 2}, nil).Once()

	// when
	third, err := c.GetCharacter(context.Background(), "Quiet Nyx")

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, third.Level)
}

func TestCachedCharacters_CachesNotFoundButNotUpstreamErrors(t *testing.T) {
	// given
	c, inner, _ := newCached(t)
	inner.EXPECT().GetCharacter(mock.Anything, "Ghost").Return(nil, ports.ErrCharacterNotFound).Once()
	inner.EXPECT().GetCharacter(mock.Anything, "Flaky").Return(nil, ports.ErrUpstreamUnavailable).Twice()

	// when
	_, errA := c.GetCharacter(context.Background(), "Ghost")
	_, errB := c.GetCharacter(context.Background(), "Ghost")
	_, errC := c.GetCharacter(context.Background(), "Flaky")
	_, errD := c.GetCharacter(context.Background(), "Flaky")

	// then
	assert.ErrorIs(t, errA, ports.ErrCharacterNotFound)
	assert.ErrorIs(t, errB, ports.ErrCharacterNotFound)
	assert.ErrorIs(t, errC, ports.ErrUpstreamUnavailable)
	assert.ErrorIs(t, errD, ports.ErrUpstreamUnavailable)
}

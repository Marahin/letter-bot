package worldapi

import (
	"context"
	"errors"
	"maps"
	"slices"
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

func TestCachedCharacters_CachesNotFoundAndUpstreamFailuresBriefly(t *testing.T) {
	// given
	c, inner, clock := newCached(t)
	inner.EXPECT().GetCharacter(mock.Anything, "Ghost").Return(nil, ports.ErrCharacterNotFound).Once()
	inner.EXPECT().GetCharacter(mock.Anything, "Flaky").Return(nil, ports.ErrUpstreamUnavailable).Once()

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

	// given
	*clock = clock.Add(UpstreamFailureTTL)
	inner.EXPECT().GetCharacter(mock.Anything, "Flaky").Return(&character.Character{Name: "Flaky"}, nil).Once()

	// when
	ch, err := c.GetCharacter(context.Background(), "Flaky")

	// then
	require.NoError(t, err)
	assert.Equal(t, "Flaky", ch.Name)
}

func TestCachedCharacters_DoesNotCacheOtherErrorsOrACancelledRequest(t *testing.T) {
	// given
	c, inner, _ := newCached(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	inner.EXPECT().GetCharacter(mock.Anything, "Broken").Return(nil, errors.New("decode")).Twice()
	inner.EXPECT().GetCharacter(mock.Anything, "Gone").Return(nil, ports.ErrUpstreamUnavailable).Twice()

	// when
	_, err1 := c.GetCharacter(context.Background(), "Broken")
	_, err2 := c.GetCharacter(context.Background(), "Broken")
	_, err3 := c.GetCharacter(cancelled, "Gone")
	_, err4 := c.GetCharacter(context.Background(), "Gone")

	// then
	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.ErrorIs(t, err3, ports.ErrUpstreamUnavailable)
	assert.ErrorIs(t, err4, ports.ErrUpstreamUnavailable)
}

func TestCachedCharacters_BoundsTheLookup(t *testing.T) {
	// given
	c, inner, _ := newCached(t)
	inner.EXPECT().GetCharacter(mock.Anything, "Slow").RunAndReturn(func(ctx context.Context, _ string) (*character.Character, error) {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(CharacterLookupTimeout), deadline, time.Second)
		return &character.Character{}, nil
	})

	// when
	_, err := c.GetCharacter(context.Background(), "Slow")

	// then
	assert.NoError(t, err)
}

func TestCachedCharacters_DropsTheEntryThatExpiresFirstWhenFull(t *testing.T) {
	// given a cache of two, filled a minute apart
	c, inner, clock := newCached(t)
	c.maxEntries = 2
	for _, name := range []string{"Old", "Newer", "Newest"} {
		inner.EXPECT().GetCharacter(mock.Anything, name).Return(&character.Character{Name: name}, nil).Once()
	}
	_, err := c.GetCharacter(context.Background(), "Old")
	require.NoError(t, err)
	*clock = clock.Add(time.Minute)
	_, err = c.GetCharacter(context.Background(), "Newer")
	require.NoError(t, err)

	// when
	_, err = c.GetCharacter(context.Background(), "Newest")

	// then
	require.NoError(t, err)
	assert.Len(t, c.entries, 2)
	assert.NotContains(t, c.entries, "old")
	assert.Contains(t, c.entries, "newer")
	assert.Contains(t, c.entries, "newest")
}

func TestCachedCharacters_DropsExpiredEntriesBeforeEvicting(t *testing.T) {
	// given a full cache whose only entry has expired
	c, inner, clock := newCached(t)
	c.maxEntries = 1
	inner.EXPECT().GetCharacter(mock.Anything, "Old").Return(&character.Character{Name: "Old"}, nil).Once()
	inner.EXPECT().GetCharacter(mock.Anything, "New").Return(&character.Character{Name: "New"}, nil).Once()
	_, err := c.GetCharacter(context.Background(), "Old")
	require.NoError(t, err)
	*clock = clock.Add(CharacterCacheTTL)

	// when
	_, err = c.GetCharacter(context.Background(), "New")

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, slices.Collect(maps.Keys(c.entries)))
}

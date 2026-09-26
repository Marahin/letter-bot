package worldapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/ports"
)

// CharacterCacheTTL matches how often TibiaData refreshes a character page.
const CharacterCacheTTL = 5 * time.Minute

type cachedCharacter struct {
	character *character.Character
	err       error
	expires   time.Time
}

// CachedCharacters keeps TibiaData character answers (found and not found) in memory for
// CharacterCacheTTL. Upstream failures are not cached.
type CachedCharacters struct {
	inner ports.CharacterAPI
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[string]cachedCharacter
}

func NewCachedCharacters(inner ports.CharacterAPI) *CachedCharacters {
	return &CachedCharacters{inner: inner, ttl: CharacterCacheTTL, now: time.Now, entries: map[string]cachedCharacter{}}
}

func (c *CachedCharacters) GetCharacter(ctx context.Context, name string) (*character.Character, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	now := c.now()
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && now.Before(e.expires) {
		return e.character, e.err
	}

	ch, err := c.inner.GetCharacter(ctx, name)
	if err != nil && !errors.Is(err, ports.ErrCharacterNotFound) {
		return nil, err
	}
	c.mu.Lock()
	for k, old := range c.entries {
		if !now.Before(old.expires) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = cachedCharacter{character: ch, err: err, expires: now.Add(c.ttl)}
	c.mu.Unlock()
	return ch, err
}

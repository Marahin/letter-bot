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

const (
	// CharacterCacheTTL matches how often TibiaData refreshes a character page.
	CharacterCacheTTL = 5 * time.Minute
	// UpstreamFailureTTL keeps every character view from waiting on a TibiaData that does not answer.
	UpstreamFailureTTL = 30 * time.Second
	// CharacterLookupTimeout is shorter than the HTTP client's: the character page renders without the profile.
	CharacterLookupTimeout = 5 * time.Second
)

type cachedCharacter struct {
	character *character.Character
	err       error
	expires   time.Time
}

// CachedCharacters keeps TibiaData character answers (found and not found) in memory for
// CharacterCacheTTL, and upstream failures for UpstreamFailureTTL.
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

	lookupCtx, cancel := context.WithTimeout(ctx, CharacterLookupTimeout)
	defer cancel()
	ch, err := c.inner.GetCharacter(lookupCtx, name)
	ttl := c.ttl
	switch {
	case err == nil || errors.Is(err, ports.ErrCharacterNotFound):
	case errors.Is(err, ports.ErrUpstreamUnavailable) && ctx.Err() == nil:
		ttl = UpstreamFailureTTL
	default:
		return nil, err
	}
	c.mu.Lock()
	for k, old := range c.entries {
		if !now.Before(old.expires) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = cachedCharacter{character: ch, err: err, expires: now.Add(ttl)}
	c.mu.Unlock()
	return ch, err
}

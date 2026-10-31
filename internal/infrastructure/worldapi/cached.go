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
	// CharacterCacheMaxEntries bounds the memory a flood of distinct names can take.
	CharacterCacheMaxEntries = 1000
)

type cachedCharacter struct {
	character *character.Character
	err       error
	expires   time.Time
}

// CachedCharacters keeps TibiaData character answers (found and not found) in memory for
// CharacterCacheTTL, and upstream failures for UpstreamFailureTTL. When full, it drops the entry
// that expires first.
type CachedCharacters struct {
	inner      ports.CharacterAPI
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]cachedCharacter
}

func NewCachedCharacters(inner ports.CharacterAPI) *CachedCharacters {
	return &CachedCharacters{
		inner:      inner,
		ttl:        CharacterCacheTTL,
		maxEntries: CharacterCacheMaxEntries,
		now:        time.Now,
		entries:    map[string]cachedCharacter{},
	}
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
	c.store(key, cachedCharacter{character: ch, err: err, expires: now.Add(ttl)}, now)
	return ch, err
}

func (c *CachedCharacters) store(key string, e cachedCharacter, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, old := range c.entries {
		if !now.Before(old.expires) {
			delete(c.entries, k)
		}
	}
	if _, ok := c.entries[key]; !ok && len(c.entries) >= c.maxEntries {
		c.evictFirstToExpire()
	}
	c.entries[key] = e
}

func (c *CachedCharacters) evictFirstToExpire() {
	var (
		first        string
		firstExpires time.Time
		found        bool
	)
	for k, e := range c.entries {
		if !found || e.expires.Before(firstExpires) {
			first, firstExpires, found = k, e.expires, true
		}
	}
	delete(c.entries, first)
}

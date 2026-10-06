package oauth

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

// Discord rate-limits these endpoints hard, and every guild page resolves access,
// so both reads are cached per user for this long.
const (
	userGuildsTTL = 5 * time.Minute
	memberTTL     = 5 * time.Minute
	// pruneAbove bounds the maps: past this size, expired entries are dropped on write.
	pruneAbove = 1000
)

// Caching decorates an OAuthPort with per-user caches of the guild list and the
// guild member records. When a refetch fails and a stale entry exists, the stale
// entry is served, so a short Discord outage does not lock users out. Errors are
// not cached.
type Caching struct {
	inner ports.OAuthPort
	log   *zap.SugaredLogger
	now   func() time.Time

	mu      sync.Mutex
	guilds  map[string]cachedGuilds
	members map[memberKey]cachedMember
}

type cachedGuilds struct {
	guilds  []access.UserGuild
	expires time.Time
}

type memberKey struct{ userID, guildID string }

type cachedMember struct {
	member  *access.GuildMember
	expires time.Time
}

func NewCaching(inner ports.OAuthPort, log *zap.SugaredLogger) *Caching {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Caching{
		inner:   inner,
		log:     log,
		now:     time.Now,
		guilds:  make(map[string]cachedGuilds),
		members: make(map[memberKey]cachedMember),
	}
}

func (c *Caching) AuthCodeURL(state string) string { return c.inner.AuthCodeURL(state) }

// Exchange drops the user's cached entries, so a fresh sign-in sees fresh guilds and roles.
func (c *Caching) Exchange(ctx context.Context, code string) (*webuser.User, error) {
	user, err := c.inner.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	delete(c.guilds, user.DiscordUserID)
	for k := range c.members {
		if k.userID == user.DiscordUserID {
			delete(c.members, k)
		}
	}
	c.mu.Unlock()
	return user, nil
}

func (c *Caching) UserGuilds(ctx context.Context, userID string) ([]access.UserGuild, error) {
	now := c.now()
	c.mu.Lock()
	stale, hasStale := c.guilds[userID]
	c.mu.Unlock()
	if hasStale && now.Before(stale.expires) {
		return stale.guilds, nil
	}

	guilds, err := c.inner.UserGuilds(ctx, userID)
	if err != nil {
		if hasStale && servesStale(err) {
			c.log.Warnw("serving stale guild list", "error", err)
			return stale.guilds, nil
		}
		return nil, err
	}
	c.mu.Lock()
	if len(c.guilds) > pruneAbove {
		for k, v := range c.guilds {
			if !now.Before(v.expires) {
				delete(c.guilds, k)
			}
		}
	}
	c.guilds[userID] = cachedGuilds{guilds: guilds, expires: now.Add(userGuildsTTL)}
	c.mu.Unlock()
	return guilds, nil
}

func (c *Caching) UserGuildMember(ctx context.Context, userID, guildID string) (*access.GuildMember, error) {
	now := c.now()
	key := memberKey{userID: userID, guildID: guildID}
	c.mu.Lock()
	stale, hasStale := c.members[key]
	c.mu.Unlock()
	if hasStale && now.Before(stale.expires) {
		return stale.member, nil
	}

	member, err := c.inner.UserGuildMember(ctx, userID, guildID)
	if err != nil {
		if hasStale && servesStale(err) {
			c.log.Warnw("serving stale guild member", "guild_id", guildID, "error", err)
			return stale.member, nil
		}
		return nil, err
	}
	c.mu.Lock()
	if len(c.members) > pruneAbove {
		for k, v := range c.members {
			if !now.Before(v.expires) {
				delete(c.members, k)
			}
		}
	}
	c.members[key] = cachedMember{member: member, expires: now.Add(memberTTL)}
	c.mu.Unlock()
	return member, nil
}

// servesStale is false for a refused token and for a left guild: serving the old
// entry there would keep access Discord already took away.
func servesStale(err error) bool {
	return !errorsIsAny(err, ports.ErrUnauthorized, ports.ErrNotFound)
}

func errorsIsAny(err error, targets ...error) bool {
	for _, t := range targets {
		if errors.Is(err, t) {
			return true
		}
	}
	return false
}

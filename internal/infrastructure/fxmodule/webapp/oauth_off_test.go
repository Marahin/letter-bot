//go:build !devauth

package webapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/web"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
)

func TestNewOAuthPort_RealInAProductionBuild(t *testing.T) {
	// given the flag on, which a production build ignores
	pool := lazyPool(t)
	users := webusersqlc.NewWebUserRepository(pool)
	cfg := web.Config{BaseURL: "http://localhost:8080", DevAuth: true}
	discord := newDiscordOAuth(cfg, users, zap.NewNop().Sugar())

	// when
	port := newOAuthPort(cfg, discord, guildsqlc.NewGuildConfigRepository(pool), users)

	// then
	assert.Same(t, discord, port)
}

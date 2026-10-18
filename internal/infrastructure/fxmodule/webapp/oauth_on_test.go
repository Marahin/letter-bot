//go:build devauth

package webapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/devauth"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/web"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
)

func TestNewOAuthPort_MockOnlyWhenDevLoginIsOn(t *testing.T) {
	for name, tc := range map[string]struct {
		devAuth bool
		baseURL string
		mock    bool
	}{
		"dev login on":    {devAuth: true, baseURL: "http://localhost:8080", mock: true},
		"flag off":        {devAuth: false, baseURL: "http://localhost:8080"},
		"real origin":     {devAuth: true, baseURL: "https://tibialoot.com"},
		"look-alike host": {devAuth: true, baseURL: "http://localhost.attacker.test"},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			pool := lazyPool(t)
			users := webusersqlc.NewWebUserRepository(pool)
			cfg := web.Config{BaseURL: tc.baseURL, DevAuth: tc.devAuth}
			discord := newDiscordOAuth(cfg, users, zap.NewNop().Sugar())

			// when
			port := newOAuthPort(cfg, discord, guildsqlc.NewGuildConfigRepository(pool), users)

			// then
			if tc.mock {
				assert.IsType(t, &devauth.MockOAuth{}, port)
			} else {
				assert.Same(t, discord, port, "the real port refuses dev-login codes")
			}
		})
	}
}

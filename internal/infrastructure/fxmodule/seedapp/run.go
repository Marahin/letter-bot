//go:build devauth

package seedapp

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/devauth"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
)

type runParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Pool       *pgxpool.Pool
	Configs    *guildsqlc.GuildConfigRepository
	Roles      *guildsqlc.GuildRoleRepository
	Log        *zap.SugaredLogger
}

// run seeds on start, then stops the app. A failed seed fails the start (exit 1).
func run(p runParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := devauth.Seed(ctx, p.Pool, p.Configs, p.Roles, time.Now()); err != nil {
				return fmt.Errorf("seed: %w", err)
			}
			p.Log.Infow("dev seed done", "premium_guild", devauth.PremiumGuildID, "locked_guild", devauth.LockedGuildID)
			return p.Shutdowner.Shutdown()
		},
	})
}

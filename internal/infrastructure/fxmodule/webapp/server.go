package webapp

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/core/experience"
	experiencesqlc "spot-assistant/internal/infrastructure/experience/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/fxmodule"
	"spot-assistant/internal/infrastructure/scheduler"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/infrastructure/worldapi"
)

const (
	shutdownTimeout = 10 * time.Second
	// experienceJobLockKey is the pg_advisory_lock key of the experience job.
	experienceJobLockKey int64 = 7419001
)

// serve runs the web server for the app's lifetime. A busy port fails the
// start; a server that fails later stops the app with exit code 1.
func serve(lc fx.Lifecycle, shutdowner fx.Shutdowner, server *web.Server, log *zap.SugaredLogger) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			serveFn, err := server.Listen()
			if err != nil {
				return fmt.Errorf("web server listen: %w", err)
			}
			go func() {
				if err := serveFn(); err != nil {
					log.Errorw("web server failed", "error", err)
					_ = shutdowner.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("shutting down")
			ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Errorw("shutdown error", "error", err)
			}
			return nil
		},
	})
}

type experienceJobParams struct {
	fx.In

	Cfg       web.Config
	WorldAPI  worldapi.Config
	API       *worldapi.HTTPWorldService
	Repo      *experiencesqlc.ExperienceRepository
	Pool      *pgxpool.Pool
	Lifecycle fx.Lifecycle
	Log       *zap.SugaredLogger
}

func runExperienceJob(p experienceJobParams) {
	switch {
	case !p.Cfg.ExperienceJobEnabled:
		p.Log.Warn("Experience job is disabled: WEB_EXPERIENCE_JOB_ENABLED=false")
		return
	case p.WorldAPI.BaseURL == "":
		p.Log.Warn("Experience job is disabled: TIBIA_WORLD_API_BASE_URL not set")
		return
	}
	job := experience.New(p.API, p.Repo, p.Cfg.ExperienceJobInterval, p.Log)
	lock := scheduler.NewPgAdvisoryLock(p.Pool, experienceJobLockKey, p.Log)
	run := func(ctx context.Context) error { return job.RunOnce(ctx, time.Now()) }
	p.Lifecycle.Append(fxmodule.LoopHook(scheduler.New("experience", run, lock, p.Cfg.ExperienceJobInterval, p.Log).Run))
}

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"spot-assistant/internal/common/version"
	"spot-assistant/internal/core/auth"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/characters"
	"spot-assistant/internal/core/experience"
	"spot-assistant/internal/core/guildaccess"
	"spot-assistant/internal/core/premium"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/core/spots"
	corestats "spot-assistant/internal/core/stats"

	adminhttp "spot-assistant/internal/infrastructure/admin-http"
	channelshttp "spot-assistant/internal/infrastructure/channels-http"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/discord/oauth"
	experienceRepository "spot-assistant/internal/infrastructure/experience/postgresql/sqlc"
	guildRepository "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/guildsettings"
	infrahttp "spot-assistant/internal/infrastructure/http"
	notifypg "spot-assistant/internal/infrastructure/notify/postgresql"
	"spot-assistant/internal/infrastructure/notify/webcomm"
	reservationRepository "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	reservationshttp "spot-assistant/internal/infrastructure/reservations-http"
	"spot-assistant/internal/infrastructure/scheduler"
	settingshttp "spot-assistant/internal/infrastructure/settings-http"
	spotRepository "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
	spotshttp "spot-assistant/internal/infrastructure/spots-http"
	statshttp "spot-assistant/internal/infrastructure/stats-http"
	statsRepository "spot-assistant/internal/infrastructure/stats/postgresql/sqlc"
	toolshttp "spot-assistant/internal/infrastructure/tools-http"
	"spot-assistant/internal/infrastructure/web"
	webUserRepository "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/worldapi"
	worldNameRepository "spot-assistant/internal/infrastructure/worldname/postgresql/sqlc"
)

// experienceJobLockKey is the pg_advisory_lock key of the experience job.
const experienceJobLockKey int64 = 7419001

func main() {
	logger, _ := zap.NewProduction()
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)
	log := logger.Sugar()
	log.Warn("Version ", version.Version,
		" - Starting web with TZ: ", time.Now().Location())

	cfg, err := web.LoadConfig()
	if err != nil {
		log.Panic(err)
	}

	// Database
	dbConfig, err := pgxpool.ParseConfig(postgresql.Dsn())
	if err != nil {
		log.Panic(err)
	}
	db, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()
	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.Ping(timeout); err != nil {
		log.Panic(err)
	}
	cancel()

	ready := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.Ping(ctx)
	}
	infrahttp.NewServerWithMetrics(cfg.MetricsAddr, log).WithHealth(nil, ready).Start()

	guildConfigRepo := guildRepository.NewGuildConfigRepository(db)
	guildRoleRepo := guildRepository.NewGuildRoleRepository(db)
	guildChannelRepo := guildRepository.NewGuildChannelRepository(db)
	worldNameRepo := worldNameRepository.NewWorldNameRepository(db)
	webUserRepo := webUserRepository.NewWebUserRepository(db)
	spotRepo := spotRepository.NewSpotRepository(db)
	reservationRepo := reservationRepository.NewReservationRepository(db).WithLogger(log)
	botNotifier := notifypg.NewNotifier(db)
	statsRepo, err := statsRepository.NewStatsRepository(db, os.Getenv("TZ"))
	if err != nil {
		log.Panic(err)
	}
	experienceRepo := experienceRepository.NewExperienceRepository(db)
	tibiaDataBaseURL := os.Getenv("TIBIA_WORLD_API_BASE_URL")
	tibiaData := worldapi.NewHttpWorldService(tibiaDataBaseURL)
	bookingService := booking.NewAdapter(spotRepo, reservationRepo, webcomm.New(botNotifier, log)).WithLogger(log)

	discordOAuth := oauth.NewCaching(oauth.New(cfg.Discord.ClientID, cfg.Discord.ClientSecret, cfg.CallbackURL(), webUserRepo), log)

	server := web.NewServer(cfg, log, db).WithServices(web.Services{
		Auth:         auth.New(discordOAuth, webUserRepo),
		Access:       guildaccess.New(discordOAuth, guildConfigRepo, guildRoleRepo, cfg.AdminDiscordIDs, log),
		Premium:      premium.New(guildConfigRepo, botNotifier, log),
		Settings:     guildsettings.New(guildConfigRepo, guildChannelRepo, guildRoleRepo, worldNameRepo, botNotifier, log),
		Spots:        spots.New(spotRepo, botNotifier, log),
		Reservations: reservations.New(bookingService, reservationRepo, spotRepo, botNotifier, log),
		Stats:        corestats.New(statsRepo, spotRepo),
		Characters:   characters.New(worldapi.NewCachedCharacters(tibiaData), worldNameRepo, experienceRepo, statsRepo, log),
	})
	server.Mount(adminhttp.Register)
	server.Mount(settingshttp.Register)
	server.Mount(channelshttp.Register)
	server.Mount(spotshttp.Register)
	server.Mount(reservationshttp.Register)
	server.Mount(statshttp.Register)
	server.Mount(toolshttp.Register)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch {
	case !cfg.ExperienceJobEnabled:
		log.Warn("Experience job is disabled: WEB_EXPERIENCE_JOB_ENABLED=false")
	case tibiaDataBaseURL == "":
		log.Warn("Experience job is disabled: TIBIA_WORLD_API_BASE_URL not set")
	default:
		job := experience.New(tibiaData, experienceRepo, cfg.ExperienceJobInterval, log)
		lock := scheduler.NewPgAdvisoryLock(db, experienceJobLockKey, log)
		run := func(ctx context.Context) error { return job.RunOnce(ctx, time.Now()) }
		go scheduler.New("experience", run, lock, cfg.ExperienceJobInterval, log).Run(ctx)
	}

	go func() {
		if err := server.Start(); err != nil {
			log.Errorw("web server stopped", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorw("web shutdown", "error", err)
	}
}

package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"spot-assistant/internal/common/version"

	"spot-assistant/internal/infrastructure/db/postgresql"
	infrahttp "spot-assistant/internal/infrastructure/http"
	"spot-assistant/internal/infrastructure/web"
)

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

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", web.HealthzHandler(db.Ping))
	mux.Handle("GET /assets/", web.AssetHandler())
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Infow("web listening", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

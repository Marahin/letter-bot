package main

import (
	"context"
	"net/http"
	"time"

	"github.com/brpaz/echozap"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	auth_rest "spot-assistant/internal/infrastructure/auth-rest"
	"spot-assistant/internal/infrastructure/db/postgresql"
	guildRepository "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}(logger) // flushes buffer, if any
	log := logger.Sugar()

	config, err := pgxpool.ParseConfig(postgresql.Dsn())
	if err != nil {
		log.Panic(err)
	}
	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()
	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.Ping(timeout); err != nil {
		log.Panic(err)
	}
	cancel()

	// Services
	guildRepo := guildRepository.NewGuildRepository(db).WithLogger(log)
	authRest := auth_rest.NewRestAuth(guildRepo)

	e := echo.New()
	e.Use(echozap.ZapLogger(logger))
	e.Use(session.Middleware(authRest.Store))
	e.GET("/", func(c echo.Context) error {
		// Step 1: Redirect to the OAuth 2.0 Authorization page.
		// This route could be named /login etc
		return c.Redirect(http.StatusTemporaryRedirect, "/auth/discord")
	})
	e.GET("/auth/:provider", authRest.ProviderHandler)
	e.GET("/auth/:provider/callback", authRest.ProviderCallbackHandler)

	e.Logger.Fatal(e.Start(":8080"))
}

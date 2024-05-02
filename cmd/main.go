package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/communication"
	"spot-assistant/internal/core/summary"

	"spot-assistant/internal/common/version"

	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/infrastructure/chart"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/eventhandler"
	reservationRepository "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	spotRepository "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
)

func main() {
	// Logger setup
	logger, _ := zap.NewProduction()
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}(logger) // flushes buffer, if any
	log := logger.Sugar()
	log.Warn("Version ", version.Version,
		" - Starting with TZ: ", time.Now().Location())

	// Database
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

	// Summary
	charter := chart.NewAdapter()
	summaryService := summary.NewAdapter(charter) // .WithLogger(log)

	// Infrastructure
	reservationRepo := reservationRepository.NewReservationRepository(db).WithLogger(log)
	spotRepo := spotRepository.NewSpotRepository(db)

	// Discord
	dcFormatter := formatter.NewFormatter()
	botService := bot.NewManager(summaryService, reservationRepo).WithFormatter(dcFormatter).WithLogger(log)
	communicationService := communication.NewAdapter(botService, botService).WithLogger(log)

	// Bot
	bookingService := booking.NewAdapter(spotRepo, reservationRepo, communicationService).WithLogger(log)
	eventHandler := eventhandler.NewHandler(bookingService, reservationRepo, communicationService, summaryService)
	err = botService.WithEventHandler(eventHandler).Run()
	if err != nil {
		panic(err)
	}
}

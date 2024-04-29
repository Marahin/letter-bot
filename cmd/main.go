package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"spot-assistant/internal/core/api"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/communication"
	"spot-assistant/internal/core/summary"

	"spot-assistant/internal/common/version"

	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/infrastructure/chart"
	"spot-assistant/internal/infrastructure/db/postgresql"
	reservationRepository "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	spotRepository "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
)

func init() {
}

func main() {
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

	// Infrastructure
	reservationRepo := reservationRepository.NewReservationRepository(db).WithLogger(log)
	spotRepo := spotRepository.NewSpotRepository(db)
	charter := chart.NewAdapter()
	dcFormatter := formatter.NewFormatter()
	botService := bot.NewManager().WithFormatter(dcFormatter).WithLogger(log)

	// Core
	summaryService := summary.NewAdapter(charter) //.WithLogger(log)
	communicationService := communication.NewAdapter(botService, dcFormatter).WithLogger(log)
	bookingService := booking.NewAdapter(spotRepo, reservationRepo).WithLogger(log)

	// App
	app := api.NewApplication().
		WithLogger(log).
		WithBot(botService).
		WithCommunicationService(communicationService).
		WithReservationRepository(reservationRepo).
		WithSummaryService(summaryService).
		WithBookingService(bookingService)

	err = app.Run()
	if err != nil {
		panic(err)
	}
}

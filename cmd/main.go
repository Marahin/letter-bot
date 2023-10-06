package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/core/api"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/summary"

	bot "spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/chart"
	"spot-assistant/internal/infrastructure/db/postgresql"
	reservationRepository "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	spotRepository "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
)

func init() {
	logrus.Warningf("Starting with TZ: %s", time.Now().Location())
}

func main() {
	config, err := pgxpool.ParseConfig(postgresql.Dsn())
	if err != nil {
		panic(err)
	}
	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.Ping(timeout); err != nil {
		panic(err)
	}
	cancel()

	// Infrastructure
	reservationRepo := reservationRepository.NewReservationRepository(db)
	spotRepo := spotRepository.NewSpotRepository(db)
	charter := chart.NewAdapter()

	// Core
	summaryService := summary.NewAdapter(charter)
	bookingService := booking.NewAdapter(spotRepo, reservationRepo)
	api := api.NewApplication(reservationRepo, summaryService, bookingService)

	// Inverted flow - our port, "input"
	// (but also an adapter for operations)
	bot := bot.NewManager(api)

	err = bot.Run()
	if err != nil {
		panic(err)
	}
}

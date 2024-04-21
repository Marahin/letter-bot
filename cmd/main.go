package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

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
	logrus.Warningf("Version %s - Starting with TZ: %s", version.Version, time.Now().Location())
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
	dcFormatter := formatter.NewFormatter()
	botService := bot.NewManager().WithFormatter(dcFormatter)

	// Core
	summaryService := summary.NewAdapter(charter)
	communicationService := communication.NewAdapter(botService)
	bookingService := booking.NewAdapter(spotRepo, reservationRepo)

	// App
	app := api.NewApplication(reservationRepo, summaryService, bookingService).
		WithBot(botService).
		WithCommunication(communicationService)

	err = app.Run()
	if err != nil {
		panic(err)
	}
}

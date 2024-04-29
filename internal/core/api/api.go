package api

import (
	"go.uber.org/zap"
	"sync/atomic"

	"spot-assistant/internal/ports"
)

type Application struct {
	db         ports.ReservationRepository
	summarySrv summaryService
	bookingSrv bookingService
	commSrv    communicationService
	botSrv     ports.BotPort
	log        *zap.SugaredLogger
	ticks      atomic.Uint64
}

func NewApplication() *Application {
	app := &Application{
		ticks: atomic.Uint64{},
		log:   zap.NewNop().Sugar(),
	}

	return app
}

func (a *Application) WithLogger(log *zap.SugaredLogger) *Application {
	a.log = log.With("layer", "core", "name", "application")

	return a
}

func (a *Application) WithBot(bot ports.BotPort) *Application {
	a.botSrv = bot.WithEventHandler(a)

	return a
}

func (a *Application) WithCommunicationService(commSrv communicationService) *Application {
	a.commSrv = commSrv

	return a
}

func (a *Application) WithReservationRepository(db ports.ReservationRepository) *Application {
	a.db = db

	return a
}

func (a *Application) WithSummaryService(summarySrv summaryService) *Application {
	a.summarySrv = summarySrv

	return a
}

func (a *Application) WithBookingService(bookingSrv bookingService) *Application {
	a.bookingSrv = bookingSrv

	return a
}

func (a *Application) Run() error {
	return a.botSrv.Run()
}

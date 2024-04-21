package api

import (
	"spot-assistant/internal/ports"

	"github.com/sirupsen/logrus"
)

type Application struct {
	db         ports.ReservationRepository
	summarySrv summaryService
	bookingSrv bookingService
	commSrv    communicationService
	botSrv     ports.BotPort
	log        *logrus.Entry
}

func NewApplication(
	db ports.ReservationRepository,
	summarySrv summaryService,
	bookingSrv bookingService) *Application {

	app := &Application{
		db:         db,
		summarySrv: summarySrv,
		bookingSrv: bookingSrv,
		log:        logrus.WithFields(logrus.Fields{"type": "application"}),
	}

	return app
}

func (a *Application) WithBot(bot ports.BotPort) *Application {
	a.botSrv = bot.WithEventHandler(a)

	return a
}

func (a *Application) WithCommunication(commSrv communicationService) *Application {
	a.commSrv = commSrv

	return a
}

func (a *Application) Run() error {
	return a.botSrv.Run()
}

package api

import (
	"spot-assistant/internal/ports"

	"github.com/sirupsen/logrus"
)

type Application struct {
	db         ports.ReservationRepository
	summarySrv summaryService
	bookingSrv bookingService
	log        *logrus.Entry
}

func NewApplication(db ports.ReservationRepository, summarySrv summaryService, bookingSrv bookingService) *Application {
	return &Application{
		db:         db,
		summarySrv: summarySrv,
		bookingSrv: bookingSrv,
		log:        logrus.WithFields(logrus.Fields{"type": "application"}),
	}
}

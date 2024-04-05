package api

import (
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"spot-assistant/internal/ports"
)

type Application struct {
	db         ports.ReservationRepository
	summarySrv summaryService
	bookingSrv bookingService
	log        *logrus.Entry
	ticks      atomic.Uint64
}

func NewApplication(db ports.ReservationRepository, summarySrv summaryService, bookingSrv bookingService) *Application {
	return &Application{
		db:         db,
		summarySrv: summarySrv,
		bookingSrv: bookingSrv,
		log:        logrus.WithFields(logrus.Fields{"type": "application"}),
		ticks:      atomic.Uint64{},
	}
}

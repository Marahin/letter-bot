package booking

import (
	"time"

	"go.uber.org/zap"
	"spot-assistant/internal/ports"
)

type Adapter struct {
	reservationRepo ports.ReservationRepository
	spotRepo        ports.SpotRepository
	commSrv         ports.CommunicationService
	log             *zap.SugaredLogger
	now             func() time.Time
}

func NewAdapter(spotRepo ports.SpotRepository, reservationRepo ports.ReservationRepository, commSrv ports.CommunicationService) *Adapter {
	return &Adapter{
		spotRepo:        spotRepo,
		reservationRepo: reservationRepo,
		commSrv:         commSrv,
		log:             zap.NewNop().Sugar(),
		now:             time.Now,
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "bookingService")
	return a
}

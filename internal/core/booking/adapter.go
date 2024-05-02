package booking

import (
	"go.uber.org/zap"
	"spot-assistant/internal/ports"
)

type Adapter struct {
	reservationRepo ports.ReservationRepository
	spotRepo        ports.SpotRepository
	commSrv         ports.CommunicationService
	log             *zap.SugaredLogger
}

func NewAdapter(spotRepo ports.SpotRepository, reservationRepo ports.ReservationRepository, commSrv ports.CommunicationService) *Adapter {
	return &Adapter{
		spotRepo:        spotRepo,
		reservationRepo: reservationRepo,
		commSrv:         commSrv,
		log:             zap.NewNop().Sugar(),
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "bookingService")
	return a
}

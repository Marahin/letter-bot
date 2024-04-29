package booking

import (
	"go.uber.org/zap"
	"spot-assistant/internal/ports"
)

type Adapter struct {
	reservationRepo ports.ReservationRepository
	spotRepo        ports.SpotRepository
	log             *zap.SugaredLogger
}

func NewAdapter(spotRepo ports.SpotRepository, reservationRepo ports.ReservationRepository) *Adapter {
	return &Adapter{
		spotRepo:        spotRepo,
		reservationRepo: reservationRepo,
		log:             zap.NewNop().Sugar(),
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "bookingService")
	return a
}

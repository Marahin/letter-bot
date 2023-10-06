package booking

import (
	"spot-assistant/internal/ports"

	"github.com/sirupsen/logrus"
)

type Adapter struct {
	reservationRepo ports.ReservationRepository
	spotRepo        ports.SpotRepository
	log             *logrus.Entry
}

func NewAdapter(spotRepo ports.SpotRepository, reservationRepo ports.ReservationRepository) *Adapter {
	return &Adapter{
		log:             logrus.WithFields(logrus.Fields{"type": "core", "name": "booking"}),
		spotRepo:        spotRepo,
		reservationRepo: reservationRepo,
	}
}

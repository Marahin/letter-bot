package upcomingreservation

import (
	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log             *zap.SugaredLogger
	reservationRepo ports.ReservationRepository
	memberRepo      ports.MemberRepository
	commService     ports.CommunicationService
}

func NewAdapter(reservationRepo ports.ReservationRepository, memberRepo ports.MemberRepository, commService ports.CommunicationService) *Adapter {
	return &Adapter{
		reservationRepo: reservationRepo,
		memberRepo:      memberRepo,
		commService:     commService,
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "upcomingReservationService")
	return a
}

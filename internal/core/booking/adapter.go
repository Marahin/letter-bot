package booking

import (
	"context"
	"time"

	"go.uber.org/zap"
	"spot-assistant/internal/ports"
)

type Adapter struct {
	reservationRepo ports.ReservationRepository
	spotRepo        ports.SpotRepository
	commSrv         ports.CommunicationService
	queue           ports.SpotQueueFulfiller
	log             *zap.SugaredLogger
	now             func() time.Time
}

// NewAdapter builds the booking service. Every booking and edit goes through it,
// so it also removes the author's queue entry for the respawn they now hold.
func NewAdapter(spotRepo ports.SpotRepository, reservationRepo ports.ReservationRepository, commSrv ports.CommunicationService, queue ports.SpotQueueFulfiller) *Adapter {
	return &Adapter{
		spotRepo:        spotRepo,
		reservationRepo: reservationRepo,
		commSrv:         commSrv,
		queue:           queue,
		log:             zap.NewNop().Sugar(),
		now:             time.Now,
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "bookingService")
	return a
}

// fulfilQueue removes the author's queue entry for the respawn. A failure does
// not undo the booking.
func (a *Adapter) fulfilQueue(ctx context.Context, guildID, authorID string, spotID int64) {
	if authorID == "" {
		return
	}
	if _, err := a.queue.DeleteUserSpotEntry(ctx, guildID, authorID, spotID); err != nil {
		a.log.With("guild.ID", guildID, "spot.ID", spotID).Warnw("could not remove the queue entry of the booking", "error", err)
	}
}

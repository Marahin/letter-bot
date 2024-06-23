package eventhandler

import (
	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Handler struct {
	bookingSrv ports.BookingService
	db         ports.ReservationRepository
	commSrv    ports.CommunicationService
	summarySrv ports.SummaryService
	guildSrv   ports.GuildRepository
	log        *zap.SugaredLogger
}

func NewHandler(bookingSrv ports.BookingService, db ports.ReservationRepository, commSrv ports.CommunicationService, summarySrv ports.SummaryService, guildSrv ports.GuildRepository) *Handler {
	return &Handler{
		bookingSrv: bookingSrv,
		db:         db,
		commSrv:    commSrv,
		summarySrv: summarySrv,
		guildSrv:   guildSrv,
	}
}

func (a *Handler) WithLogger(log *zap.SugaredLogger) *Handler {
	a.log = log.With(
		"layer", "infrastructure",
		"name", "EventHandler")

	return a
}

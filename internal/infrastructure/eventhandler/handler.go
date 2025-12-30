package eventhandler

import (
	"time"

	"spot-assistant/internal/ports"
)

const DefaultInteractionTimeout = 15 * time.Second

type Handler struct {
	bookingSrv ports.BookingService
	db         ports.ReservationRepository
	commSrv    ports.CommunicationService
	summarySrv ports.SummaryService
	metrics    ports.MetricsPort
}

func NewHandler(bookingSrv ports.BookingService, db ports.ReservationRepository, commSrv ports.CommunicationService, summarySrv ports.SummaryService) *Handler {
	return &Handler{
		bookingSrv: bookingSrv,
		db:         db,
		commSrv:    commSrv,
		summarySrv: summarySrv,
	}
}

func (h *Handler) WithMetrics(m ports.MetricsPort) *Handler {
	h.metrics = m
	return h
}

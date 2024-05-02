package eventhandler

import (
	"spot-assistant/internal/ports"
)

type Handler struct {
	bookingSrv ports.BookingService
	db         ports.ReservationRepository
	commSrv    ports.CommunicationService
	summarySrv ports.SummaryService
}

func NewHandler(bookingSrv ports.BookingService, db ports.ReservationRepository, commSrv ports.CommunicationService, summarySrv ports.SummaryService) *Handler {
	return &Handler{
		bookingSrv: bookingSrv,
		db:         db,
		commSrv:    commSrv,
		summarySrv: summarySrv,
	}
}

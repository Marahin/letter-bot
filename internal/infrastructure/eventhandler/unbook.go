package eventhandler

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func (h *Handler) OnUnbookAutocomplete(request book.UnbookAutocompleteRequest) (book.UnbookAutocompleteResponse, error) {
	reservations, err := h.bookingSrv.UnbookAutocomplete(request.Guild, request.Member, request.Value)
	if err != nil {
		return book.UnbookAutocompleteResponse{}, err
	}

	return book.UnbookAutocompleteResponse{
		Choices: reservations,
	}, nil
}

func (h *Handler) OnUnbook(request book.UnbookRequest) (*reservation.ReservationWithSpot, error) {
	return h.bookingSrv.Unbook(request.Guild, request.Member, request.ReservationID)
}

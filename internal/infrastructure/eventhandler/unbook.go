package eventhandler

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func (a *Handler) OnUnbookAutocomplete(request book.UnbookAutocompleteRequest) (book.UnbookAutocompleteResponse, error) {
	reservations, err := a.bookingSrv.UnbookAutocomplete(request.Guild, request.Member, request.Value)
	if err != nil {
		return book.UnbookAutocompleteResponse{}, err
	}

	return book.UnbookAutocompleteResponse{
		Choices: reservations,
	}, nil
}

func (a *Handler) OnUnbook(request book.UnbookRequest) (*reservation.ReservationWithSpot, error) {
	return a.bookingSrv.Unbook(request.Guild, request.Member, request.ReservationID)
}

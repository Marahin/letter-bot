package api

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func (a *Application) OnUnbookAutocomplete(request book.UnbookAutocompleteRequest) (book.UnbookAutocompleteResponse, error) {
	reservations, err := a.bookingSrv.UnbookAutocomplete(request.Guild, request.Member, request.Value)
	if err != nil {
		return book.UnbookAutocompleteResponse{}, err
	}

	return book.UnbookAutocompleteResponse{
		Choices: reservations,
	}, nil
}

func (a *Application) OnUnbook(request book.UnbookRequest) (*reservation.ReservationWithSpot, error) {
	res, err := a.bookingSrv.Unbook(request.Guild, request.Member, request.ReservationID)
	if err != nil {
		return nil, err
	}

	go a.UpdateGuildSummaryAndLogError(request.Guild)

	return res, nil
}

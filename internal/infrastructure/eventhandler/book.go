package eventhandler

import (
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/book"
)

func (a *Handler) OnBook(request book.BookRequest) (book.BookResponse, error) {
	conflicting, err := a.bookingSrv.Book(request)
	response := book.BookResponse{
		Request:                 &request,
		ConflictingReservations: conflicting,
	}
	if err != nil {
		return response, err
	}

	return response, nil
}

func (a *Handler) OnBookAutocomplete(request book.BookAutocompleteRequest) (book.BookAutocompleteResponse, error) {
	switch request.Field {
	case book.BookAutocompleteOverbook:
		// @TODO: make it based on user permissions
		return []string{"true", "false"}, nil
	case book.BookAutocompleteStartAt:
		return a.bookingSrv.GetSuggestedHours(time.Now(), request.Value), nil
	case book.BookAutocompleteEndAt:
		return a.bookingSrv.GetSuggestedHours(time.Now().Add(2*time.Hour), request.Value), nil
	case book.BookAutocompleteSpot:
		return a.bookingSrv.FindAvailableSpots(request.Value)
	default:
		return []string{}, fmt.Errorf("autocomplete not implemented for %v", request.Field)
	}
}

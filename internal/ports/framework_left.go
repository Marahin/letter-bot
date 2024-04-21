package ports

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

type APIPort interface {
	OnReady()
	OnGuildCreate(*discord.Guild)
	OnTick()
	OnBook(book.BookRequest) (book.BookResponse, error)
	OnBookAutocomplete(book.BookAutocompleteRequest) (book.BookAutocompleteResponse, error)
	OnUnbook(request book.UnbookRequest) (*reservation.ReservationWithSpot, error)
	OnUnbookAutocomplete(request book.UnbookAutocompleteRequest) (book.UnbookAutocompleteResponse, error)
	OnPrivateSummary(summary.PrivateSummaryRequest) error
}

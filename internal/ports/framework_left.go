package ports

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

type APIPort interface {
	OnReady(BotPort)
	OnGuildCreate(BotPort, *discord.Guild)
	OnTick(BotPort)
	OnBook(BotPort, book.BookRequest) (book.BookResponse, error)
	OnBookAutocomplete(book.BookAutocompleteRequest) (book.BookAutocompleteResponse, error)
	OnUnbook(bot BotPort, request book.UnbookRequest) (*reservation.ReservationWithSpot, error)
	OnUnbookAutocomplete(request book.UnbookAutocompleteRequest) (book.UnbookAutocompleteResponse, error)
}

package formatter

import (
	"errors"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func TestDiscordFormatter_FormatGenericError(t *testing.T) {
	// given
	err := errors.New("test error")
	formatter := NewFormatter()

	// when
	output := formatter.FormatGenericError(err)

	// assert
	snaps.MatchSnapshot(t, output)
}

func TestDiscordFormatter_FormatUnbookResponse(t *testing.T) {
	// given
	formatter := NewFormatter()
	res := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              0,
			Author:          "sample-author",
			CreatedAt:       time.Time{},
			StartAt:         time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			EndAt:           time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
			SpotID:          0,
			GuildID:         "sample-guild-id",
			AuthorDiscordID: "sample-author-discord-id",
		},
		Spot: reservation.Spot{
			ID:   0,
			Name: "sample-spot-name",
		},
	}

	// when
	output := formatter.FormatUnbookResponse(res)

	// assert
	snaps.MatchSnapshot(t, output)
}

func TestDiscordFormatter_FormatBookError(t *testing.T) {
	// given
	formatter := NewFormatter()
	response := book.BookResponse{
		Request: &book.BookRequest{
			Guild:          nil,
			Member:         nil,
			Spot:           "",
			StartAt:        time.Time{},
			EndAt:          time.Time{},
			Overbook:       false,
			HasPermissions: false,
		},
		ConflictingReservations: []*reservation.ClippedOrRemovedReservation{
			{
				Original: &reservation.Reservation{
					ID:      0,
					Author:  "sample-author",
					StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
				},
				New: []*reservation.Reservation{
					{
						ID:      0,
						Author:  "sample-author",
						StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:   time.Date(2021, 1, 1, 1, 30, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	// when
	output := formatter.FormatBookError(response, errors.New("test error"))

	// assert
	snaps.MatchSnapshot(t, output)
}

func TestDiscordFormatter_FormatUnbookResponse1(t *testing.T) {
	// given
	formatter := NewFormatter()
	res := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:      0,
			SpotID:  0,
			StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
		},
		Spot: reservation.Spot{
			ID:   0,
			Name: "test-spot",
		},
	}

	// when
	output := formatter.FormatUnbookResponse(res)

	// assert
	snaps.MatchSnapshot(t, output)
}

func TestDiscordFormatter_FormatBookResponse(t *testing.T) {
	// given
	formatter := NewFormatter()
	response := book.BookResponse{
		Request: &book.BookRequest{
			Member: &member.Member{
				ID:   "test-id",
				Nick: "test-nick",
			},
			Guild:   &guild.Guild{},
			Spot:    "test-spot",
			StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
		},
		ConflictingReservations: []*reservation.ClippedOrRemovedReservation{
			{Original: &reservation.Reservation{
				ID:      0,
				Author:  "sample-author",
				StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				EndAt:   time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
			},
				New: []*reservation.Reservation{
					{
						ID:      0,
						Author:  "sample-author",
						StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
						EndAt:   time.Date(2021, 1, 1, 1, 30, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	// when
	output := formatter.FormatBookResponse(response)

	// assert
	snaps.MatchSnapshot(t, output)
}

func TestDiscordFormatter_FormatOverbookedMemberNotification(t *testing.T) {
	// given
	formatter := NewFormatter()
	member := &member.Member{
		ID:   "test-id",
		Nick: "test-nick",
	}
	request := book.BookRequest{
		Member: member,
	}
	res := &reservation.ClippedOrRemovedReservation{
		Original: &reservation.Reservation{
			ID:      0,
			StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
		},
		New: []*reservation.Reservation{
			{
				ID:      1,
				StartAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				EndAt:   time.Date(2021, 1, 1, 1, 10, 0, 0, time.UTC),
			},
			{
				ID:      2,
				StartAt: time.Date(2021, 1, 1, 1, 30, 0, 0, time.UTC),
				EndAt:   time.Date(2021, 1, 1, 2, 0, 0, 0, time.UTC),
			},
		},
	}

	// when
	output := formatter.FormatOverbookedMemberNotification(member, request, res)

	// assert
	snaps.MatchSnapshot(t, output)
}

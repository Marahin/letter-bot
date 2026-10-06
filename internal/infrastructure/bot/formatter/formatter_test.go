package formatter

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/reservationforms"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/ports"
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
	m := &member.Member{
		ID:       "test-id",
		Nick:     "test-nick",
		Username: "test-username",
	}
	request := book.BookRequest{
		Member: m,
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
	output := formatter.FormatOverbookedMemberNotification(m, request, res)

	// assert
	snaps.MatchSnapshot(t, output)
}

func TestDiscordFormatter_FormatFormError(t *testing.T) {
	// given
	f := NewFormatter()
	errs := []error{
		reservationforms.ErrChoiceIncomplete,
		booking.ErrSpotNotFound,
		booking.ErrSpotArchived,
		booking.ErrReserveNotAllowed,
		reservations.ErrForbidden,
		fmt.Errorf("select reservation: %w", ports.ErrNotFound),
		booking.ErrReservationEnded,
		booking.ErrReservationTooLong,
		booking.ErrQuotaExceeded,
		booking.ErrStartInPast,
		booking.ErrInvalidRange,
		booking.ErrSpotLocked,
		booking.ErrSelfOverbook,
		booking.ErrConflict,
		booking.ErrInsufficientPermissions,
		errors.New("db down"),
	}
	lines := make([]string, 0, len(errs))

	// when
	for _, err := range errs {
		lines = append(lines, f.FormatFormError(err))
	}

	// then
	snaps.MatchSnapshot(t, strings.Join(lines, "\n---\n"))
	assert.NotContains(t, lines[len(lines)-2], "overbook' parameter", "the form copy does not mention the slash command option")
}

func TestDiscordFormatter_FormTexts(t *testing.T) {
	// given
	f := NewFormatter()
	start := time.Date(2026, 10, 5, 18, 30, 0, 0, time.UTC)
	r := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{StartAt: start, EndAt: start.Add(2 * time.Hour)},
		Spot:        reservation.Spot{Name: "Library -1"},
	}
	outcome := &reservation.FormOutcome{Draft: reservation.Draft{StartAt: start, EndAt: start.Add(2 * time.Hour)}, SpotName: "Library -1"}

	// when
	texts := []string{
		f.FormatReservationLine(1, r, start.Add(-time.Minute)),
		f.FormatReservationLine(2, r, start),
		f.FormatFormEdited(outcome),
		f.FormatCancelConfirm(r),
		f.FormatCancelled(r),
		f.FormatFormOutdated(),
		f.FormatFormNoGuild(),
	}

	// then
	snaps.MatchSnapshot(t, strings.Join(texts, "\n"))
}

func TestDiscordFormatter_WizardTexts(t *testing.T) {
	// given
	f := NewFormatter()
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	booked := &reservation.Reservation{Author: "Druid", StartAt: now.Add(time.Hour), EndAt: now.Add(3 * time.Hour)}
	editing := &reservation.ReservationWithSpot{Reservation: reservation.Reservation{StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)}}
	many := make([]*reservation.Reservation, 0, 12)
	for i := range 12 {
		many = append(many, &reservation.Reservation{Author: fmt.Sprintf("Player %d", i), StartAt: now.Add(time.Duration(i) * time.Hour), EndAt: now.Add(time.Duration(i)*time.Hour + 30*time.Minute)})
	}
	spot := reservation.Spot{ID: 7, Name: "Library -1"}

	// when
	texts := make([]string, 0, 21)
	texts = append(texts,
		f.FormatRespawnStep(false, false, ""),
		f.FormatRespawnStep(true, true, "No respawn has this name."),
		f.FormatRespawnMatches("lib", false),
		f.FormatRespawnMatches("lib", true),
		f.FormatUsualRespawn(reservation.UsualRespawn{}),
		f.FormatUsualRespawn(reservation.UsualRespawn{Bookings: 1}),
		f.FormatUsualRespawn(reservation.UsualRespawn{Bookings: 7}),
		f.FormatTimeStep(&reservation.TimePicker{Spot: spot}, "", now),
		f.FormatTimeStep(&reservation.TimePicker{Spot: spot, Choice: reservation.TimeChoice{StartAt: now.Add(30 * time.Hour), Length: 90 * time.Minute}, Booked: []*reservation.Reservation{booked}}, "", now),
		f.FormatTimeStep(&reservation.TimePicker{Spot: spot, Editing: editing, Choice: reservation.TimeChoice{Now: true, Length: time.Hour}, Booked: many}, "Status.", now),
		f.FormatTimeStep(&reservation.TimePicker{Spot: spot, Editing: editing, Ongoing: true}, "", now),
		f.FormatClock(now.Add(time.Hour), now),
		f.FormatClock(now.Add(7*time.Hour), now),
		f.FormatClock(now.Add(31*time.Hour), now),
		f.FormatLength(30*time.Minute),
		f.FormatLength(2*time.Hour),
		f.FormatLength(90*time.Minute),
	)
	for _, o := range []reservation.StartOption{
		{StartAt: now, Now: true},
		{StartAt: now.Add(time.Hour), BookedBy: booked},
		{StartAt: now.Add(10 * time.Minute), Current: true},
		{StartAt: now.Add(-time.Hour), Keep: true},
	} {
		label, description := f.FormatStartOption(o, now)
		texts = append(texts, label+" | "+description)
	}

	// then
	snaps.MatchSnapshot(t, strings.Join(texts, "\n---\n"))
}

package reservationforms

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/ports"
)

func TestRespawnPicker(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpots", ctx, guildID, false).Return(named("Banuta", "Asura"), nil)
	f.spots.On("SelectTopGuildSpots", ctx, guildID, "u1", now.Add(-UsualPeriod), SelectOptions).
		Return([]spot.Ranked{ranked(1, "Banuta", 4, now)}, nil)
	f.spots.On("SelectTopGuildSpots", ctx, guildID, "", now.Add(-PopularPeriod), SelectOptions).
		Return([]spot.Ranked{ranked(1, "Banuta", 9, now), ranked(2, "Asura", 3, now)}, nil)

	// when
	picker, err := f.s.RespawnPicker(ctx, guildID, member(), 0)

	// then
	require.NoError(t, err)
	assert.Equal(t, []reservation.UsualRespawn{
		{Spot: reservation.Spot{ID: 1, Name: "Banuta"}, Bookings: 4},
		{Spot: reservation.Spot{ID: 2, Name: "Asura"}},
	}, picker.Usual)
	require.Len(t, picker.Groups, 1)
	assert.Equal(t, "Asura", picker.Groups[0].Spots[0].Name)
}

func TestRespawnPicker_WithoutAUserAsksOnlyForPopular(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpots", ctx, guildID, false).Return([]*spot.Spot{}, nil)
	f.spots.On("SelectTopGuildSpots", ctx, guildID, "", mock.Anything, SelectOptions).Return(nil, nil)

	// when
	picker, err := f.s.RespawnPicker(ctx, guildID, reservation.Actor{}, 3)

	// then
	require.NoError(t, err)
	assert.Empty(t, picker.Usual)
	assert.Equal(t, 1, picker.Pages)
}

func TestRespawnPicker_Errors(t *testing.T) {
	boom := errors.New("db down")
	tests := []struct {
		name  string
		setup func(f fixture)
	}{
		{"spots", func(f fixture) {
			f.spots.On("SelectGuildSpots", ctx, guildID, false).Return(nil, boom)
		}},
		{"usual", func(f fixture) {
			f.spots.On("SelectGuildSpots", ctx, guildID, false).Return(nil, nil)
			f.spots.On("SelectTopGuildSpots", ctx, guildID, "u1", mock.Anything, SelectOptions).Return(nil, boom)
		}},
		{"popular", func(f fixture) {
			f.spots.On("SelectGuildSpots", ctx, guildID, false).Return(nil, nil)
			f.spots.On("SelectTopGuildSpots", ctx, guildID, "u1", mock.Anything, SelectOptions).Return(nil, nil)
			f.spots.On("SelectTopGuildSpots", ctx, guildID, "", mock.Anything, SelectOptions).Return(nil, boom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFixture(t)
			tt.setup(f)

			// when
			_, err := f.s.RespawnPicker(ctx, guildID, member(), 0)

			// then
			assert.ErrorIs(t, err, boom)
		})
	}
}

func TestFindRespawn(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	candidates := []*spot.Spot{library(), {ID: 8, Name: "Library -2"}}
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "Library -1").Return(library(), nil)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, mock.Anything).Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "libr").Return([]*spot.Spot{library()}, nil)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "Library").Return(candidates, nil)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "Nowhere").Return([]*spot.Spot{}, nil)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "B").Return(nil, boom)

	// when
	exact, exactErr := f.s.FindRespawn(ctx, guildID, " Library -1 ")
	partial, partialErr := f.s.FindRespawn(ctx, guildID, "libr")
	_, ambiguousErr := f.s.FindRespawn(ctx, guildID, "Library")
	_, noneErr := f.s.FindRespawn(ctx, guildID, "Nowhere")
	_, emptyErr := f.s.FindRespawn(ctx, guildID, "  ")
	_, failed := f.s.FindRespawn(ctx, guildID, "B")

	// then
	require.NoError(t, exactErr)
	assert.Equal(t, library(), exact)
	require.NoError(t, partialErr)
	assert.Equal(t, library(), partial)
	var ambiguous *AmbiguousSpotError
	require.ErrorAs(t, ambiguousErr, &ambiguous)
	assert.Equal(t, candidates, ambiguous.Candidates)
	assert.False(t, ambiguous.Capped)
	assert.Contains(t, ambiguous.Error(), "2 respawns")
	assert.ErrorIs(t, noneErr, booking.ErrSpotNotFound)
	assert.ErrorIs(t, emptyErr, booking.ErrSpotNotFound)
	assert.ErrorIs(t, failed, boom)
}

func TestFindRespawn_FullListMayMissSome(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "a").Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "a").Return(numbered(ports.SpotsLikeLimit), nil)

	// when
	_, err := f.s.FindRespawn(ctx, guildID, "a")

	// then
	var ambiguous *AmbiguousSpotError
	require.ErrorAs(t, err, &ambiguous)
	assert.True(t, ambiguous.Capped)
}

func TestFindRespawn_ByNameFails(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "A").Return(nil, boom)

	// when
	_, err := f.s.FindRespawn(ctx, guildID, "A")

	// then
	assert.ErrorIs(t, err, boom)
}

func searchPage(items ...*reservation.ReservationWithSpot) *reservation.Page {
	return &reservation.Page{Items: items, Total: int64(len(items))}
}

func bookedBy(id int64, author string, start, end time.Time) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: id, SpotID: 7, Author: author, StartAt: start, EndAt: end},
		Spot:        reservation.Spot{ID: 7, Name: "Library -1"},
	}
}

func searching(spotID int64, from, to time.Time) any {
	return mock.MatchedBy(func(f reservation.SearchFilter) bool {
		return f.GuildID == guildID && f.SpotID != nil && *f.SpotID == spotID &&
			f.From.Equal(from) && f.To.Equal(to) && f.Scope == reservation.ScopeAll
	})
}

func TestTimePicker_Booking(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	early := bookedBy(3, "Druid", at(5, 17, 0), at(5, 18, 0))
	late := bookedBy(2, "Knight", at(5, 20, 0), at(5, 22, 0))
	middle := bookedBy(1, "Sorcerer", at(5, 18, 30), at(5, 19, 0))
	f.reservations.On("Search", ctx, searching(7, now, at(6, 6, 30)), 1).Return(searchPage(late, early, middle), nil)

	// when
	picker, err := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, Window: reservation.AutoWindow, Length: time.Hour})

	// then
	require.NoError(t, err)
	assert.Equal(t, reservation.Spot{ID: 7, Name: "Library -1"}, picker.Spot)
	assert.Equal(t, 0, picker.Choice.Window)
	assert.Equal(t, Windows, picker.Windows)
	assert.Nil(t, picker.Editing)
	require.Len(t, picker.Booked, 2, "a reservation that ends at the window start is left out")
	assert.Equal(t, "Sorcerer", picker.Booked[0].Author)
	assert.Equal(t, "Knight", picker.Booked[1].Author)
	assert.Len(t, picker.Starts, SelectOptions)
	assert.True(t, picker.Lengths[1].Selected)
}

func TestTimePicker_WindowOfTheStart(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	f.reservations.On("Search", ctx, searching(7, at(6, 6, 30), at(6, 18, 30)), 1).Return(searchPage(), nil)

	// when
	picker, err := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, Window: reservation.AutoWindow, StartAt: at(6, 10, 0)})

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, picker.Choice.Window)
	assert.Len(t, picker.Starts, WindowSlots)
}

func TestTimePicker_ClampsTheWindow(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	f.reservations.On("Search", ctx, mock.Anything, 1).Return(searchPage(), nil)

	// when
	picker, err := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, Window: 9})

	// then
	require.NoError(t, err)
	assert.Equal(t, Windows-1, picker.Choice.Window)
}

func TestTimePicker_Refusals(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	archivedAt := now
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(1)).Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(2)).Return(&spot.Spot{ID: 2, Name: "Old", ArchivedAt: &archivedAt}, nil)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(3)).Return(nil, boom)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	f.reservations.On("Search", ctx, mock.Anything, 1).Return(nil, boom)

	// when
	_, notFound := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 1})
	_, archived := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 2})
	_, failed := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 3})
	_, searchFailed := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7})
	_, noSpot := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{})

	// then
	assert.ErrorIs(t, notFound, booking.ErrSpotNotFound)
	assert.ErrorIs(t, archived, booking.ErrSpotArchived)
	assert.ErrorIs(t, failed, boom)
	assert.ErrorIs(t, searchFailed, boom)
	assert.ErrorIs(t, noSpot, booking.ErrSpotNotFound)
}

func TestTimePicker_EditFillsTheReservation(t *testing.T) {
	// given
	f := newFixture(t)
	editing := owned(5, at(5, 19, 10), at(5, 21, 0))
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(editing, nil)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	f.reservations.On("Search", ctx, searching(7, now, at(6, 6, 30)), 1).Return(searchPage(editing), nil)

	// when
	picker, err := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, Window: reservation.AutoWindow})

	// then
	require.NoError(t, err)
	assert.Same(t, editing, picker.Editing)
	assert.False(t, picker.Ongoing)
	assert.Equal(t, reservation.TimeChoice{SpotID: 7, ReservationID: 5, StartAt: at(5, 19, 10), Length: 110 * time.Minute}, picker.Choice)
	assert.Empty(t, picker.Booked, "the edited reservation is left out")
	assert.True(t, picker.Starts[0].Current)
	assert.True(t, picker.Starts[0].Selected)
	assert.True(t, picker.Lengths[0].Current)
	assert.True(t, picker.Lengths[0].Selected)
}

func TestTimePicker_Ongoing(t *testing.T) {
	// given
	f := newFixture(t)
	editing := owned(5, at(5, 17, 0), at(5, 19, 0))
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(editing, nil)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	f.reservations.On("Search", ctx, searching(7, now, now.Add(booking.MaximumReservationLength)), 1).Return(searchPage(), nil)

	// when
	picker, err := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, Window: 2, Now: true, Length: 150 * time.Minute})
	_, moved := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, SpotID: 8})

	// then
	require.NoError(t, err)
	assert.True(t, picker.Ongoing)
	assert.Equal(t, []reservation.StartOption{{StartAt: at(5, 17, 0), Keep: true, Selected: true}}, picker.Starts)
	assert.False(t, picker.Choice.Now)
	assert.Equal(t, 0, picker.Choice.Window)
	assert.Equal(t, 90*time.Minute, picker.Lengths[0].Length)
	assert.True(t, picker.Lengths[2].Selected)
	assert.ErrorIs(t, moved, booking.ErrSpotLocked)
}

func TestTimePicker_EditRefused(t *testing.T) {
	// given
	f := newFixture(t)
	someone := owned(5, at(5, 19, 0), at(5, 21, 0))
	someone.AuthorDiscordID = "u2"
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(someone, nil)

	// when
	_, err := f.s.TimePicker(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5})

	// then
	assert.ErrorIs(t, err, reservations.ErrForbidden)
}

func TestBookChoice(t *testing.T) {
	// given
	f := newFixture(t)
	f.s.now = func() time.Time { return now.Add(10 * time.Minute) }
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	draft := reservation.Draft{SpotID: 7, StartAt: at(5, 18, 10), EndAt: at(5, 20, 10)}
	f.reservations.On("Create", ctx, guildID, member(), draft).Return(nil, nil)

	// when
	outcome, err := f.s.BookChoice(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, Now: true, Length: 2 * time.Hour})
	_, incomplete := f.s.BookChoice(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, Now: true})
	_, noStart := f.s.BookChoice(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, Length: time.Hour})
	_, past := f.s.BookChoice(ctx, guildID, member(), reservation.TimeChoice{SpotID: 7, StartAt: at(5, 17, 0), Length: time.Hour})

	// then
	require.NoError(t, err)
	assert.Equal(t, draft, outcome.Draft)
	assert.ErrorIs(t, incomplete, ErrChoiceIncomplete)
	assert.ErrorIs(t, noStart, ErrChoiceIncomplete)
	assert.ErrorIs(t, past, booking.ErrStartInPast)
}

func TestEditChoice_KeepsTheStoredStart(t *testing.T) {
	// given
	f := newFixture(t)
	stored := at(5, 17, 0).Add(123 * time.Microsecond)
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(owned(5, stored, at(5, 19, 0)), nil)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	draft := reservation.Draft{SpotID: 7, StartAt: stored, EndAt: stored.Add(3 * time.Hour)}
	f.reservations.On("Edit", ctx, guildID, member(), int64(5), draft).Return(nil, nil)

	// when
	outcome, err := f.s.EditChoice(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, SpotID: 7, StartAt: time.Unix(stored.Unix(), 0), Length: 3 * time.Hour})

	// then
	require.NoError(t, err)
	assert.Equal(t, "Library -1", outcome.SpotName)
}

func TestEditChoice_MovesTheStart(t *testing.T) {
	// given
	f := newFixture(t)
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(owned(5, at(5, 19, 0), at(5, 21, 0)), nil)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(8)).Return(&spot.Spot{ID: 8, Name: "Hero Cave"}, nil)
	draft := reservation.Draft{SpotID: 8, StartAt: at(5, 20, 0), EndAt: at(5, 21, 0)}
	conflicts := []*reservation.Reservation{{ID: 11}}
	f.reservations.On("Edit", ctx, guildID, member(), int64(5), draft).Return(conflicts, booking.ErrConflict)

	// when
	outcome, err := f.s.EditChoice(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, SpotID: 8, StartAt: at(5, 20, 0), Length: time.Hour})

	// then
	assert.ErrorIs(t, err, booking.ErrConflict)
	assert.Equal(t, conflicts, outcome.Conflicts)
}

func TestEditChoice_Refusals(t *testing.T) {
	// given
	f := newFixture(t)
	f.reservations.On("Get", ctx, guildID, int64(1)).Return(nil, ports.ErrNotFound)
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(owned(5, at(5, 19, 0), at(5, 21, 0)), nil)

	// when
	_, notFound := f.s.EditChoice(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 1})
	_, incomplete := f.s.EditChoice(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, SpotID: 7, StartAt: at(5, 20, 0)})
	_, past := f.s.EditChoice(ctx, guildID, member(), reservation.TimeChoice{ReservationID: 5, SpotID: 7, StartAt: at(5, 10, 0), Length: time.Hour})

	// then
	assert.ErrorIs(t, notFound, ports.ErrNotFound)
	assert.ErrorIs(t, incomplete, ErrChoiceIncomplete)
	assert.ErrorIs(t, past, booking.ErrStartInPast)
}

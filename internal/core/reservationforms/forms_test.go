package reservationforms

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

var (
	ctx = context.Background()
	now = time.Date(2026, 10, 5, 18, 0, 0, 0, berlin)
)

type fixture struct {
	s            *Service
	reservations *mocks.MockReservationService
	spots        *mocks.MockSpotRepository
}

func newFixture(t *testing.T) fixture {
	f := fixture{reservations: mocks.NewMockReservationService(t), spots: mocks.NewMockSpotRepository(t)}
	f.s = New(f.reservations, f.spots)
	f.s.now = func() time.Time { return now }
	return f
}

func member() reservation.Actor {
	return reservation.Actor{UserID: "u1", Name: "Knight", Caps: permission.Capabilities{View: true, Reserve: true}}
}

func library() *spot.Spot {
	return &spot.Spot{ID: 7, Name: "Library -1", GuildID: guildID}
}

func owned(id int64, start, end time.Time) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: id, SpotID: 7, StartAt: start, EndAt: end, AuthorDiscordID: "u1", Author: "Knight"},
		Spot:        reservation.Spot{ID: 7, Name: "Library -1"},
	}
}

func TestBookForm_BooksTheMatchingRespawn(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "Library -1").Return(library(), nil)
	draft := reservation.Draft{SpotID: 7, StartAt: at(5, 19, 0), EndAt: at(5, 21, 0)}
	overbooked := []*reservation.ClippedOrRemovedReservation{{Original: &reservation.Reservation{ID: 3}}}
	f.reservations.On("Create", ctx, guildID, member(), draft).Return(overbooked, nil)

	// when
	outcome, err := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: " Library -1 ", StartAt: "19:00", EndAt: "21:00"})

	// then
	require.NoError(t, err)
	assert.Equal(t, &reservation.FormOutcome{Draft: draft, SpotName: "Library -1", Overbooked: overbooked}, outcome)
}

func TestBookForm_RejectsABadTimeFirst(t *testing.T) {
	// given
	f := newFixture(t)

	// when
	outcome, err := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "Library", StartAt: "late", EndAt: "21:00"})

	// then
	assert.ErrorIs(t, err, ErrTimeFormat)
	assert.Nil(t, outcome)
}

func TestBookForm_PicksTheOnlyPartialMatch(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "libr").Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "libr").Return([]*spot.Spot{library()}, nil)
	f.reservations.On("Create", ctx, guildID, member(), mock.Anything).Return(nil, nil)

	// when
	outcome, err := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "libr", StartAt: "19:00", EndAt: "21:00"})

	// then
	require.NoError(t, err)
	assert.Equal(t, "Library -1", outcome.SpotName)
}

func TestBookForm_ReturnsTheCandidatesOfAnAmbiguousName(t *testing.T) {
	// given
	f := newFixture(t)
	candidates := []*spot.Spot{library(), {ID: 8, Name: "Library -2"}}
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "Library").Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "Library").Return(candidates, nil)

	// when
	outcome, err := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "Library", StartAt: "19:00", EndAt: "21:00"})

	// then
	var ambiguous *AmbiguousSpotError
	require.ErrorAs(t, err, &ambiguous)
	assert.Equal(t, candidates, ambiguous.Candidates)
	assert.Contains(t, ambiguous.Error(), "2 respawns")
	assert.Equal(t, reservation.Draft{StartAt: at(5, 19, 0), EndAt: at(5, 21, 0)}, outcome.Draft)
}

func TestBookForm_UnknownRespawn(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "Nowhere").Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "Nowhere").Return([]*spot.Spot{}, nil)

	// when
	_, err := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "Nowhere", StartAt: "19:00", EndAt: "21:00"})
	_, emptyErr := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "  ", StartAt: "19:00", EndAt: "21:00"})

	// then
	assert.ErrorIs(t, err, booking.ErrSpotNotFound)
	assert.ErrorIs(t, emptyErr, booking.ErrSpotNotFound)
}

func TestBookForm_WrapsRepositoryErrors(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "A").Return(nil, boom)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "B").Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "B").Return(nil, boom)

	// when
	_, errA := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "A", StartAt: "19:00", EndAt: "21:00"})
	_, errB := f.s.BookForm(ctx, guildID, member(), reservation.Form{Spot: "B", StartAt: "19:00", EndAt: "21:00"})

	// then
	assert.ErrorIs(t, errA, boom)
	assert.ErrorIs(t, errB, boom)
}

func TestBook_Conflicts(t *testing.T) {
	other := &reservation.Reservation{ID: 3, AuthorDiscordID: "u2", StartAt: at(5, 19, 0), EndAt: at(5, 20, 0)}
	own := &reservation.Reservation{ID: 4, AuthorDiscordID: "u1", StartAt: at(5, 20, 0), EndAt: at(5, 21, 0)}
	tests := []struct {
		name      string
		caps      permission.Capabilities
		overbook  bool
		conflicts []*reservation.Reservation
		want      bool
	}{
		{"with the overbook right", permission.Capabilities{Reserve: true, Overbook: true}, false, []*reservation.Reservation{other}, true},
		{"without the right", permission.Capabilities{Reserve: true}, false, []*reservation.Reservation{other}, false},
		{"over an own reservation", permission.Capabilities{Reserve: true, Overbook: true}, false, []*reservation.Reservation{other, own}, false},
		{"after an overbook", permission.Capabilities{Reserve: true, Overbook: true}, true, []*reservation.Reservation{other}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFixture(t)
			actor := reservation.Actor{UserID: "u1", Name: "Knight", Caps: tt.caps}
			draft := reservation.Draft{SpotID: 7, StartAt: at(5, 19, 0), EndAt: at(5, 21, 0), Overbook: tt.overbook}
			blocking := make([]*reservation.ClippedOrRemovedReservation, 0, len(tt.conflicts))
			for _, c := range tt.conflicts {
				blocking = append(blocking, &reservation.ClippedOrRemovedReservation{Original: c, New: []*reservation.Reservation{c}})
			}
			f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
			f.reservations.On("Create", ctx, guildID, actor, draft).Return(blocking, booking.ErrInsufficientPermissions)

			// when
			outcome, err := f.s.Book(ctx, guildID, actor, draft)

			// then
			assert.ErrorIs(t, err, booking.ErrInsufficientPermissions)
			assert.Equal(t, tt.conflicts, outcome.Conflicts)
			assert.Equal(t, tt.want, outcome.CanOverbook)
			assert.Equal(t, "Library -1", outcome.SpotName)
		})
	}
}

func TestBook_DropsTheAuthorFields(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	booked := reservation.Draft{SpotID: 7, StartAt: at(5, 19, 0), EndAt: at(5, 21, 0)}
	f.reservations.On("Create", ctx, guildID, member(), booked).Return(nil, booking.ErrQuotaExceeded)

	// when
	_, err := f.s.Book(ctx, guildID, member(), reservation.Draft{SpotID: 7, StartAt: at(5, 19, 0), EndAt: at(5, 21, 0), Author: "Someone", AuthorDiscordID: "9"})

	// then
	assert.ErrorIs(t, err, booking.ErrQuotaExceeded)
}

func TestBook_UnknownSpot(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(8)).Return(nil, boom)

	// when
	_, notFound := f.s.Book(ctx, guildID, member(), reservation.Draft{SpotID: 7})
	_, failed := f.s.Book(ctx, guildID, member(), reservation.Draft{SpotID: 8})

	// then
	assert.ErrorIs(t, notFound, booking.ErrSpotNotFound)
	assert.ErrorIs(t, failed, boom)
}

func TestEditForm_KeepsTheRespawnWithTheSameName(t *testing.T) {
	// given
	f := newFixture(t)
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(owned(5, at(5, 17, 0), at(5, 19, 0)), nil)
	draft := reservation.Draft{SpotID: 7, StartAt: at(5, 17, 0), EndAt: at(5, 20, 0)}
	f.reservations.On("Edit", ctx, guildID, member(), int64(5), draft).Return(nil, nil)

	// when
	outcome, err := f.s.EditForm(ctx, guildID, member(), 5, reservation.Form{Spot: "library -1", StartAt: "17:00", EndAt: "20:00"})

	// then
	require.NoError(t, err)
	assert.Equal(t, &reservation.FormOutcome{Draft: draft, SpotName: "Library -1"}, outcome)
}

func TestEditForm_MovesToAnotherRespawn(t *testing.T) {
	// given
	f := newFixture(t)
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(owned(5, at(6, 10, 0), at(6, 12, 0)), nil)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "Hero Cave").Return(&spot.Spot{ID: 9, Name: "Hero Cave"}, nil)
	draft := reservation.Draft{SpotID: 9, StartAt: at(6, 10, 0), EndAt: at(6, 12, 0)}
	conflicts := []*reservation.Reservation{{ID: 11}}
	f.reservations.On("Edit", ctx, guildID, member(), int64(5), draft).Return(conflicts, booking.ErrConflict)

	// when
	outcome, err := f.s.EditForm(ctx, guildID, member(), 5, reservation.Form{Spot: "Hero Cave", StartAt: "10:00", EndAt: "12:00"})

	// then
	assert.ErrorIs(t, err, booking.ErrConflict)
	assert.Equal(t, conflicts, outcome.Conflicts)
	assert.Equal(t, "Hero Cave", outcome.SpotName)
}

func TestEditForm_Refusals(t *testing.T) {
	// given
	f := newFixture(t)
	f.reservations.On("Get", ctx, guildID, int64(1)).Return(nil, ports.ErrNotFound)
	f.reservations.On("Get", ctx, guildID, int64(2)).Return(owned(2, at(5, 10, 0), at(5, 12, 0)), nil)
	f.reservations.On("Get", ctx, guildID, int64(3)).Return(owned(3, at(6, 10, 0), at(6, 12, 0)), nil)
	f.spots.On("SelectGuildSpotByName", ctx, guildID, "Lib").Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotsLike", ctx, guildID, "Lib").Return([]*spot.Spot{library(), {ID: 8, Name: "Library -2"}}, nil)

	// when
	_, notFound := f.s.EditForm(ctx, guildID, member(), 1, reservation.Form{})
	_, ended := f.s.EditForm(ctx, guildID, member(), 2, reservation.Form{})
	_, badTime := f.s.EditForm(ctx, guildID, member(), 3, reservation.Form{Spot: "Library -1", StartAt: "x", EndAt: "12:00"})
	outcome, ambiguous := f.s.EditForm(ctx, guildID, member(), 3, reservation.Form{Spot: "Lib", StartAt: "10:00", EndAt: "12:00"})

	// then
	assert.ErrorIs(t, notFound, ports.ErrNotFound)
	assert.ErrorIs(t, ended, booking.ErrReservationEnded)
	assert.ErrorIs(t, badTime, ErrTimeFormat)
	var amb *AmbiguousSpotError
	assert.ErrorAs(t, ambiguous, &amb)
	assert.Equal(t, at(6, 10, 0), outcome.Draft.StartAt)
}

func TestEdit_DraftNeverOverbooks(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	sent := reservation.Draft{SpotID: 7, StartAt: at(6, 10, 0), EndAt: at(6, 12, 0)}
	f.reservations.On("Edit", ctx, guildID, member(), int64(5), sent).Return(nil, nil)

	// when
	outcome, err := f.s.Edit(ctx, guildID, member(), 5, reservation.Draft{SpotID: 7, StartAt: at(6, 10, 0), EndAt: at(6, 12, 0), Overbook: true, Author: "x"})

	// then
	require.NoError(t, err)
	assert.Equal(t, "Library -1", outcome.SpotName)
}

func TestEdit_UnknownSpot(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(nil, ports.ErrNotFound)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(8)).Return(nil, boom)

	// when
	_, notFound := f.s.Edit(ctx, guildID, member(), 5, reservation.Draft{SpotID: 7})
	_, failed := f.s.Edit(ctx, guildID, member(), 5, reservation.Draft{SpotID: 8})

	// then
	assert.ErrorIs(t, notFound, booking.ErrSpotNotFound)
	assert.ErrorIs(t, failed, boom)
}

func TestEditable_RefusesSomeoneElsesReservation(t *testing.T) {
	// given
	f := newFixture(t)
	theirs := owned(5, at(6, 10, 0), at(6, 12, 0))
	theirs.AuthorDiscordID = "u2"
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(theirs, nil)
	manager := reservation.Actor{UserID: "m1", Caps: permission.Capabilities{Reserve: true, Manage: true}}

	// when
	_, err := f.s.Editable(ctx, guildID, member(), 5)
	r, managerErr := f.s.Editable(ctx, guildID, manager, 5)

	// then
	assert.ErrorIs(t, err, reservations.ErrForbidden)
	require.NoError(t, managerErr)
	assert.Same(t, theirs, r)
}

func TestCancellable(t *testing.T) {
	// given
	f := newFixture(t)
	theirs := owned(5, at(6, 10, 0), at(6, 12, 0))
	theirs.AuthorDiscordID = "u2"
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(theirs, nil)
	f.reservations.On("Get", ctx, guildID, int64(6)).Return(nil, ports.ErrNotFound)

	// when
	_, forbidden := f.s.Cancellable(ctx, guildID, member(), 5)
	_, notFound := f.s.Cancellable(ctx, guildID, member(), 6)

	// then
	assert.ErrorIs(t, forbidden, reservations.ErrForbidden)
	assert.ErrorIs(t, notFound, ports.ErrNotFound)
}

func TestCancel_DeletesAndReturnsTheReservation(t *testing.T) {
	// given
	f := newFixture(t)
	mine := owned(5, at(6, 10, 0), at(6, 12, 0))
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(mine, nil)
	f.reservations.On("Delete", ctx, guildID, member(), int64(5)).Return(nil).Once()

	// when
	r, err := f.s.Cancel(ctx, guildID, member(), 5)

	// then
	require.NoError(t, err)
	assert.Same(t, mine, r)
}

func TestCancel_Errors(t *testing.T) {
	// given
	f := newFixture(t)
	boom := errors.New("db down")
	f.reservations.On("Get", ctx, guildID, int64(5)).Return(owned(5, at(6, 10, 0), at(6, 12, 0)), nil)
	f.reservations.On("Delete", ctx, guildID, member(), int64(5)).Return(boom)
	f.reservations.On("Get", ctx, guildID, int64(6)).Return(nil, ports.ErrNotFound)

	// when
	_, deleteErr := f.s.Cancel(ctx, guildID, member(), 5)
	_, getErr := f.s.Cancel(ctx, guildID, member(), 6)

	// then
	assert.ErrorIs(t, deleteErr, boom)
	assert.ErrorIs(t, getErr, ports.ErrNotFound)
}

func TestMine_SearchesTheActorsUpcomingReservations(t *testing.T) {
	// given
	f := newFixture(t)
	page := &reservation.Page{Total: 1, Items: []*reservation.ReservationWithSpot{owned(5, at(6, 10, 0), at(6, 12, 0))}}
	f.reservations.On("Search", ctx, reservation.SearchFilter{GuildID: guildID, AuthorDiscordID: "u1", Scope: reservation.ScopeUpcoming, Limit: 4}, 1).Return(page, nil)

	// when
	got, err := f.s.Mine(ctx, guildID, member(), 4)
	anonymous, anonymousErr := f.s.Mine(ctx, guildID, reservation.Actor{}, 4)

	// then
	require.NoError(t, err)
	assert.Same(t, page, got)
	require.NoError(t, anonymousErr)
	assert.Empty(t, anonymous.Items)
}

func TestSpotByID(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(8)).Return(nil, ports.ErrNotFound)

	// when
	sp, err := f.s.spotByID(ctx, guildID, 7)
	_, notFound := f.s.spotByID(ctx, guildID, 8)

	// then
	require.NoError(t, err)
	assert.Equal(t, library(), sp)
	assert.ErrorIs(t, notFound, ports.ErrNotFound)
}

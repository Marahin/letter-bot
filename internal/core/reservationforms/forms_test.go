package reservationforms

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestEdit_DraftNeverOverbooks(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.On("SelectGuildSpotByID", ctx, guildID, int64(7)).Return(library(), nil)
	sent := reservation.Draft{SpotID: 7, StartAt: at(6, 10, 0), EndAt: at(6, 12, 0)}
	f.reservations.On("Edit", ctx, guildID, member(), int64(5), sent).Return(nil, nil)

	// when
	outcome, err := f.s.edit(ctx, guildID, member(), 5, reservation.Draft{SpotID: 7, StartAt: at(6, 10, 0), EndAt: at(6, 12, 0), Overbook: true, Author: "x"})

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
	_, notFound := f.s.edit(ctx, guildID, member(), 5, reservation.Draft{SpotID: 7})
	_, failed := f.s.edit(ctx, guildID, member(), 5, reservation.Draft{SpotID: 8})

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

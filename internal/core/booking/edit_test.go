package booking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

var editNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func newEditAdapter(t *testing.T) (*Adapter, *mocks.MockReservationRepository, *mocks.MockSpotRepository) {
	spots := mocks.NewMockSpotRepository(t)
	res := mocks.NewMockReservationRepository(t)
	a := NewAdapter(spots, res, mocks.NewMockCommunicationService(t))
	a.now = func() time.Time { return editNow }
	return a, res, spots
}

func existingReservation() *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID: 7, SpotID: 1, GuildID: "g1", Author: "Quiet Nyx", AuthorDiscordID: "u1",
			StartAt: editNow.Add(time.Hour), EndAt: editNow.Add(2 * time.Hour),
		},
		Spot: reservation.Spot{ID: 1, Name: "Hero Cave"},
	}
}

func editRequest() book.EditRequest {
	return book.EditRequest{
		GuildID: "g1", ReservationID: 7, SpotID: 1,
		StartAt: editNow.Add(time.Hour), EndAt: editNow.Add(3 * time.Hour),
		Author: "Quiet Nyx", AuthorDiscordID: "u1",
	}
}

func TestEdit_UpdatesAndExcludesItselfFromQuotaAndOverlap(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	existing := existingReservation()
	req := editRequest()
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existing, nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, &guild.Guild{ID: "g1"}, &member.Member{ID: "u1"}, int64(7)).
		Return(nil, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(1), req.StartAt, req.EndAt, int64(7)).Return(nil, nil)
	res.EXPECT().UpdateReservation(ctx, "g1", reservation.Reservation{
		ID: 7, SpotID: 1, StartAt: req.StartAt, EndAt: req.EndAt, Author: "Quiet Nyx", AuthorDiscordID: "u1",
	}).Return(nil)

	// when
	conflicts, err := a.Edit(ctx, req)

	// then
	require.NoError(t, err)
	assert.Empty(t, conflicts)
}

func TestEdit_Refusals(t *testing.T) {
	cases := map[string]struct {
		mutate   func(r *book.EditRequest, existing *reservation.ReservationWithSpot)
		expected error
	}{
		"ended": {
			mutate: func(_ *book.EditRequest, e *reservation.ReservationWithSpot) {
				e.StartAt, e.EndAt = editNow.Add(-2*time.Hour), editNow.Add(-time.Minute)
			},
			expected: ErrReservationEnded,
		},
		"end before start": {
			mutate:   func(r *book.EditRequest, _ *reservation.ReservationWithSpot) { r.EndAt = r.StartAt },
			expected: ErrInvalidRange,
		},
		"moved start in the past": {
			mutate: func(r *book.EditRequest, _ *reservation.ReservationWithSpot) {
				r.StartAt = editNow.Add(-time.Hour)
			},
			expected: ErrStartInPast,
		},
		"too long": {
			mutate: func(r *book.EditRequest, _ *reservation.ReservationWithSpot) {
				r.EndAt = r.StartAt.Add(3*time.Hour + time.Minute)
			},
			expected: ErrReservationTooLong,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			a, res, _ := newEditAdapter(t)
			existing := existingReservation()
			req := editRequest()
			tc.mutate(&req, existing)
			res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existing, nil)

			// when
			_, err := a.Edit(ctx, req)

			// then
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestEdit_OngoingReservationKeepsItsStart(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	existing := existingReservation()
	existing.StartAt = editNow.Add(-30 * time.Minute)
	req := editRequest()
	req.StartAt = existing.StartAt
	req.EndAt = editNow.Add(time.Hour)
	req.AuthorDiscordID = ""
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existing, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(1), req.StartAt, req.EndAt, int64(7)).Return(nil, nil)
	res.EXPECT().UpdateReservation(ctx, "g1", mock.Anything).Return(nil)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.NoError(t, err)
}

func TestEdit_QuotaExceeded(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	req := editRequest()
	other := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: 8, StartAt: editNow.Add(5 * time.Hour), EndAt: editNow.Add(7 * time.Hour)},
		Spot:        reservation.Spot{Name: "Dragon Lords"},
	}
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, mock.Anything, mock.Anything, int64(7)).Return([]*reservation.ReservationWithSpot{other}, nil)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestEdit_ConflictReturnsOverlaps(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	req := editRequest()
	blocking := []*reservation.Reservation{{ID: 9, Author: "Storm Quiet"}}
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, mock.Anything, mock.Anything, int64(7)).Return(nil, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(1), req.StartAt, req.EndAt, int64(7)).Return(blocking, nil)

	// when
	conflicts, err := a.Edit(ctx, req)

	// then
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, blocking, conflicts)
}

func TestEdit_ConstraintConflictFromUpdate(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	req := editRequest()
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, mock.Anything, mock.Anything, int64(7)).Return(nil, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(1), req.StartAt, req.EndAt, int64(7)).Return(nil, nil)
	res.EXPECT().UpdateReservation(ctx, "g1", mock.Anything).Return(ports.ErrConflict)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.ErrorIs(t, err, ErrConflict)
}

func TestEdit_ChangedSpot(t *testing.T) {
	archivedAt := editNow.Add(-time.Hour)
	cases := map[string]struct {
		spot     *spot.Spot
		spotErr  error
		expected error
	}{
		"missing":  {spotErr: ports.ErrNotFound, expected: ErrSpotNotFound},
		"archived": {spot: &spot.Spot{ID: 2, Name: "Old", ArchivedAt: &archivedAt}, expected: ErrSpotArchived},
		"broken":   {spotErr: errors.New("db down"), expected: nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			a, res, spots := newEditAdapter(t)
			req := editRequest()
			req.SpotID = 2
			res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
			spots.EXPECT().SelectGuildSpotByID(ctx, "g1", int64(2)).Return(tc.spot, tc.spotErr)

			// when
			_, err := a.Edit(ctx, req)

			// then
			require.Error(t, err)
			if tc.expected != nil {
				assert.ErrorIs(t, err, tc.expected)
			}
		})
	}
}

func TestEdit_MovesToActiveSpot(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, spots := newEditAdapter(t)
	req := editRequest()
	req.SpotID = 2
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
	spots.EXPECT().SelectGuildSpotByID(ctx, "g1", int64(2)).Return(&spot.Spot{ID: 2, Name: "Dragon Lords"}, nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, mock.Anything, mock.Anything, int64(7)).Return(nil, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(2), req.StartAt, req.EndAt, int64(7)).Return(nil, nil)
	res.EXPECT().UpdateReservation(ctx, "g1", mock.MatchedBy(func(r reservation.Reservation) bool { return r.SpotID == 2 })).Return(nil)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.NoError(t, err)
}

func TestEdit_NotFound(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(nil, ports.ErrNotFound)

	// when
	_, err := a.Edit(ctx, editRequest())

	// then
	assert.ErrorIs(t, err, ports.ErrNotFound)
}

func TestDeleteForGuild(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	res.EXPECT().DeleteGuildReservation(ctx, "g1", int64(7)).Return(nil)

	// when
	err := a.DeleteForGuild(ctx, "g1", 7)

	// then
	assert.NoError(t, err)
}

func TestCheckNewWindow(t *testing.T) {
	cases := map[string]struct {
		start, end time.Time
		expected   error
	}{
		"ok":                {editNow.Add(time.Hour), editNow.Add(2 * time.Hour), nil},
		"current minute ok": {editNow.Add(30 * time.Second).Truncate(time.Minute), editNow.Add(time.Hour), nil},
		"empty":             {editNow.Add(time.Hour), editNow.Add(time.Hour), ErrInvalidRange},
		"past":              {editNow.Add(-2 * time.Minute), editNow.Add(time.Hour), ErrStartInPast},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			err := CheckNewWindow(tc.start, tc.end, editNow)

			// then
			if tc.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.expected)
			}
		})
	}
}

func TestBook_FreeTextAuthorSkipsQuota(t *testing.T) {
	// given
	a, res, spots := newEditAdapter(t)
	g := &guild.Guild{ID: "g1"}
	m := &member.Member{Nick: "Someone Else"}
	start, end := editNow.Add(time.Hour), editNow.Add(2*time.Hour)
	spots.EXPECT().SelectGuildSpotByName(mock.Anything, "g1", "Hero Cave").Return(&spot.Spot{ID: 1, Name: "Hero Cave"}, nil)
	res.EXPECT().SelectOverlappingReservations(mock.Anything, int64(1), start, end, "g1").Return(nil, nil)
	res.EXPECT().CreateAndDeleteConflicting(mock.Anything, m, g, []*reservation.Reservation(nil), int64(1), start, end).Return(nil, nil)

	// when
	_, err := a.Book(book.BookRequest{Guild: g, Member: m, Spot: "Hero Cave", StartAt: start, EndAt: end})

	// then
	assert.NoError(t, err)
}

func TestEdit_AuthorizeRefuses(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	existing := existingReservation()
	req := editRequest()
	var seen reservation.Reservation
	req.Authorize = func(r reservation.Reservation) error {
		seen = r
		return assert.AnError
	}
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existing, nil)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, existing.Reservation, seen)
}

func TestEdit_EmptyAuthorKeepsTheCurrentOne(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	req := editRequest()
	req.Author, req.AuthorDiscordID = "", ""
	req.Authorize = func(reservation.Reservation) error { return nil }
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, &guild.Guild{ID: "g1"}, &member.Member{ID: "u1"}, int64(7)).Return(nil, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(1), req.StartAt, req.EndAt, int64(7)).Return(nil, nil)
	res.EXPECT().UpdateReservation(ctx, "g1", mock.MatchedBy(func(r reservation.Reservation) bool {
		return r.Author == "Quiet Nyx" && r.AuthorDiscordID == "u1"
	})).Return(nil)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.NoError(t, err)
}

func TestEdit_StartedReservationKeepsItsSpot(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	existing := existingReservation()
	existing.StartAt = editNow.Add(-30 * time.Minute)
	req := editRequest()
	req.StartAt, req.EndAt, req.SpotID = existing.StartAt, editNow.Add(time.Hour), 2
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existing, nil)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.ErrorIs(t, err, ErrSpotLocked)
}

func TestEdit_DuplicateFromUpdateIsAConflict(t *testing.T) {
	// given
	ctx := context.Background()
	a, res, _ := newEditAdapter(t)
	req := editRequest()
	res.EXPECT().SelectGuildReservationWithSpot(ctx, "g1", int64(7)).Return(existingReservation(), nil)
	res.EXPECT().SelectUpcomingMemberReservationsWithSpots(ctx, mock.Anything, mock.Anything, int64(7)).Return(nil, nil)
	res.EXPECT().SelectOverlappingReservationsBySpotID(ctx, "g1", int64(1), req.StartAt, req.EndAt, int64(7)).Return(nil, nil)
	res.EXPECT().UpdateReservation(ctx, "g1", mock.Anything).Return(ports.ErrDuplicate)

	// when
	_, err := a.Edit(ctx, req)

	// then
	assert.ErrorIs(t, err, ErrConflict)
}

func TestBook_BySpotID(t *testing.T) {
	archivedAt := editNow.Add(-time.Hour)
	cases := map[string]struct {
		spot     *spot.Spot
		spotErr  error
		expected error
	}{
		"missing":  {spotErr: ports.ErrNotFound, expected: ErrSpotNotFound},
		"archived": {spot: &spot.Spot{ID: 2, Name: "Old", ArchivedAt: &archivedAt}, expected: ErrSpotArchived},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			a, _, spots := newEditAdapter(t)
			spots.EXPECT().SelectGuildSpotByID(mock.Anything, "g1", int64(2)).Return(tc.spot, tc.spotErr)

			// when
			_, err := a.Book(book.BookRequest{Guild: &guild.Guild{ID: "g1"}, Member: &member.Member{}, SpotID: 2})

			// then
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestBook_BySpotIDNotifiesOverbookedDiscordAuthorsOnly(t *testing.T) {
	// given
	spots := mocks.NewMockSpotRepository(t)
	res := mocks.NewMockReservationRepository(t)
	comm := mocks.NewMockCommunicationService(t)
	a := NewAdapter(spots, res, comm)
	g := &guild.Guild{ID: "g1"}
	m := &member.Member{Nick: "Boss"}
	start, end := editNow.Add(time.Hour), editNow.Add(2*time.Hour)
	withDiscord := &reservation.Reservation{ID: 8, AuthorDiscordID: "u2", StartAt: start, EndAt: end}
	freeText := &reservation.Reservation{ID: 9, Author: "Someone", StartAt: start, EndAt: end}
	conflicts := []*reservation.Reservation{withDiscord, freeText}
	removed := []*reservation.ClippedOrRemovedReservation{{Original: withDiscord}, {Original: freeText}}
	spots.EXPECT().SelectGuildSpotByID(mock.Anything, "g1", int64(1)).Return(&spot.Spot{ID: 1, Name: "Hero Cave"}, nil)
	res.EXPECT().SelectOverlappingReservations(mock.Anything, int64(1), start, end, "g1").Return(conflicts, nil)
	res.EXPECT().CreateAndDeleteConflicting(mock.Anything, m, g, conflicts, int64(1), start, end).Return(removed, nil)
	notified := make(chan book.BookRequest, 2)
	comm.EXPECT().NotifyOverbookedMember(mock.Anything, removed[0]).Run(func(r book.BookRequest, _ *reservation.ClippedOrRemovedReservation) {
		notified <- r
	}).Once()

	// when
	got, err := a.Book(book.BookRequest{Guild: g, Member: m, SpotID: 1, StartAt: start, EndAt: end, Overbook: true, HasPermissions: true})

	// then
	require.NoError(t, err)
	assert.Equal(t, removed, got)
	select {
	case r := <-notified:
		assert.Equal(t, "Hero Cave", r.Spot)
	case <-time.After(time.Second):
		t.Fatal("the Discord author was not notified")
	}
}

package reservations

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
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

var now = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

type fixture struct {
	s        *Service
	booker   *mocks.MockBookingService
	repo     *mocks.MockReservationRepository
	spots    *mocks.MockSpotRepository
	notifier *mocks.MockBotNotifier
}

func newFixture(t *testing.T) fixture {
	f := fixture{
		booker:   mocks.NewMockBookingService(t),
		repo:     mocks.NewMockReservationRepository(t),
		spots:    mocks.NewMockSpotRepository(t),
		notifier: mocks.NewMockBotNotifier(t),
	}
	f.s = New(f.booker, f.repo, f.spots, f.notifier, nil)
	f.s.now = func() time.Time { return now }
	return f
}

func member1() reservation.Actor {
	return reservation.Actor{UserID: "u1", Name: "Quiet Nyx", Caps: permission.Capabilities{View: true, Reserve: true}}
}

func manager() reservation.Actor {
	return reservation.Actor{UserID: "m1", Name: "Boss", Caps: permission.Capabilities{View: true, Reserve: true, Manage: true, Overbook: true}}
}

func viewer() reservation.Actor {
	return reservation.Actor{UserID: "v1", Name: "Watcher", Caps: permission.Capabilities{View: true}}
}

func upcoming(authorID string) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: 7, SpotID: 1, Author: "Someone", AuthorDiscordID: authorID, StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour)},
		Spot:        reservation.Spot{ID: 1, Name: "Hero Cave"},
	}
}

func past(authorID string) *reservation.ReservationWithSpot {
	r := upcoming(authorID)
	r.StartAt, r.EndAt = now.Add(-3*time.Hour), now.Add(-time.Hour)
	return r
}

func draft() reservation.Draft {
	return reservation.Draft{SpotID: 1, StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour)}
}

func TestCanEditAndCanDelete(t *testing.T) {
	cases := map[string]struct {
		actor      reservation.Actor
		r          *reservation.ReservationWithSpot
		edit, drop bool
	}{
		"owner upcoming":            {member1(), upcoming("u1"), true, true},
		"owner past":                {member1(), past("u1"), false, false},
		"other member":              {member1(), upcoming("u2"), false, false},
		"manager upcoming":          {manager(), upcoming("u2"), true, true},
		"manager past":              {manager(), past("u2"), false, true},
		"viewer own id":             {viewer(), upcoming("v1"), false, false},
		"empty id never owns":       {reservation.Actor{Caps: permission.Capabilities{Reserve: true}}, upcoming(""), false, false},
		"owner without reserve now": {reservation.Actor{UserID: "u1", Caps: permission.Capabilities{View: true}}, upcoming("u1"), false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			edit := CanEdit(tc.actor, tc.r.Reservation, now)
			drop := CanDelete(tc.actor, tc.r.Reservation, now)

			// then
			assert.Equal(t, tc.edit, edit, "edit")
			assert.Equal(t, tc.drop, drop, "delete")
		})
	}
}

func TestNormalizeFilter(t *testing.T) {
	// given
	from, to := now, now.Add(-24*time.Hour)
	cases := map[string]struct {
		in       reservation.SearchFilter
		expected reservation.SearchFilter
	}{
		"defaults": {
			in:       reservation.SearchFilter{GuildID: guildID, Author: "  nyx "},
			expected: reservation.SearchFilter{GuildID: guildID, Author: "nyx", Scope: reservation.ScopeUpcoming, Limit: DefaultPerPage},
		},
		"unknown scope and huge limit": {
			in:       reservation.SearchFilter{Scope: "soon", Limit: 1000},
			expected: reservation.SearchFilter{Scope: reservation.ScopeUpcoming, Limit: MaxPerPage},
		},
		"swapped dates kept scope": {
			in:       reservation.SearchFilter{Scope: reservation.ScopePast, From: &from, To: &to, Limit: 10},
			expected: reservation.SearchFilter{Scope: reservation.ScopePast, From: &to, To: &from, Limit: 10},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			got := NormalizeFilter(tc.in)

			// then
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSearch_PagesAndClampsPage(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	filter := reservation.SearchFilter{GuildID: guildID, Scope: reservation.ScopeAll}
	normalized := NormalizeFilter(filter)
	f.repo.EXPECT().CountReservations(ctx, normalized).Return(120, nil)
	items := []*reservation.ReservationWithSpot{upcoming("u1")}
	withOffset := normalized
	withOffset.Offset = 100
	f.repo.EXPECT().SearchReservationsWithSpot(ctx, withOffset).Return(items, nil)

	// when
	page, err := f.s.Search(ctx, filter, 9)

	// then
	require.NoError(t, err)
	assert.Equal(t, &reservation.Page{Items: items, Total: 120, Page: 3, Pages: 3, PerPage: 50}, page)
}

func TestSearch_EmptySkipsTheQuery(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.repo.EXPECT().CountReservations(ctx, mock.Anything).Return(0, nil)

	// when
	page, err := f.s.Search(ctx, reservation.SearchFilter{GuildID: guildID}, 0)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 0, page.Pages)
	assert.Empty(t, page.Items)
}

func TestSearch_Errors(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.repo.EXPECT().CountReservations(ctx, mock.Anything).Return(0, errors.New("down")).Once()
	f.repo.EXPECT().CountReservations(ctx, mock.Anything).Return(3, nil).Once()
	f.repo.EXPECT().SearchReservationsWithSpot(ctx, mock.Anything).Return(nil, errors.New("down"))

	// when
	_, countErr := f.s.Search(ctx, reservation.SearchFilter{}, 1)
	_, searchErr := f.s.Search(ctx, reservation.SearchFilter{}, 1)

	// then
	assert.Error(t, countErr)
	assert.Error(t, searchErr)
}

func TestKnownAuthors(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	authors := []*reservation.KnownAuthor{{AuthorDiscordID: "u1", Author: "Quiet Nyx"}}
	f.repo.EXPECT().SelectKnownAuthors(ctx, guildID, "ny").Return(authors, nil)
	f.repo.EXPECT().SelectKnownAuthors(ctx, guildID, "zz").Return(nil, errors.New("down"))

	// when
	short, shortErr := f.s.KnownAuthors(ctx, guildID, " n ")
	got, err := f.s.KnownAuthors(ctx, guildID, " ny ")
	_, failErr := f.s.KnownAuthors(ctx, guildID, "zz")

	// then
	assert.NoError(t, shortErr)
	assert.Empty(t, short)
	assert.NoError(t, err)
	assert.Equal(t, authors, got)
	assert.Error(t, failErr)
}

func TestGet(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(upcoming("u1"), nil)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(8)).Return(nil, ports.ErrNotFound)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(9)).Return(nil, errors.New("down"))

	// when
	r, err := f.s.Get(ctx, guildID, 7)
	_, missing := f.s.Get(ctx, guildID, 8)
	_, broken := f.s.Get(ctx, guildID, 9)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(7), r.Reservation.ID)
	assert.ErrorIs(t, missing, ErrNotFound)
	assert.Error(t, broken)
	assert.NotErrorIs(t, broken, ErrNotFound)
}

func TestCreate_MemberBooksAsThemselves(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	d := draft()
	d.Author, d.AuthorDiscordID, d.Overbook = "Not Me", "u9", true
	f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1, Name: "Hero Cave"}, nil)
	f.booker.EXPECT().Book(book.BookRequest{
		Guild:    &guild.Guild{ID: guildID},
		Member:   &member.Member{ID: "u1", Nick: "Quiet Nyx", Username: "Quiet Nyx"},
		Spot:     "Hero Cave",
		StartAt:  d.StartAt,
		EndAt:    d.EndAt,
		Overbook: true,
	}).Return(nil, nil)
	f.notifier.EXPECT().SummaryChanged(ctx, guildID).Return(errors.New("notify is best effort"))

	// when
	_, err := f.s.Create(ctx, guildID, member1(), d)

	// then
	assert.NoError(t, err)
}

func TestCreate_ManagerBooksForAnotherAuthor(t *testing.T) {
	cases := map[string]struct {
		author, authorID     string
		wantNick, wantMember string
	}{
		"known author": {" Storm Quiet ", "u2", "Storm Quiet", "u2"},
		"free text":    {"Guest Hunter", "", "Guest Hunter", ""},
		"empty author": {"  ", "u2", "Boss", "m1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			f := newFixture(t)
			d := draft()
			d.Author, d.AuthorDiscordID = tc.author, tc.authorID
			overbooked := []*reservation.ClippedOrRemovedReservation{{Original: &reservation.Reservation{ID: 3}}}
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1, Name: "Hero Cave"}, nil)
			f.booker.EXPECT().Book(mock.MatchedBy(func(r book.BookRequest) bool {
				return r.Member.ID == tc.wantMember && r.Member.Nick == tc.wantNick && r.HasPermissions
			})).Return(overbooked, nil)
			f.notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

			// when
			res, err := f.s.Create(ctx, guildID, manager(), d)

			// then
			require.NoError(t, err)
			assert.Equal(t, overbooked, res)
		})
	}
}

func TestCreate_Refusals(t *testing.T) {
	archivedAt := now
	long := make([]rune, MaxAuthorLength+1)
	for i := range long {
		long[i] = 'a'
	}
	cases := map[string]struct {
		actor    reservation.Actor
		draft    func() reservation.Draft
		spot     *spot.Spot
		spotErr  error
		expected error
	}{
		"viewer":          {actor: viewer(), draft: draft, expected: ErrForbidden},
		"author too long": {actor: manager(), draft: func() reservation.Draft { d := draft(); d.Author = string(long); return d }, expected: ErrAuthorTooLong},
		"past start": {actor: member1(), draft: func() reservation.Draft {
			d := draft()
			d.StartAt = now.Add(-time.Hour)
			return d
		}, expected: booking.ErrStartInPast},
		"unknown spot":  {actor: member1(), draft: draft, spotErr: ports.ErrNotFound, expected: booking.ErrSpotNotFound},
		"archived spot": {actor: member1(), draft: draft, spot: &spot.Spot{ID: 1, ArchivedAt: &archivedAt}, expected: booking.ErrSpotArchived},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			f := newFixture(t)
			if tc.spot != nil || tc.spotErr != nil {
				f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(tc.spot, tc.spotErr)
			}

			// when
			_, err := f.s.Create(ctx, guildID, tc.actor, tc.draft())

			// then
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestCreate_SpotLookupFails(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(nil, errors.New("down"))

	// when
	_, err := f.s.Create(ctx, guildID, member1(), draft())

	// then
	assert.Error(t, err)
}

func TestCreate_ConflictKeepsBlockingReservations(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	blocking := []*reservation.ClippedOrRemovedReservation{{Original: &reservation.Reservation{ID: 3, Author: "Storm Quiet"}}}
	f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1, Name: "Hero Cave"}, nil)
	f.booker.EXPECT().Book(mock.Anything).Return(blocking, booking.ErrInsufficientPermissions)

	// when
	res, err := f.s.Create(ctx, guildID, member1(), draft())

	// then
	assert.ErrorIs(t, err, booking.ErrInsufficientPermissions)
	assert.Equal(t, blocking, res)
}

func TestEdit_OwnerKeepsAuthor(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	d := draft()
	d.Author, d.AuthorDiscordID = "Hijack", "u9"
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(upcoming("u1"), nil)
	f.booker.EXPECT().Edit(ctx, book.EditRequest{
		GuildID: guildID, ReservationID: 7, SpotID: 1, StartAt: d.StartAt, EndAt: d.EndAt,
		Author: "Someone", AuthorDiscordID: "u1",
	}).Return(nil, nil)
	f.notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	_, err := f.s.Edit(ctx, guildID, member1(), 7, d)

	// then
	assert.NoError(t, err)
}

func TestEdit_ManagerChangesAuthor(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	d := draft()
	d.Author, d.AuthorDiscordID = "Storm Quiet", "u2"
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(upcoming("u1"), nil)
	f.booker.EXPECT().Edit(ctx, mock.MatchedBy(func(r book.EditRequest) bool {
		return r.Author == "Storm Quiet" && r.AuthorDiscordID == "u2"
	})).Return(nil, nil)
	f.notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	_, err := f.s.Edit(ctx, guildID, manager(), 7, d)

	// then
	assert.NoError(t, err)
}

func TestEdit_Refusals(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	blocking := []*reservation.Reservation{{ID: 9}}
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(upcoming("u2"), nil)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(8)).Return(nil, ports.ErrNotFound)
	f.booker.EXPECT().Edit(ctx, mock.Anything).Return(blocking, booking.ErrConflict).Once()
	f.booker.EXPECT().Edit(ctx, mock.Anything).Return(nil, ports.ErrNotFound).Once()

	// when
	_, forbidden := f.s.Edit(ctx, guildID, member1(), 7, draft())
	_, missing := f.s.Edit(ctx, guildID, manager(), 8, draft())
	conflicts, conflict := f.s.Edit(ctx, guildID, manager(), 7, draft())
	_, gone := f.s.Edit(ctx, guildID, manager(), 7, draft())

	// then
	assert.ErrorIs(t, forbidden, ErrForbidden)
	assert.ErrorIs(t, missing, ErrNotFound)
	assert.ErrorIs(t, conflict, booking.ErrConflict)
	assert.Equal(t, blocking, conflicts)
	assert.ErrorIs(t, gone, ErrNotFound)
}

func TestEdit_EndedReservation(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(past("u2"), nil)

	// when
	_, err := f.s.Edit(ctx, guildID, manager(), 7, draft())

	// then
	assert.ErrorIs(t, err, booking.ErrReservationEnded)
}

func TestEdit_AuthorTooLong(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	d := draft()
	for range MaxAuthorLength + 1 {
		d.Author += "x"
	}
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(upcoming("u2"), nil)

	// when
	_, err := f.s.Edit(ctx, guildID, manager(), 7, d)

	// then
	assert.ErrorIs(t, err, ErrAuthorTooLong)
}

func TestDelete(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(past("u2"), nil)
	f.booker.EXPECT().DeleteForGuild(ctx, guildID, int64(7)).Return(nil)
	f.notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	err := f.s.Delete(ctx, guildID, manager(), 7)

	// then
	assert.NoError(t, err)
}

func TestDelete_Refusals(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(7)).Return(past("u1"), nil)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(8)).Return(nil, ports.ErrNotFound)
	f.repo.EXPECT().SelectGuildReservationWithSpot(ctx, guildID, int64(9)).Return(upcoming("u1"), nil)
	f.booker.EXPECT().DeleteForGuild(ctx, guildID, int64(9)).Return(ports.ErrNotFound).Once()
	f.booker.EXPECT().DeleteForGuild(ctx, guildID, int64(9)).Return(errors.New("down")).Once()

	// when
	pastOwn := f.s.Delete(ctx, guildID, member1(), 7)
	missing := f.s.Delete(ctx, guildID, member1(), 8)
	raced := f.s.Delete(ctx, guildID, member1(), 9)
	broken := f.s.Delete(ctx, guildID, member1(), 9)

	// then
	assert.ErrorIs(t, pastOwn, ErrForbidden)
	assert.ErrorIs(t, missing, ErrNotFound)
	assert.ErrorIs(t, raced, ErrNotFound)
	assert.Error(t, broken)
}

func TestSpots(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	all := []*spot.Spot{{ID: 1, Name: "Hero Cave"}}
	f.spots.EXPECT().SelectGuildSpots(ctx, guildID, true).Return(all, nil).Once()
	f.spots.EXPECT().SelectGuildSpots(ctx, guildID, true).Return(nil, errors.New("down")).Once()

	// when
	got, err := f.s.Spots(ctx, guildID)
	_, broken := f.s.Spots(ctx, guildID)

	// then
	require.NoError(t, err)
	assert.Equal(t, all, got)
	assert.Error(t, broken)
}

func TestSpotOverview(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	spotID := int64(1)
	f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, spotID).Return(&spot.Spot{ID: 1, Name: "Hero Cave"}, nil)
	f.repo.EXPECT().CountReservations(ctx, reservation.SearchFilter{GuildID: guildID, SpotID: &spotID, Scope: reservation.ScopeAll}).Return(12, nil)
	f.repo.EXPECT().CountReservations(ctx, reservation.SearchFilter{GuildID: guildID, SpotID: &spotID, Scope: reservation.ScopeUpcoming}).Return(2, nil)

	// when
	got, err := f.s.SpotOverview(ctx, guildID, spotID)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Hero Cave", got.Name)
	assert.Equal(t, spot.ReservationCounts{Total: 12, Upcoming: 2}, got.Reservations)
}

func TestSpotOverview_Errors(t *testing.T) {
	cases := map[string]struct {
		spotErr, allErr, upcomingErr error
		expected                     error
	}{
		"missing spot":   {spotErr: ports.ErrNotFound, expected: ports.ErrNotFound},
		"count fails":    {allErr: errors.New("down")},
		"upcoming fails": {upcomingErr: errors.New("down")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			f := newFixture(t)
			if tc.spotErr != nil {
				f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(nil, tc.spotErr)
			} else {
				f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1}, nil)
				f.repo.EXPECT().CountReservations(ctx, mock.MatchedBy(func(r reservation.SearchFilter) bool { return r.Scope == reservation.ScopeAll })).Return(1, tc.allErr)
				if tc.allErr == nil {
					f.repo.EXPECT().CountReservations(ctx, mock.MatchedBy(func(r reservation.SearchFilter) bool { return r.Scope == reservation.ScopeUpcoming })).Return(1, tc.upcomingErr)
				}
			}

			// when
			_, err := f.s.SpotOverview(ctx, guildID, 1)

			// then
			require.Error(t, err)
			if tc.expected != nil {
				assert.ErrorIs(t, err, tc.expected)
			}
		})
	}
}

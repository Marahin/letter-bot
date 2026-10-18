package queue

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
	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

var (
	ctx = context.Background()
	// now is off the minute, as a real click is.
	now     = time.Date(2026, 10, 18, 18, 0, 20, 500, time.UTC)
	nowSec  = now.Truncate(time.Second)
	start   = time.Date(2026, 10, 18, 18, 1, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

type fixture struct {
	s     *Service
	repo  *mocks.MockSpotQueueRepository
	spots *mocks.MockSpotRepository
	forms *mocks.MockReservationFormService
}

func newFixture(t *testing.T) fixture {
	f := fixture{
		repo:  mocks.NewMockSpotQueueRepository(t),
		spots: mocks.NewMockSpotRepository(t),
		forms: mocks.NewMockReservationFormService(t),
	}
	f.s = New(f.repo, f.spots, f.forms)
	f.s.now = func() time.Time { return now }
	return f
}

func knight() reservation.Actor {
	return reservation.Actor{UserID: "u1", Name: "Knight", Caps: permission.Capabilities{View: true, Reserve: true}}
}

func library() *spot.Spot {
	return &spot.Spot{ID: 7, Name: "Library -1", GuildID: guildID}
}

func entry(id int64, offeredAt *time.Time) queue.Entry {
	return queue.Entry{ID: id, GuildID: guildID, SpotID: 7, SpotName: "Library -1", UserID: "u1", Length: 2 * time.Hour, OfferedAt: offeredAt}
}

func ptr(t time.Time) *time.Time { return &t }

func TestBookingStart(t *testing.T) {
	// given
	onMinute := time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

	// when
	off, on := BookingStart(now), BookingStart(onMinute)

	// then
	assert.Equal(t, start, off)
	assert.Equal(t, onMinute, on)
}

func joinRequest(actor reservation.Actor, length time.Duration) queue.JoinRequest {
	return queue.JoinRequest{GuildID: guildID, Actor: actor, SpotID: 7, Length: length}
}

func userCount() queue.UserCount {
	return queue.UserCount{GuildID: guildID, UserID: "u1", ExceptSpotID: 7, Now: now}
}

func claim(entryID int64) queue.Claim {
	return queue.Claim{EntryID: entryID, OfferedAt: nowSec, StartAt: start}
}

func TestJoin_Refusals(t *testing.T) {
	archived := library()
	archived.ArchivedAt = ptr(now)
	tests := []struct {
		name   string
		actor  reservation.Actor
		length time.Duration
		setup  func(f fixture)
		want   error
	}{
		{name: "no reserve right", actor: reservation.Actor{UserID: "u1"}, want: booking.ErrReserveNotAllowed},
		{name: "too short", actor: knight(), length: 15 * time.Minute, want: ErrInvalidLength},
		{name: "too long", actor: knight(), length: 210 * time.Minute, want: ErrInvalidLength},
		{name: "off the step", actor: knight(), length: 45 * time.Minute, want: ErrInvalidLength},
		{name: "unknown spot", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(nil, ports.ErrNotFound)
		}, want: booking.ErrSpotNotFound},
		{name: "archived spot", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(archived, nil)
		}, want: booking.ErrSpotArchived},
		{name: "spot read fails", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(nil, errBoom)
		}, want: errBoom},
		{name: "queue full", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(library(), nil)
			f.repo.EXPECT().CountUserEntries(ctx, userCount()).Return(MaxEntriesPerUser, nil)
		}, want: ErrQueueFull},
		{name: "count fails", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(library(), nil)
			f.repo.EXPECT().CountUserEntries(ctx, userCount()).Return(0, errBoom)
		}, want: errBoom},
		{name: "upsert fails", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(library(), nil)
			f.repo.EXPECT().CountUserEntries(ctx, userCount()).Return(2, nil)
			f.repo.EXPECT().UpsertEntry(ctx, mock.Anything).Return(nil, errBoom)
		}, want: errBoom},
		{name: "claim fails", actor: knight(), setup: func(f fixture) {
			f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(library(), nil)
			f.repo.EXPECT().CountUserEntries(ctx, userCount()).Return(0, nil)
			e := entry(1, nil)
			f.repo.EXPECT().UpsertEntry(ctx, mock.Anything).Return(&e, nil)
			f.repo.EXPECT().ClaimOffer(ctx, claim(1)).Return(false, errBoom)
		}, want: errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}

			// when
			out, err := f.s.Join(ctx, joinRequest(tt.actor, tt.length))

			// then
			require.ErrorIs(t, err, tt.want)
			assert.Nil(t, out)
		})
	}
}

func TestJoin_TakenSpotWaits(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(library(), nil)
	f.repo.EXPECT().CountUserEntries(ctx, userCount()).Return(2, nil)
	stored := entry(1, nil)
	stored.Length = 90 * time.Minute
	f.repo.EXPECT().UpsertEntry(ctx, queue.Entry{
		GuildID: guildID, SpotID: 7, SpotName: "Library -1", UserID: "u1", Length: 90 * time.Minute, ExpiresAt: now.Add(EntryTTL),
	}).Return(&stored, nil)
	f.repo.EXPECT().ClaimOffer(ctx, claim(1)).Return(false, nil)

	// when
	out, err := f.s.Join(ctx, joinRequest(knight(), 90*time.Minute))

	// then
	require.NoError(t, err)
	assert.Equal(t, &queue.JoinOutcome{Entry: stored}, out)
}

func TestJoin_FreeSpotGetsTheOfferAtOnce(t *testing.T) {
	// given
	f := newFixture(t)
	f.spots.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(library(), nil)
	f.repo.EXPECT().CountUserEntries(ctx, userCount()).Return(0, nil)
	stored := entry(1, nil)
	f.repo.EXPECT().UpsertEntry(ctx, mock.MatchedBy(func(e queue.Entry) bool { return e.Length == DefaultLength })).Return(&stored, nil)
	f.repo.EXPECT().ClaimOffer(ctx, claim(1)).Return(true, nil)

	// when
	out, err := f.s.Join(ctx, joinRequest(knight(), 0))

	// then
	require.NoError(t, err)
	assert.True(t, out.FreeNow)
	assert.Equal(t, ptr(nowSec), out.Entry.OfferedAt)
}

func TestLeave(t *testing.T) {
	// given
	f := newFixture(t)
	f.repo.EXPECT().DeleteEntry(ctx, int64(1), "u1").Return(true, nil)
	f.repo.EXPECT().DeleteEntry(ctx, int64(2), "u1").Return(false, nil)
	f.repo.EXPECT().DeleteEntry(ctx, int64(3), "u1").Return(false, errBoom)

	// when
	left := f.s.Leave(ctx, "u1", 1)
	gone := f.s.Leave(ctx, "u1", 2)
	failed := f.s.Leave(ctx, "u1", 3)

	// then
	assert.NoError(t, left)
	assert.ErrorIs(t, gone, ports.ErrNotFound)
	assert.ErrorIs(t, failed, errBoom)
}

func TestMineAndEntry(t *testing.T) {
	// given
	f := newFixture(t)
	e := entry(1, nil)
	f.repo.EXPECT().ListUserEntries(ctx, guildID, "u1", now).Return([]queue.Entry{e}, nil)
	f.repo.EXPECT().GetEntry(ctx, int64(1)).Return(&e, nil)

	// when
	mine, mineErr := f.s.Mine(ctx, guildID, "u1")
	got, getErr := f.s.Entry(ctx, 1)

	// then
	require.NoError(t, errors.Join(mineErr, getErr))
	assert.Equal(t, []queue.Entry{e}, mine)
	assert.Equal(t, &e, got)
}

func TestDueOffers(t *testing.T) {
	// given
	f := newFixture(t)
	f.repo.EXPECT().DeleteExpired(ctx, now).Return(1, nil)
	f.repo.EXPECT().ResetTakenOffers(ctx, guildID, start).Return(nil)
	f.repo.EXPECT().SelectDueEntries(ctx, queue.DueQuery{GuildID: guildID, Now: now, StartAt: start}).Return([]queue.Entry{entry(1, nil)}, nil)

	// when
	due, err := f.s.DueOffers(ctx, guildID)

	// then
	require.NoError(t, err)
	assert.Equal(t, []queue.Entry{entry(1, nil)}, due)
}

func TestDueOffers_Failures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f fixture)
	}{
		{name: "expiry cleanup fails", setup: func(f fixture) {
			f.repo.EXPECT().DeleteExpired(ctx, now).Return(0, errBoom)
		}},
		{name: "reset fails", setup: func(f fixture) {
			f.repo.EXPECT().DeleteExpired(ctx, now).Return(0, nil)
			f.repo.EXPECT().ResetTakenOffers(ctx, guildID, start).Return(errBoom)
		}},
		{name: "select fails", setup: func(f fixture) {
			f.repo.EXPECT().DeleteExpired(ctx, now).Return(0, nil)
			f.repo.EXPECT().ResetTakenOffers(ctx, guildID, start).Return(nil)
			f.repo.EXPECT().SelectDueEntries(ctx, mock.Anything).Return(nil, errBoom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFixture(t)
			tt.setup(f)

			// when
			due, err := f.s.DueOffers(ctx, guildID)

			// then
			require.ErrorIs(t, err, errBoom)
			assert.Empty(t, due)
		})
	}
}

func TestClaimOffer(t *testing.T) {
	// given
	f := newFixture(t)
	f.repo.EXPECT().ClaimOffer(ctx, claim(1)).Return(true, nil)
	f.repo.EXPECT().ClaimOffer(ctx, claim(2)).Return(false, nil)
	f.repo.EXPECT().ClaimOffer(ctx, claim(3)).Return(false, errBoom)

	// when
	at, claimed, err := f.s.ClaimOffer(ctx, 1)
	lostAt, lost, lostErr := f.s.ClaimOffer(ctx, 2)
	_, _, failErr := f.s.ClaimOffer(ctx, 3)

	// then
	require.NoError(t, errors.Join(err, lostErr))
	assert.True(t, claimed)
	assert.Equal(t, nowSec, at)
	assert.False(t, lost)
	assert.True(t, lostAt.IsZero())
	assert.ErrorIs(t, failErr, errBoom)
}

func TestUnclaim(t *testing.T) {
	// given
	f := newFixture(t)
	f.repo.EXPECT().ResetOffer(ctx, int64(4), nowSec).Return(nil)

	// when
	err := f.s.Unclaim(ctx, 4, nowSec)

	// then
	assert.NoError(t, err)
}

func TestOfferUsable(t *testing.T) {
	offered := now.Add(-5 * time.Minute)
	tests := []struct {
		name      string
		entry     queue.Entry
		offeredAt time.Time
		want      error
	}{
		{name: "current offer", entry: entry(1, ptr(offered)), offeredAt: offered},
		{name: "cleared after the spot was taken again", entry: entry(1, nil), offeredAt: offered},
		{name: "last second", entry: entry(1, ptr(now.Add(-OfferTTL))), offeredAt: now.Add(-OfferTTL)},
		{name: "past the time gate", entry: entry(1, nil), offeredAt: now.Add(-OfferTTL - time.Second), want: ErrOfferExpired},
		{name: "from the future", entry: entry(1, nil), offeredAt: now.Add(time.Minute), want: ErrOfferExpired},
		{name: "a newer offer replaced it", entry: entry(1, ptr(now.Add(-time.Minute))), offeredAt: offered, want: ErrOfferExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFixture(t)

			// when
			err := f.s.OfferUsable(&tt.entry, tt.offeredAt)

			// then
			assert.ErrorIs(t, err, tt.want)
			if tt.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBook(t *testing.T) {
	// given
	f := newFixture(t)
	offered := nowSec.Add(-time.Minute)
	e := entry(1, ptr(offered))
	f.repo.EXPECT().GetEntry(ctx, int64(1)).Return(&e, nil)
	draft := reservation.Draft{SpotID: 7, StartAt: start, EndAt: start.Add(2 * time.Hour)}
	outcome := &reservation.FormOutcome{Draft: draft, SpotName: "Library -1"}
	f.forms.EXPECT().Book(ctx, guildID, knight(), draft).Return(outcome, nil)

	// when
	got, err := f.s.Book(ctx, knight(), 1, offered)

	// then
	require.NoError(t, err)
	assert.Equal(t, outcome, got)
}

func TestBook_Refusals(t *testing.T) {
	offered := nowSec.Add(-time.Minute)
	tests := []struct {
		name  string
		setup func(f fixture)
		want  error
	}{
		{name: "entry gone", setup: func(f fixture) {
			f.repo.EXPECT().GetEntry(ctx, int64(1)).Return(nil, ports.ErrNotFound)
		}, want: ports.ErrNotFound},
		{name: "not the owner", setup: func(f fixture) {
			e := entry(1, ptr(offered))
			e.UserID = "u2"
			f.repo.EXPECT().GetEntry(ctx, int64(1)).Return(&e, nil)
		}, want: ErrOfferNotYours},
		{name: "replaced by a newer offer", setup: func(f fixture) {
			e := entry(1, ptr(nowSec))
			f.repo.EXPECT().GetEntry(ctx, int64(1)).Return(&e, nil)
		}, want: ErrOfferExpired},
		{name: "taken in the meantime", setup: func(f fixture) {
			e := entry(1, nil)
			f.repo.EXPECT().GetEntry(ctx, int64(1)).Return(&e, nil)
			f.forms.EXPECT().Book(ctx, guildID, knight(), mock.Anything).Return(&reservation.FormOutcome{}, booking.ErrInsufficientPermissions)
		}, want: booking.ErrInsufficientPermissions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFixture(t)
			tt.setup(f)

			// when
			_, err := f.s.Book(ctx, knight(), 1, offered)

			// then
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

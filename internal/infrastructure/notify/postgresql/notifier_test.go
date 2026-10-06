package postgresql

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func TestNotifier_SendsEachChannel(t *testing.T) {
	cases := map[string]struct {
		send    func(n *Notifier) error
		channel string
		payload string
	}{
		"summary": {func(n *Notifier) error { return n.SummaryChanged(context.Background(), "g1") }, ChannelSummaryRefresh, "g1"},
		"resync":  {func(n *Notifier) error { return n.ResyncRequested(context.Background(), "g1") }, ChannelGuildResync, "g1"},
		"config":  {func(n *Notifier) error { return n.ConfigChanged(context.Background(), "g1") }, ChannelGuildConfig, "g1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			db, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer db.Close()
			db.ExpectExec(`SELECT pg_notify\(\$1, \$2\)`).WithArgs(tc.channel, tc.payload).WillReturnResult(pgxmock.NewResult("SELECT", 1))

			// when
			err = tc.send(NewNotifier(db))

			// then
			assert.NoError(t, err)
			assert.NoError(t, db.ExpectationsWereMet())
		})
	}
}

func TestNotifier_WrapsError(t *testing.T) {
	// given
	db, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer db.Close()
	db.ExpectExec(`SELECT pg_notify`).WithArgs(ChannelGuildConfig, "g1").WillReturnError(assert.AnError)

	// when
	err = NewNotifier(db).ConfigChanged(context.Background(), "g1")

	// then
	assert.ErrorIs(t, err, assert.AnError)
	assert.ErrorContains(t, err, ChannelGuildConfig)
}

func TestNotifier_OverbookedSendsTheEncodedPayload(t *testing.T) {
	// given
	db, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer db.Close()
	request := book.BookRequest{Guild: &guild.Guild{ID: "g1"}, Member: &member.Member{ID: "m1"}, Spot: "Hero Cave"}
	res := &reservation.ClippedOrRemovedReservation{Original: &reservation.Reservation{ID: 3, AuthorDiscordID: "u2"}}
	payload, err := NewOverbookedPayload(request, res).Encode()
	require.NoError(t, err)
	db.ExpectExec(`SELECT pg_notify`).WithArgs(ChannelOverbooked, string(payload)).WillReturnResult(pgxmock.NewResult("SELECT", 1))

	// when
	err = NewNotifier(db).Overbooked(context.Background(), request, res)

	// then
	assert.NoError(t, err)
	assert.NoError(t, db.ExpectationsWereMet())
}

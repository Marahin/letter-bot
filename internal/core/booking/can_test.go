package booking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/reservation"
)

func TestOverbookAllowed_trueWhenHasPermissions(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := true
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.True(output)
}

func TestOverbookAllowed_falseWithoutPermissionsOrAbandonedReservation(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.False(output)
}

func TestOverbookAllowed_falseWhenReservationStartedLessThan10MinutesAgo(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-5 * time.Minute), // Started 5 mins ago
			EndAt:   now.Add(55 * time.Minute),
		},
	}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.False(output)
}

func TestOverbookAllowed_trueWhenReservationStartedMoreThan10MinutesAgo(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-11 * time.Minute), // Started 11 mins ago
			EndAt:   now.Add(49 * time.Minute),
		},
	}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.True(output)
}

func TestOverbookAllowed_falseWhenMultipleConflictingReservations(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-15 * time.Minute),
			EndAt:   now.Add(45 * time.Minute),
		},
		{
			StartAt: now.Add(-10 * time.Minute),
			EndAt:   now.Add(50 * time.Minute),
		},
	}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.False(output)
}

func TestOverbookAllowed_falseWhenReservationHasNotStartedYet(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(5 * time.Minute), // Starts in future
			EndAt:   now.Add(65 * time.Minute),
		},
	}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.False(output)
}

func TestOverbookAllowed_falseWhenReservationAlreadyEnded(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-2 * time.Hour),
			EndAt:   now.Add(-1 * time.Hour), // Ended in past
		},
	}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.False(output)
}

func TestOverbookAllowed_trueWhenPrivilegedUserOverbooksEarly(t *testing.T) {
	// given
	is := assert.New(t)
	hasPermissions := true
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-5 * time.Minute), // Started 5 mins ago
			EndAt:   now.Add(55 * time.Minute),
		},
	}

	// when
	output := OverbookAllowed(hasPermissions, "", conflictingReservations, now)

	// assert
	is.True(output)
}

func TestOverbookAllowed(t *testing.T) {
	// given
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	upcoming := &reservation.Reservation{StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour), AuthorDiscordID: "other"}
	abandoned := &reservation.Reservation{StartAt: now.Add(-20 * time.Minute), EndAt: now.Add(time.Hour), AuthorDiscordID: "other"}
	own := &reservation.Reservation{StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour), AuthorDiscordID: "me"}
	tests := []struct {
		name           string
		hasPermissions bool
		conflicts      []*reservation.Reservation
		want           bool
	}{
		{"with the right", true, []*reservation.Reservation{upcoming}, true},
		{"without the right", false, []*reservation.Reservation{upcoming}, false},
		{"an abandoned one without the right", false, []*reservation.Reservation{abandoned}, true},
		{"two with an abandoned one", false, []*reservation.Reservation{abandoned, upcoming}, false},
		{"own reservation with the right", true, []*reservation.Reservation{upcoming, own}, false},
		{"own abandoned reservation", false, []*reservation.Reservation{{StartAt: abandoned.StartAt, EndAt: abandoned.EndAt, AuthorDiscordID: "me"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := OverbookAllowed(tt.hasPermissions, "me", tt.conflicts, now)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

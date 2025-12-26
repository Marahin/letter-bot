package booking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/reservation"
)

func Test_canOverbook_trueWhenHasPermissionsAndAttemptsToOverbook(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
	hasPermissions := true
	conflictingReservations := []*reservation.Reservation{}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.True(output)
}

func Test_canOverbook_falseWhenHasPermissionsAndDoesntAttemptToOverbook(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := false
	hasPermissions := false
	conflictingReservations := []*reservation.Reservation{}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.False(output)
}

func Test_canOverbook_falseWhenReservationStartedLessThan10MinutesAgo(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-5 * time.Minute), // Started 5 mins ago
			EndAt:   now.Add(55 * time.Minute),
		},
	}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.False(output)
}

func Test_canOverbook_trueWhenReservationStartedMoreThan10MinutesAgo(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-11 * time.Minute), // Started 11 mins ago
			EndAt:   now.Add(49 * time.Minute),
		},
	}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.True(output)
}

func Test_canOverbook_falseWhenMultipleConflictingReservations(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
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
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.False(output)
}

func Test_canOverbook_falseWhenReservationHasNotStartedYet(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(5 * time.Minute), // Starts in future
			EndAt:   now.Add(65 * time.Minute),
		},
	}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.False(output)
}

func Test_canOverbook_falseWhenReservationAlreadyEnded(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
	hasPermissions := false
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-2 * time.Hour),
			EndAt:   now.Add(-1 * time.Hour), // Ended in past
		},
	}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.False(output)
}

func Test_canOverbook_trueWhenPrivilegedUserOverbooksEarly(t *testing.T) {
	// given
	assert := assert.New(t)
	attemptsToOverbook := true
	hasPermissions := true
	now := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			StartAt: now.Add(-5 * time.Minute), // Started 5 mins ago
			EndAt:   now.Add(55 * time.Minute),
		},
	}

	// when
	output := canOverbook(attemptsToOverbook, hasPermissions, conflictingReservations)

	// assert
	assert.True(output)
}

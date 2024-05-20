package booking

import (
	"testing"

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

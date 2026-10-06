package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTotals_WithoutExperienceDataHasNoFigures(t *testing.T) {
	// given
	tot := Totals{Reservations: 2, Seconds: 7200}

	// then
	assert.False(t, tot.HasExp())
	assert.Nil(t, tot.ExpTotal())
	assert.Nil(t, tot.ExpPerHour())
	assert.InDelta(t, 2.0, tot.Hours(), 1e-9)
}

func TestTotals_ExpPerHourUsesHoursWithData(t *testing.T) {
	// given
	tot := Totals{Reservations: 3, Seconds: 3 * 3600, ExpReservations: 1, ExpSeconds: 1800, Exp: 500_000}

	// when
	exp, perHour := tot.ExpTotal(), tot.ExpPerHour()

	// then
	require.NotNil(t, exp)
	require.NotNil(t, perHour)
	assert.Equal(t, int64(500_000), *exp)
	assert.InDelta(t, 1_000_000.0, *perHour, 1e-6)
}

func TestTotals_ZeroGainWithDataIsAFigure(t *testing.T) {
	// given
	tot := Totals{Reservations: 1, Seconds: 3600, ExpReservations: 1, ExpSeconds: 3600}

	// then
	require.NotNil(t, tot.ExpTotal())
	assert.Zero(t, *tot.ExpTotal())
	assert.Nil(t, Totals{ExpReservations: 1}.ExpPerHour())
}

func TestTotals_Add(t *testing.T) {
	// when
	got := Totals{1, 2, 3, 4, 5}.Add(Totals{10, 20, 30, 40, 50})

	// then
	assert.Equal(t, Totals{11, 22, 33, 44, 55}, got)
}

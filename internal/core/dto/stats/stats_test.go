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
	got := Totals{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}.Add(Totals{10, 20, 30, 40, 50, 60, 70, 80, 90, 100})

	// then
	assert.Equal(t, Totals{11, 22, 33, 44, 55, 66, 77, 88, 99, 110}, got)
}

func TestTotals_WithoutLootDataHasNoLootFigures(t *testing.T) {
	// given
	tot := Totals{Reservations: 2, Seconds: 7200, ExpReservations: 1, ExpSeconds: 3600, Exp: 10}

	// then
	assert.False(t, tot.HasLoot())
	assert.Nil(t, tot.ProfitTotal())
	assert.Nil(t, tot.ProfitPerHour())
	assert.Nil(t, tot.LootTotal())
	assert.Nil(t, tot.SuppliesTotal())
}

func TestTotals_ProfitPerHourUsesSessionTime(t *testing.T) {
	// given
	tot := Totals{Reservations: 3, Seconds: 3 * 3600, LootReservations: 2, LootSeconds: 1800, Profit: -300_000, Loot: 900_000, Supplies: 1_200_000}

	// when
	profit, perHour := tot.ProfitTotal(), tot.ProfitPerHour()

	// then
	require.NotNil(t, profit)
	require.NotNil(t, perHour)
	assert.Equal(t, int64(-300_000), *profit)
	assert.InDelta(t, -600_000.0, *perHour, 1e-6)
	assert.Equal(t, int64(900_000), *tot.LootTotal())
	assert.Equal(t, int64(1_200_000), *tot.SuppliesTotal())
	assert.InDelta(t, 0.5, tot.LootHours(), 1e-9)
}

func TestTotals_ZeroProfitWithDataIsAFigure(t *testing.T) {
	// given
	tot := Totals{LootReservations: 1, LootSeconds: 3600}

	// then
	require.NotNil(t, tot.ProfitTotal())
	assert.Zero(t, *tot.ProfitTotal())
	assert.Nil(t, Totals{LootReservations: 1}.ProfitPerHour())
}

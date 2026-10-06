package prommetrics

import (
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromMetrics_CountersAndGauges(t *testing.T) {
	// Use a fresh registry to avoid interference across tests
	reg := prom.NewRegistry()

	m := &PromMetrics{
		slashCommands:        prom.NewCounterVec(prom.CounterOpts{Name: "slash_command_invocations_total"}, []string{"guild_id", "guild_name", "command"}),
		overbookInvocations:  prom.NewCounterVec(prom.CounterOpts{Name: "overbook_invocations_total"}, []string{"guild_id", "guild_name"}),
		commandErrors:        prom.NewCounterVec(prom.CounterOpts{Name: "command_errors_total"}, []string{"guild_id", "guild_name", "command"}),
		upcomingReservations: prom.NewGaugeVec(prom.GaugeOpts{Name: "upcoming_count"}, []string{"guild_id", "guild_name"}),
	}
	reg.MustRegister(m.slashCommands, m.overbookInvocations, m.commandErrors, m.upcomingReservations)

	// when: increment various metrics
	m.IncSlashCommand("123", "Guild 123", "book")
	m.IncSlashCommand("123", "Guild 123", "book")
	m.IncSlashCommand("123", "Guild 123", "unbook")
	m.IncOverbook("123", "Guild 123")
	m.IncCommandError("123", "Guild 123", "book")
	m.SetUpcomingReservations("123", "Guild 123", 5)

	// then: validate values using the registry
	if got := testutil.ToFloat64(m.slashCommands.WithLabelValues("123", "Guild 123", "book")); got != 2 {
		t.Fatalf("slash command counter for book = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.slashCommands.WithLabelValues("123", "Guild 123", "unbook")); got != 1 {
		t.Fatalf("slash command counter for unbook = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.overbookInvocations.WithLabelValues("123", "Guild 123")); got != 1 {
		t.Fatalf("overbook counter = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.commandErrors.WithLabelValues("123", "Guild 123", "book")); got != 1 {
		t.Fatalf("command error counter = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.upcomingReservations.WithLabelValues("123", "Guild 123")); got != 5 {
		t.Fatalf("upcoming reservations gauge = %v, want 5", got)
	}
}

func TestNew_RegistersOnTheGivenRegistry(t *testing.T) {
	// given
	reg := NewRegistry()

	// when
	m, err := New(reg)
	m.IncTicks()
	families, gatherErr := reg.Gather()

	// then
	require.NoError(t, err)
	require.NoError(t, gatherErr)
	names := make([]string, 0, len(families))
	for _, f := range families {
		names = append(names, f.GetName())
	}
	assert.Contains(t, names, "letter_bot_discord_tick_total")
	assert.Contains(t, names, "go_goroutines")
}

func TestNew_FailsOnASecondRegistration(t *testing.T) {
	// given
	reg := prom.NewRegistry()
	_, err := New(reg)
	require.NoError(t, err)

	// when
	_, err = New(reg)

	// then
	assert.Error(t, err)
}

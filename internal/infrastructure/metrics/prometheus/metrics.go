package prommetrics

import (
	"strconv"

	prom "github.com/prometheus/client_golang/prometheus"
)

// PromMetrics implements ports.MetricsPort via Prometheus client.
// It registers counters/gauges with a guild label to enable per-guild and global totals.
type PromMetrics struct {
	slashCommands        *prom.CounterVec
	overbookInvocations  *prom.CounterVec
	commandErrors        *prom.CounterVec
	upcomingReservations *prom.GaugeVec
}

// New creates and registers Prometheus metrics using the default registry.
func New() *PromMetrics {
	m := &PromMetrics{
		slashCommands: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "slash_command_invocations_total",
			Help:      "Total number of slash command invocations.",
		}, []string{"guild_id", "command"}),
		overbookInvocations: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "booking",
			Name:      "overbook_invocations_total",
			Help:      "Total number of invocations with overbook flag set.",
		}, []string{"guild_id"}),
		commandErrors: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "command_errors_total",
			Help:      "Total number of errors while handling commands.",
		}, []string{"guild_id", "command"}),
		upcomingReservations: prom.NewGaugeVec(prom.GaugeOpts{
			Namespace: "letter_bot",
			Subsystem: "reservations",
			Name:      "upcoming_count",
			Help:      "Number of upcoming reservations per guild.",
		}, []string{"guild_id"}),
	}

	prom.MustRegister(m.slashCommands, m.overbookInvocations, m.commandErrors, m.upcomingReservations)

	return m
}

// IncSlashCommand increments counter of slash command invocations.
func (m *PromMetrics) IncSlashCommand(guildID, command string) {
	m.slashCommands.WithLabelValues(guildID, command).Inc()
}

// IncOverbook increments counter for overbook flag usage in book command.
func (m *PromMetrics) IncOverbook(guildID string) {
	m.overbookInvocations.WithLabelValues(guildID).Inc()
}

// IncCommandError increments counter for command invocation errors.
func (m *PromMetrics) IncCommandError(guildID, command string) {
	m.commandErrors.WithLabelValues(guildID, command).Inc()
}

// SetUpcomingReservations sets gauge of upcoming reservations for a guild.
func (m *PromMetrics) SetUpcomingReservations(guildID string, count int) {
	m.upcomingReservations.WithLabelValues(guildID).Set(float64(count))
}

// helper to quiet import usage in some contexts
var _ = strconv.Itoa

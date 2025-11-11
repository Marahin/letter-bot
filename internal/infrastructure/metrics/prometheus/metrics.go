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
	ticks                *prom.CounterVec
	messagesSent         *prom.CounterVec
	messagesDeleted      *prom.CounterVec
}

// New creates and registers Prometheus metrics using the default registry.
func New() *PromMetrics {
	m := &PromMetrics{
		slashCommands: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "slash_command_invocations_total",
			Help:      "Total number of slash command invocations.",
		}, []string{"guild_id", "guild_name", "command"}),
		overbookInvocations: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "booking",
			Name:      "overbook_invocations_total",
			Help:      "Total number of invocations with overbook flag set.",
		}, []string{"guild_id", "guild_name"}),
		commandErrors: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "command_errors_total",
			Help:      "Total number of errors while handling commands.",
		}, []string{"guild_id", "guild_name", "command"}),
		upcomingReservations: prom.NewGaugeVec(prom.GaugeOpts{
			Namespace: "letter_bot",
			Subsystem: "reservations",
			Name:      "upcoming_count",
			Help:      "Number of upcoming reservations per guild.",
		}, []string{"guild_id", "guild_name"}),
		ticks: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "tick_total",
			Help:      "A counter of bot ticks.",
		}, []string{}),
		messagesSent: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "messages_sent_total",
			Help:      "Total number of messages sent by the bot.",
		}, []string{"channel_id", "channel_name"}),
		messagesDeleted: prom.NewCounterVec(prom.CounterOpts{
			Namespace: "letter_bot",
			Subsystem: "discord",
			Name:      "messages_deleted_total",
			Help:      "Total number of messages deleted by the bot.",
		}, []string{"channel_id", "channel_name"}),
	}

	prom.MustRegister(m.slashCommands, m.overbookInvocations, m.commandErrors, m.upcomingReservations, m.ticks, m.messagesSent, m.messagesDeleted)

	return m
}

// IncSlashCommand increments counter of slash command invocations.
func (m *PromMetrics) IncSlashCommand(guildID, guildName, command string) {
	m.slashCommands.WithLabelValues(guildID, guildName, command).Inc()
}

// IncOverbook increments counter for overbook flag usage in book command.
func (m *PromMetrics) IncOverbook(guildID, guildName string) {
	m.overbookInvocations.WithLabelValues(guildID, guildName).Inc()
}

// IncCommandError increments counter for command invocation errors.
func (m *PromMetrics) IncCommandError(guildID, guildName, command string) {
	m.commandErrors.WithLabelValues(guildID, guildName, command).Inc()
}

// SetUpcomingReservations sets gauge of upcoming reservations for a guild.
func (m *PromMetrics) SetUpcomingReservations(guildID, guildName string, count int) {
	m.upcomingReservations.WithLabelValues(guildID, guildName).Set(float64(count))
}

// IncTicks increments counter of ticks.
func (m *PromMetrics) IncTicks() {
	m.ticks.WithLabelValues().Inc()
}

// AddMessagesDeleted adds to counter of messages deleted by the bot.
func (m *PromMetrics) AddMessagesSent(channelID, channelName string, count int) {
	m.messagesSent.WithLabelValues(channelID, channelName).Add(float64(count))
}

// IncMessagesSent increments counter of messages sent by the bot.
func (m *PromMetrics) IncMessagesSent(channelID, channelName string) {
	m.messagesSent.WithLabelValues(channelID, channelName).Inc()
}

// AddMessagesDeleted adds to counter of messages deleted by the bot.
func (m *PromMetrics) AddMessagesDeleted(channelID, channelName string, count int) {
	m.messagesDeleted.WithLabelValues(channelID, channelName).Add(float64(count))
}

// helper to quiet import usage in some contexts
var _ = strconv.Itoa

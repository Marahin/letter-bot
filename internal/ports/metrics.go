package ports

// MetricsPort defines metrics operations that the core/infrastructure can use
// without depending on a specific metrics backend implementation.
// Implementations should be safe for concurrent use.
type MetricsPort interface {
	// IncSlashCommand increments counter of slash command invocations.
	// Labels: guildID, command
	IncSlashCommand(guildID, command string)

	// IncOverbook increments counter for overbook flag usage in book command.
	// Labels: guildID
	IncOverbook(guildID string)

	// IncCommandError increments counter for command invocation errors.
	// Labels: guildID, command
	IncCommandError(guildID, command string)

	// SetUpcomingReservations sets gauge of upcoming reservations for a guild.
	// Labels: guildID
	SetUpcomingReservations(guildID string, count int)
}

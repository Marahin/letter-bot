package ports

// MetricsPort defines metrics operations that the core/infrastructure can use
// without depending on a specific metrics backend implementation.
// Implementations should be safe for concurrent use.
type MetricsPort interface {
	// IncSlashCommand increments counter of slash command invocations.
	// Labels: guild_id, guild_name, command
	IncSlashCommand(guildID, guildName, command string)

	// IncOverbook increments counter for overbook flag usage in book command.
	// Labels: guild_id, guild_name
	IncOverbook(guildID, guildName string)

	// IncCommandError increments counter for command invocation errors.
	// Labels: guild_id, guild_name, command
	IncCommandError(guildID, guildName, command string)

	// SetUpcomingReservations sets gauge of upcoming reservations for a guild.
	// Labels: guild_id, guild_name
	SetUpcomingReservations(guildID, guildName string, count int)

	// IncTicks increments counter of ticks.
	IncTicks()

	// AddMessagesSent increments counter of messages sent by the bot.
	AddMessagesSent(channelID, channelName string, count int)

	// IncMessagesSent increments counter of messages sent by the bot.
	IncMessagesSent(channelID, channelName string)

	// AddMessagesDeleted increments counter of messages deleted by the bot.
	AddMessagesDeleted(channelID, channelName string, count int)

	// IncUpcomingReservationNotificationSent increments counter of upcoming reservation notifications sent.
	// Labels: guild_id, guild_name
	IncUpcomingReservationNotificationSent(guildID, guildName string)
}

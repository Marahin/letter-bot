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
}

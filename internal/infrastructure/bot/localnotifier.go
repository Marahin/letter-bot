package bot

import (
	"context"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/ports"
)

// LocalNotifier implements ports.BotNotifier inside the bot process, for the
// core services that the bot shares with the web.
type LocalNotifier struct {
	guilds GuildActions
	comm   ports.CommunicationService
}

func NewLocalNotifier(guilds GuildActions, comm ports.CommunicationService) *LocalNotifier {
	return &LocalNotifier{guilds: guilds, comm: comm}
}

func (n *LocalNotifier) SummaryChanged(_ context.Context, guildID string) error {
	n.guilds.RefreshGuildLetter(guildID)
	return nil
}

func (n *LocalNotifier) ResyncRequested(ctx context.Context, guildID string) error {
	return n.guilds.SyncGuild(ctx, guildID)
}

func (n *LocalNotifier) ConfigChanged(ctx context.Context, guildID string) error {
	n.guilds.ApplyGuildConfig(ctx, guildID)
	return nil
}

func (n *LocalNotifier) Overbooked(_ context.Context, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) error {
	n.comm.NotifyOverbookedMember(request, res)
	return nil
}

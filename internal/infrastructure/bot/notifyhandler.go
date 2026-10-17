package bot

import (
	"context"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/ports"
)

// GuildActions is what NotifyHandler asks the bot to do. *Bot implements it.
type GuildActions interface {
	RefreshGuildLetter(guildID string)
	// RefreshAllGuildLetters refreshes the summary of every premium guild the bot is in.
	RefreshAllGuildLetters(ctx context.Context)
	SyncGuild(ctx context.Context, guildID string) error
	ApplyGuildConfig(ctx context.Context, guildID string)
	BookingMade(ctx context.Context, event book.Booked)
}

// NotifyHandler implements ports.NotifyHandler for the bot.
type NotifyHandler struct {
	guilds  GuildActions
	commSrv ports.CommunicationService
	log     *zap.SugaredLogger
}

func NewNotifyHandler(guilds GuildActions, commSrv ports.CommunicationService) *NotifyHandler {
	return &NotifyHandler{guilds: guilds, commSrv: commSrv, log: zap.NewNop().Sugar()}
}

func (h *NotifyHandler) WithLogger(log *zap.SugaredLogger) *NotifyHandler {
	h.log = log.With("layer", "infrastructure", "name", "notifyHandler")
	return h
}

func (h *NotifyHandler) OnSummaryRefresh(_ context.Context, guildID string) {
	h.guilds.RefreshGuildLetter(guildID)
}

func (h *NotifyHandler) OnGuildResync(ctx context.Context, guildID string) {
	if err := h.guilds.SyncGuild(ctx, guildID); err != nil {
		h.log.With("guild.ID", guildID).Errorf("could not sync guild: %s", err)
	}
}

func (h *NotifyHandler) OnGuildConfig(ctx context.Context, guildID string) {
	h.guilds.ApplyGuildConfig(ctx, guildID)
}

func (h *NotifyHandler) OnOverbooked(_ context.Context, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) {
	h.commSrv.NotifyOverbookedMember(request, res)
}

// OnBooked announces in the background, so a slow Discord does not hold up the
// signals behind it.
func (h *NotifyHandler) OnBooked(ctx context.Context, event book.Booked) {
	go h.guilds.BookingMade(context.WithoutCancel(ctx), event)
}

// OnListenerConnected refreshes every summary, because a signal sent while the
// listener was disconnected is lost.
func (h *NotifyHandler) OnListenerConnected(ctx context.Context) {
	h.guilds.RefreshAllGuildLetters(ctx)
}

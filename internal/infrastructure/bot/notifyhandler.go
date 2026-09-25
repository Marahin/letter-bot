package bot

import (
	"context"

	"go.uber.org/zap"

	notify "spot-assistant/internal/infrastructure/notify/postgresql"
	"spot-assistant/internal/ports"
)

// GuildActions is what NotifyHandler asks the bot to do. *Bot implements it.
type GuildActions interface {
	RefreshGuildLetter(guildID string)
	SyncGuild(ctx context.Context, guildID string) error
	ApplyGuildConfig(ctx context.Context, guildID string)
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

func (h *NotifyHandler) OnOverbooked(_ context.Context, payload []byte) {
	p, err := notify.DecodeOverbookedPayload(payload)
	if err != nil {
		h.log.Warnf("ignoring malformed overbooked payload: %s", err)
		return
	}
	h.commSrv.NotifyOverbookedMember(p.BookRequest(), p.Reservation())
}

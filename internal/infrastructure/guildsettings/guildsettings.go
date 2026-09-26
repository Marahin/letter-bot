// Package guildsettings implements ports.GuildSettingsService, behind the admin
// Settings and Channels pages. It lists the Discord options the bot synced
// (channels, roles), validates each choice against them, stores it and signals
// the bot. As in scxmanager, the service lives in the infra layer; its refusals
// are ports errors, so the HTTP handlers depend on ports only.
package guildsettings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/worlds"
	"spot-assistant/internal/ports"
)

// ResyncCooldown is the minimum time between two "Refresh server data" requests of one guild.
const ResyncCooldown = 5 * time.Minute

var postableChannelTypes = []discord.ChannelType{discord.ChannelTypeGuildText, discord.ChannelTypeGuildNews}

// Service implements ports.GuildSettingsService.
type Service struct {
	configs  ports.GuildConfigRepository
	channels ports.GuildChannelRepository
	roles    ports.GuildRoleRepository
	worlds   ports.WorldNameRepository
	notifier ports.BotNotifier
	log      *zap.SugaredLogger
	now      func() time.Time

	mu          sync.Mutex
	lastRefresh map[string]time.Time
}

func New(configs ports.GuildConfigRepository, channels ports.GuildChannelRepository, roles ports.GuildRoleRepository,
	worldNames ports.WorldNameRepository, notifier ports.BotNotifier, log *zap.SugaredLogger) *Service {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Service{
		configs:     configs,
		channels:    channels,
		roles:       roles,
		worlds:      worldNames,
		notifier:    notifier,
		log:         log,
		now:         time.Now,
		lastRefresh: map[string]time.Time{},
	}
}

// Channels returns the synced channels a bot can post to: text and announcement channels.
func (s *Service) Channels(ctx context.Context, guildID string) ([]*discord.Channel, error) {
	channels, err := s.channels.ListByTypes(ctx, guildID, postableChannelTypes)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return channels, nil
}

func (s *Service) Roles(ctx context.Context, guildID string) ([]*role.Role, error) {
	roles, err := s.roles.List(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return roles, nil
}

func (s *Service) World(ctx context.Context, guildID string) (string, error) {
	w, err := s.worlds.SelectGuildWorld(ctx, guildID)
	if errors.Is(err, ports.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("select guild world: %w", err)
	}
	return w.WorldName, nil
}

// SetChannels validates both ids against the synced channels, stores them, and
// tells the bot: the command gate reads the config, and the summary moves.
func (s *Service) SetChannels(ctx context.Context, guildID, commandChannelID, summaryChannelID string) error {
	channels, err := s.Channels(ctx, guildID)
	if err != nil {
		return err
	}
	for _, id := range []string{commandChannelID, summaryChannelID} {
		if id != "" && !slices.ContainsFunc(channels, func(c *discord.Channel) bool { return c.ID == id }) {
			return ports.ErrUnknownChannel
		}
	}
	if err := s.configs.SetChannels(ctx, guildID, commandChannelID, summaryChannelID); err != nil {
		return fmt.Errorf("set channels: %w", err)
	}
	s.signal(guildID, "config", s.notifier.ConfigChanged(ctx, guildID))
	s.signal(guildID, "summary", s.notifier.SummaryChanged(ctx, guildID))
	return nil
}

// SetRoleIDs validates the ids against the synced roles and stores the rank list.
// The bot and the web read the lists on each command and request, so no signal is needed.
func (s *Service) SetRoleIDs(ctx context.Context, guildID string, kind guildconfig.RoleKind, roleIDs []string) error {
	if !kind.Valid() {
		return ports.ErrUnknownRoleKind
	}
	roles, err := s.Roles(ctx, guildID)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(roleIDs))
	for _, id := range roleIDs {
		if !slices.ContainsFunc(roles, func(r *role.Role) bool { return r.ID == id }) {
			return ports.ErrUnknownRole
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if err := s.configs.SetRoleIDs(ctx, guildID, kind, ids); err != nil {
		return fmt.Errorf("set %s role ids: %w", kind, err)
	}
	return nil
}

// SetWorld stores the Tibia world (case-insensitive, saved in its canonical
// spelling) and tells the bot to reload it for the online list.
func (s *Service) SetWorld(ctx context.Context, guildID, world string) error {
	i := slices.IndexFunc(worlds.Worlds, func(w string) bool { return strings.EqualFold(w, strings.TrimSpace(world)) })
	if i < 0 {
		return ports.ErrUnknownWorld
	}
	if err := s.worlds.UpsertGuildWorld(ctx, guildID, worlds.Worlds[i]); err != nil {
		return fmt.Errorf("upsert guild world: %w", err)
	}
	s.signal(guildID, "config", s.notifier.ConfigChanged(ctx, guildID))
	return nil
}

// RequestResync sets the durable resync flag, which the bot checks on each tick,
// and sends a NOTIFY so the sync usually starts at once.
func (s *Service) RequestResync(ctx context.Context, guildID string) (time.Duration, error) {
	now := s.now()
	s.mu.Lock()
	if last, ok := s.lastRefresh[guildID]; ok {
		if elapsed := now.Sub(last); elapsed < ResyncCooldown {
			s.mu.Unlock()
			return ResyncCooldown - elapsed, nil
		}
	}
	s.lastRefresh[guildID] = now
	s.mu.Unlock()

	if err := s.configs.RequestResync(ctx, guildID); err != nil {
		// A failed request must not start the cooldown.
		s.mu.Lock()
		delete(s.lastRefresh, guildID)
		s.mu.Unlock()
		return 0, fmt.Errorf("request resync: %w", err)
	}
	s.signal(guildID, "resync", s.notifier.ResyncRequested(ctx, guildID))
	return 0, nil
}

// signal logs a failed best-effort NOTIFY. The bot's tick recovers a missed one.
func (s *Service) signal(guildID, what string, err error) {
	if err != nil {
		s.log.Warnw("notify bot", "signal", what, "guild_id", guildID, "error", err)
	}
}

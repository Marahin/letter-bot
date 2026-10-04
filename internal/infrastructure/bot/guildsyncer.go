package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/ports"
)

const guildSyncDelay = 5 * time.Second

// DiscordGuildReader is the part of the Discord REST client that GuildSyncer reads from.
type DiscordGuildReader interface {
	GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error)
	GuildRoles(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Role, error)
}

// GuildSyncer copies the channels and roles of a guild into the database, so the web can show them.
type GuildSyncer struct {
	reader    DiscordGuildReader
	channels  ports.GuildChannelRepository
	roles     ports.GuildRoleRepository
	configs   ports.GuildConfigRepository
	scheduler *debouncer
	log       *zap.SugaredLogger
}

func NewGuildSyncer(reader DiscordGuildReader, channels ports.GuildChannelRepository, roles ports.GuildRoleRepository, configs ports.GuildConfigRepository) *GuildSyncer {
	return &GuildSyncer{
		reader:    reader,
		channels:  channels,
		roles:     roles,
		configs:   configs,
		scheduler: newDebouncer(guildSyncDelay),
		log:       zap.NewNop().Sugar(),
	}
}

func (s *GuildSyncer) WithLogger(log *zap.SugaredLogger) *GuildSyncer {
	s.log = log.With("layer", "infrastructure", "name", "guildSyncer")
	return s
}

func (s *GuildSyncer) Sync(ctx context.Context, guildID string) error {
	startedAt := time.Now()
	channels, err := s.reader.GuildChannels(guildID)
	if err != nil {
		return fmt.Errorf("fetch channels: %w", err)
	}
	roles, err := s.reader.GuildRoles(guildID)
	if err != nil {
		return fmt.Errorf("fetch roles: %w", err)
	}

	if err := s.channels.Replace(ctx, guildID, syncableChannels(channels)); err != nil {
		return fmt.Errorf("store channels: %w", err)
	}
	if err := s.roles.Replace(ctx, guildID, syncableRoles(guildID, roles)); err != nil {
		return fmt.Errorf("store roles: %w", err)
	}

	return s.configs.MarkSynced(ctx, guildID, startedAt)
}

// Schedule syncs the guild once Discord events for it stop arriving for a few seconds.
func (s *GuildSyncer) Schedule(guildID string) {
	s.scheduler.Trigger(guildID, func() {
		if err := s.Sync(context.Background(), guildID); err != nil {
			s.log.With("guild.ID", guildID).Errorf("could not sync guild: %s", err)
		}
	})
}

func syncableChannels(input []*discordgo.Channel) []*discord.Channel {
	out := make([]*discord.Channel, 0, len(input))
	for _, ch := range input {
		switch ch.Type {
		case discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews, discordgo.ChannelTypeGuildCategory:
			out = append(out, MapChannel(ch))
		default:
		}
	}
	return out
}

func syncableRoles(guildID string, input []*discordgo.Role) []*role.Role {
	out := make([]*role.Role, 0, len(input))
	for _, r := range input {
		// The @everyone role has the id of the guild.
		if r.ID == guildID || r.Managed {
			continue
		}
		out = append(out, mapRole(r))
	}
	return out
}

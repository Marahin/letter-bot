package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"

	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/ports"
)

// guildConfig returns the stored configuration. A guild that is not stored gets an empty, non-premium one.
func (b *Bot) guildConfig(ctx context.Context, guildID string) (*guildconfig.Config, error) {
	cfg, err := b.guildConfigs.Get(ctx, guildID)
	if errors.Is(err, ports.ErrNotFound) {
		return &guildconfig.Config{GuildID: guildID}, nil
	}
	return cfg, err
}

func (b *Bot) guildConfigsByID(ctx context.Context, guildIDs []string) (map[string]*guildconfig.Config, error) {
	cfgs, err := b.guildConfigs.ListByIDs(ctx, guildIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*guildconfig.Config, len(cfgs))
	for _, cfg := range cfgs {
		byID[cfg.GuildID] = cfg
	}
	return byID, nil
}

func notPremiumMessage(webBaseURL string) string {
	msg := "Letter is not active on this server, because the server does not have premium."
	if webBaseURL != "" {
		msg += fmt.Sprintf(" See %s for more information.", webBaseURL)
	}
	return msg
}

// commandGateReply returns the ephemeral reply that stops a slash command, or "" when the command may run.
func commandGateReply(cfg *guildconfig.Config, command, channelID, webBaseURL string) string {
	if !cfg.IsPremium() {
		return notPremiumMessage(webBaseURL)
	}
	if cfg.CommandChannelID == "" || channelID == cfg.CommandChannelID {
		return ""
	}
	switch command {
	case "book", "unbook":
		return fmt.Sprintf("Please use this command in <#%s>.", cfg.CommandChannelID)
	}
	return ""
}

func memberSubject(g *guild.Guild, m *member.Member) permission.Subject {
	return permission.Subject{
		IsAdmin:    m.Permissions&discordgo.PermissionAdministrator != 0 || (g.OwnerID != "" && g.OwnerID == m.ID),
		RoleIDs:    m.Roles,
		GuildRoles: g.Roles,
	}
}

// legacyChannelsToCreate returns the names of the legacy channels that the guild needs and does not have.
// A legacy channel is needed only when the matching channel setting is empty.
func legacyChannelsToCreate(cfg *guildconfig.Config, existing []*discordgo.Channel) []string {
	have := map[string]bool{}
	for _, ch := range existing {
		have[ch.Name] = true
	}

	var missing []string
	if cfg.SummaryChannelID == "" && !have[discord.SummaryChannel] {
		missing = append(missing, discord.SummaryChannel)
	}
	if cfg.CommandChannelID == "" && !have[discord.CommandChannel] {
		missing = append(missing, discord.CommandChannel)
	}
	return missing
}

// summaryChannel returns the configured summary channel, or the legacy #letter-summary channel when none is set.
func (b *Bot) summaryChannel(g *guild.Guild, cfg *guildconfig.Config) (*discord.Channel, error) {
	if cfg.SummaryChannelID != "" {
		return b.FindChannelByID(g, cfg.SummaryChannelID)
	}
	return b.FindChannelByName(g, discord.SummaryChannel)
}

// setupPremiumGuild creates the legacy channels and the Postman role where the settings leave them empty.
func (b *Bot) setupPremiumGuild(g *guild.Guild, cfg *guildconfig.Config) error {
	if err := b.EnsureChannel(g, cfg); err != nil {
		return fmt.Errorf("could not ensure channels: %w", err)
	}
	if len(cfg.OverbookRoleIDs) == 0 {
		if err := b.EnsureRoles(g); err != nil {
			return fmt.Errorf("could not ensure roles: %w", err)
		}
	}
	return nil
}

// RefreshGuildLetter updates the summary of the guild once the refresh requests for it stop for a few seconds.
func (b *Bot) RefreshGuildLetter(guildID string) {
	b.summaryRefresh.Trigger(guildID, func() {
		g, err := b.guildByID(guildID)
		if err != nil {
			b.log.With("guild.ID", guildID).Errorf("could not refresh guild letter: %s", err)
			return
		}
		b.TryUpdateGuildLetter(g)
	})
}

// SyncGuild copies the channels and roles of the guild into the database.
func (b *Bot) SyncGuild(ctx context.Context, guildID string) error {
	return b.syncer.Sync(ctx, guildID)
}

// ApplyGuildConfig applies a configuration that the web changed.
func (b *Bot) ApplyGuildConfig(ctx context.Context, guildID string) {
	log := b.log.With("guild.ID", guildID)
	if err := b.onlineCheckService.ConfigureWorldNameForGuild(guildID); err != nil {
		log.Errorf("ConfigureWorldNameForGuild failed: %v", err)
	}

	cfg, err := b.guildConfig(ctx, guildID)
	if err != nil {
		log.Errorf("could not load guild config: %s", err)
		return
	}
	if !cfg.IsPremium() {
		return
	}

	g, err := b.guildByID(guildID)
	if err != nil {
		log.Errorf("could not fetch guild: %s", err)
		return
	}
	if err := b.setupPremiumGuild(g, cfg); err != nil {
		log.Error(err)
	}
	go b.onlineCheckService.TryRefresh(guildID)
	b.RefreshGuildLetter(guildID)
}

func (b *Bot) guildByID(guildID string) (*guild.Guild, error) {
	gID, err := stringsHelper.StrToInt64(guildID)
	if err != nil {
		return nil, err
	}
	return b.GetGuild(gID)
}

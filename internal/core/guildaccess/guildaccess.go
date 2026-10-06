// Package guildaccess decides which stored guilds a web user may open, and what
// they may do there. It reads the user's Discord guild list and member roles
// through ports.OAuthPort and applies the rank rules of internal/core/permission.
package guildaccess

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/ports"
)

// Service implements ports.GuildAccessService.
type Service struct {
	oauth      ports.OAuthPort
	configs    ports.GuildConfigRepository
	roles      ports.GuildRoleRepository
	siteAdmins map[string]bool
	log        *zap.SugaredLogger
}

func New(oauth ports.OAuthPort, configs ports.GuildConfigRepository, roles ports.GuildRoleRepository, siteAdminIDs []string, log *zap.SugaredLogger) *Service {
	admins := make(map[string]bool, len(siteAdminIDs))
	for _, id := range siteAdminIDs {
		if id = strings.TrimSpace(id); id != "" {
			admins[id] = true
		}
	}
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Service{oauth: oauth, configs: configs, roles: roles, siteAdmins: admins, log: log}
}

func (s *Service) IsSiteAdmin(userID string) bool {
	return userID != "" && s.siteAdmins[userID]
}

// AccessibleGuilds returns the guilds the user may view, sorted by name. For a
// non-admin these are the bot-present ones, and only View is exact in the list:
// the other capabilities need the member roles (use Access for one guild). A
// site admin gets every stored guild with full capabilities, bot-present first.
func (s *Service) AccessibleGuilds(ctx context.Context, userID string) ([]access.GuildAccess, error) {
	if s.IsSiteAdmin(userID) {
		return s.allGuilds(ctx)
	}
	userGuilds, err := s.oauth.UserGuilds(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch user guilds: %w", err)
	}
	if len(userGuilds) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(userGuilds))
	byID := make(map[string]access.UserGuild, len(userGuilds))
	for _, g := range userGuilds {
		ids = append(ids, g.ID)
		byID[g.ID] = g
	}
	configs, err := s.configs.ListPresentByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list guild configs: %w", err)
	}

	var out []access.GuildAccess
	renamed := false
	for _, cfg := range configs {
		a, err := s.resolve(ctx, userID, byID[cfg.GuildID], *cfg, false)
		if err != nil {
			// One guild's member lookup failing must not hide the others.
			s.log.Warnw("resolve guild access", "guild_id", cfg.GuildID, "error", err)
			continue
		}
		if a.Caps.View {
			renamed = renamed || cfg.Name == ""
			out = append(out, a)
		}
	}
	// The SQL order holds unless a guild took its name from Discord.
	if renamed {
		sortByName(out)
	}
	return out, nil
}

// Access resolves one guild with exact capabilities. It returns ports.ErrNotFound
// when the guild is not stored, the bot is absent, the user is not a member, or
// the user may not view it.
func (s *Service) Access(ctx context.Context, userID, guildID string) (*access.GuildAccess, error) {
	cfg, err := s.configs.Get(ctx, guildID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("load guild config: %w", err)
	}
	if s.IsSiteAdmin(userID) {
		return &access.GuildAccess{Config: *cfg, Caps: fullCapabilities}, nil
	}
	if !cfg.BotPresent {
		return nil, ports.ErrNotFound
	}
	userGuilds, err := s.oauth.UserGuilds(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch user guilds: %w", err)
	}
	for _, g := range userGuilds {
		if g.ID != guildID {
			continue
		}
		a, err := s.resolve(ctx, userID, g, *cfg, true)
		if err != nil {
			return nil, err
		}
		if !a.Caps.View {
			return nil, ports.ErrNotFound
		}
		return &a, nil
	}
	return nil, ports.ErrNotFound
}

func (s *Service) Public(ctx context.Context, guildID string) (*access.GuildAccess, error) {
	cfg, err := s.configs.Get(ctx, guildID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("load guild config: %w", err)
	}
	if !cfg.BotPresent {
		return nil, ports.ErrNotFound
	}
	return &access.GuildAccess{Config: *cfg}, nil
}

func (s *Service) Member(ctx context.Context, userID, guildID string) (*access.GuildMember, error) {
	return s.oauth.UserGuildMember(ctx, userID, guildID)
}

var fullCapabilities = permission.Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true}

func (s *Service) allGuilds(ctx context.Context) ([]access.GuildAccess, error) {
	configs, err := s.configs.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list guild configs: %w", err)
	}
	out := make([]access.GuildAccess, 0, len(configs))
	for _, c := range configs {
		out = append(out, access.GuildAccess{Config: *c, Caps: fullCapabilities})
	}
	return out, nil
}

// resolve computes the capabilities. The member roles are fetched only for a
// non-admin, and only when they can change the answer: with exact=false only
// View matters, and View is already granted when no reserve rank is set.
func (s *Service) resolve(ctx context.Context, userID string, g access.UserGuild, cfg guildconfig.Config, exact bool) (access.GuildAccess, error) {
	if cfg.Name == "" {
		cfg.Name = g.Name
	}
	if cfg.Icon == "" {
		cfg.Icon = g.Icon
	}
	subject := permission.Subject{IsAdmin: g.Admin}
	if !g.Admin && (exact || len(cfg.ReserveRoleIDs) > 0) {
		member, err := s.oauth.UserGuildMember(ctx, userID, g.ID)
		if errors.Is(err, ports.ErrNotFound) {
			return access.GuildAccess{Config: cfg}, nil
		}
		if err != nil {
			return access.GuildAccess{}, fmt.Errorf("fetch guild member: %w", err)
		}
		subject.RoleIDs = member.RoleIDs
		if exact && len(cfg.OverbookRoleIDs) == 0 {
			roles, err := s.roles.List(ctx, g.ID)
			if err != nil {
				return access.GuildAccess{}, fmt.Errorf("list guild roles: %w", err)
			}
			subject.GuildRoles = roles
		}
	}
	return access.GuildAccess{Config: cfg, Caps: permission.Resolve(cfg, subject)}, nil
}

func sortByName(list []access.GuildAccess) {
	sort.SliceStable(list, func(i, j int) bool {
		return strings.ToLower(list[i].Config.Name) < strings.ToLower(list[j].Config.Name)
	})
}

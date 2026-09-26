// Package premium is the site-admin switch for the premium status of a guild.
package premium

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/ports"
)

// ErrPremiumForever means the guild is premium forever, which only a migration changes.
var ErrPremiumForever = errors.New("guild is premium forever")

// Service implements ports.PremiumService.
type Service struct {
	configs  ports.GuildConfigRepository
	notifier ports.BotNotifier
	log      *zap.SugaredLogger
}

func New(configs ports.GuildConfigRepository, notifier ports.BotNotifier, log *zap.SugaredLogger) *Service {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Service{configs: configs, notifier: notifier, log: log}
}

// List returns every stored guild.
func (s *Service) List(ctx context.Context) ([]*guildconfig.Config, error) {
	return s.configs.ListAll(ctx)
}

// SetPremium changes the premium flag and tells the bot. It returns
// ports.ErrNotFound for an unknown guild and ErrPremiumForever when turning off a
// premium-forever guild.
func (s *Service) SetPremium(ctx context.Context, guildID string, premium bool) error {
	cfg, err := s.configs.Get(ctx, guildID)
	if err != nil {
		return err
	}
	if !premium && cfg.PremiumForever {
		return ErrPremiumForever
	}
	if cfg.Premium == premium {
		return nil
	}
	if err := s.configs.SetPremium(ctx, guildID, premium); err != nil {
		return fmt.Errorf("set premium: %w", err)
	}
	// Best-effort: the bot reads the flag again on its next tick.
	if err := s.notifier.ConfigChanged(ctx, guildID); err != nil {
		s.log.Warnw("notify bot of config change", "guild_id", guildID, "error", err)
	}
	return nil
}

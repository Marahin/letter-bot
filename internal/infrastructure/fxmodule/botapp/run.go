package botapp

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/eventhandler"
	prommetrics "spot-assistant/internal/infrastructure/metrics/prometheus"
	notifypg "spot-assistant/internal/infrastructure/notify/postgresql"
)

// wireBot closes the cycle between the bot and the event handler, which needs
// the bot through the communication adapter.
func wireBot(b *bot.Bot, h *eventhandler.Handler, m *prommetrics.PromMetrics) {
	b.WithEventHandler(h)
	b.WithMetrics(m)
	h.WithMetrics(m)
}

type runParams struct {
	fx.In

	Bot           *bot.Bot
	Listener      *notifypg.Listener
	NotifyHandler *bot.NotifyHandler
	Log           *zap.SugaredLogger
}

func run(lc fx.Lifecycle, p runParams) {
	listen := func(ctx context.Context) { p.Listener.Listen(ctx, p.NotifyHandler) }
	lc.Append(botHooks(p.Bot, listen, p.Log))
}

type lifecycleBot interface {
	Start() error
	Shutdown() error
}

// botHooks starts the NOTIFY listener before the gateway, as the web may signal
// at any time. The listener takes its own context: a hook's context ends with the hook.
func botHooks(b lifecycleBot, listen func(ctx context.Context), log *zap.SugaredLogger) fx.Hook {
	ctx, cancel := context.WithCancel(context.Background())
	return fx.Hook{
		OnStart: func(context.Context) error {
			go listen(ctx)
			if err := b.Start(); err != nil {
				cancel()
				return fmt.Errorf("bot start failed: %w", err)
			}
			return nil
		},
		OnStop: func(context.Context) error {
			log.Info("shutting down")
			if err := b.Shutdown(); err != nil {
				log.Errorw("bot shutdown", "error", err)
			}
			cancel()
			return nil
		},
	}
}

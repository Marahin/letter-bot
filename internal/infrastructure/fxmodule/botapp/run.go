package botapp

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/eventhandler"
	"spot-assistant/internal/infrastructure/fxmodule"
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
// at any time. The stop ends the listener first, so no NOTIFY reaches a stopping
// bot and the listener is done with the pool before the pool closes.
func botHooks(b lifecycleBot, listen func(ctx context.Context), log *zap.SugaredLogger) fx.Hook {
	listener := fxmodule.NewLoop(listen)
	return fx.Hook{
		OnStart: func(ctx context.Context) error {
			listener.Start()
			if err := b.Start(); err != nil {
				_ = listener.Stop(ctx)
				return fmt.Errorf("bot start failed: %w", err)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("shutting down")
			listenerErr := listener.Stop(ctx)
			if err := b.Shutdown(); err != nil {
				log.Errorw("bot shutdown", "error", err)
			}
			if listenerErr != nil {
				return fmt.Errorf("notify listener stop: %w", listenerErr)
			}
			return nil
		},
	}
}

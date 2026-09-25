package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

// defaultSlowDispatch is how long one notification may take before the listener
// says so. A handler runs on the goroutine that also reads the next notification,
// so a slow handler holds up every signal behind it.
const defaultSlowDispatch = 30 * time.Second

// Listener receives the NOTIFY signals the web sends to the bot. LISTEN needs a
// dedicated connection for its lifetime, so it opens its own connection instead
// of using the pool, and reconnects when the connection drops.
type Listener struct {
	dsn          string
	log          *zap.SugaredLogger
	retryBackoff time.Duration
	slowDispatch time.Duration
}

func NewListener(dsn string, log *zap.SugaredLogger) *Listener {
	return &Listener{
		dsn:          dsn,
		log:          log.With("layer", "infrastructure", "name", "notifyListener"),
		retryBackoff: 2 * time.Second,
		slowDispatch: defaultSlowDispatch,
	}
}

// Listen blocks until ctx is cancelled.
func (l *Listener) Listen(ctx context.Context, handler ports.NotifyHandler) {
	backoff := l.retryBackoff
	for ctx.Err() == nil {
		started := time.Now()
		err := l.listenOnce(ctx, handler)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			backoff = l.retryBackoff
		}
		l.log.Warnw("notify listener disconnected; retrying", "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, time.Minute)
	}
}

func (l *Listener) listenOnce(ctx context.Context, handler ports.NotifyHandler) error {
	conn, err := pgx.Connect(ctx, l.dsn)
	if err != nil {
		return fmt.Errorf("listener connect: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	for _, ch := range Channels {
		if _, err := conn.Exec(ctx, "LISTEN "+ch); err != nil {
			return fmt.Errorf("listen %s: %w", ch, err)
		}
	}
	l.log.Infow("listening for web signals", "channels", Channels)

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		l.dispatch(ctx, handler, n.Channel, n.Payload)
	}
}

func (l *Listener) dispatch(ctx context.Context, handler ports.NotifyHandler, channel, payload string) {
	l.log.Infow("notify received", "channel", channel, "payload", payload)
	defer func(started time.Time) {
		if took := time.Since(started); took > l.slowDispatch {
			l.log.Warnw("slow notify dispatch", "channel", channel, "duration_ms", took.Milliseconds())
		}
	}(time.Now())

	if payload == "" {
		l.log.Warnw("ignoring empty notify payload", "channel", channel)
		return
	}
	switch channel {
	case ChannelSummaryRefresh:
		handler.OnSummaryRefresh(ctx, payload)
	case ChannelGuildResync:
		handler.OnGuildResync(ctx, payload)
	case ChannelGuildConfig:
		handler.OnGuildConfig(ctx, payload)
	case ChannelOverbooked:
		handler.OnOverbooked(ctx, []byte(payload))
	default:
		l.log.Warnw("ignoring notify on unknown channel", "channel", channel)
	}
}

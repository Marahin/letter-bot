package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// NotifyExecutor runs the pg_notify statement. A pgxpool.Pool satisfies it.
type NotifyExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Notifier implements ports.BotNotifier with SELECT pg_notify. pg_notify takes the
// channel as a parameter, so nothing is interpolated into SQL.
type Notifier struct {
	db NotifyExecutor
}

func NewNotifier(db NotifyExecutor) *Notifier {
	return &Notifier{db: db}
}

func (n *Notifier) SummaryChanged(ctx context.Context, guildID string) error {
	return n.notify(ctx, ChannelSummaryRefresh, guildID)
}

func (n *Notifier) ResyncRequested(ctx context.Context, guildID string) error {
	return n.notify(ctx, ChannelGuildResync, guildID)
}

func (n *Notifier) ConfigChanged(ctx context.Context, guildID string) error {
	return n.notify(ctx, ChannelGuildConfig, guildID)
}

func (n *Notifier) Overbooked(ctx context.Context, payload []byte) error {
	return n.notify(ctx, ChannelOverbooked, string(payload))
}

func (n *Notifier) notify(ctx context.Context, channel, payload string) error {
	if _, err := n.db.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload); err != nil {
		return fmt.Errorf("notify %s: %w", channel, err)
	}
	return nil
}

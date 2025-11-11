package health

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	db      ports.DBPinger
	runtime ports.RuntimeStatus
	log     *zap.SugaredLogger
	timeout time.Duration
}

func NewAdapter(db ports.DBPinger, runtime ports.RuntimeStatus) *Adapter {
	return &Adapter{db: db, runtime: runtime, timeout: 2 * time.Second}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "infrastructure", "name", "health")
	return a
}

func (a *Adapter) Live() error {
	if a.runtime == nil || !a.runtime.IsRunning() {
		return fmt.Errorf("bot not running")
	}
	return nil
}

func (a *Adapter) Ready() error {
	// DB ping
	if a.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		if err := a.db.Ping(ctx); err != nil {
			return fmt.Errorf("db ping failed: %w", err)
		}
	}
	return a.Live()
}

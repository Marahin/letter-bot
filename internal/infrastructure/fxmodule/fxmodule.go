// Package fxmodule holds the fx modules the binary apps share.
package fxmodule

import (
	"fmt"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"spot-assistant/internal/common/version"
)

// Binary names the process in the first log line. Each app supplies it.
type Binary string

// Logger provides *zap.Logger and *zap.SugaredLogger, routes fx's own event log
// through them and writes the first log line.
var Logger = fx.Options(
	fx.Provide(newLogger, (*zap.Logger).Sugar),
	fx.WithLogger(newEventLogger),
	fx.Invoke(logStart),
)

func logStart(binary Binary, log *zap.SugaredLogger) {
	log.Infow("starting letter-"+string(binary), "version", version.Version, "tz", time.Now().Location().String())
}

func newLogger(lc fx.Lifecycle) (*zap.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	lc.Append(fx.StopHook(func() { _ = logger.Sync() }))
	return logger, nil
}

// newEventLogger keeps the PROVIDE and INVOKE lines at Debug, below the
// production logger's level.
func newEventLogger(logger *zap.Logger) fxevent.Logger {
	l := &fxevent.ZapLogger{Logger: logger}
	l.UseLogLevel(zapcore.DebugLevel)
	return l
}

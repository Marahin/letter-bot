package fxmodule

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
	"go.uber.org/zap"

	infrahttp "spot-assistant/internal/infrastructure/http"
	prommetrics "spot-assistant/internal/infrastructure/metrics/prometheus"
)

// MetricsAddr is the listen address of the internal metrics and health server.
// Each binary provides it from its own config.
type MetricsAddr string

// HealthChecks gate /livez and /readyz. A nil Live keeps /livez at 200; a nil
// Ready keeps /readyz at 503.
type HealthChecks struct {
	Live  infrahttp.CheckFunc
	Ready infrahttp.CheckFunc
}

// Registry provides the binary's own Prometheus registry.
var Registry = fx.Provide(prommetrics.NewRegistry)

// Metrics serves /metrics for the registry, /livez and /readyz on MetricsAddr
// for the app's lifetime. A busy address fails the start.
var Metrics = fx.Invoke(serveMetrics)

func serveMetrics(lc fx.Lifecycle, addr MetricsAddr, reg *prometheus.Registry, checks HealthChecks, log *zap.SugaredLogger) {
	srv := infrahttp.NewServerWithMetrics(string(addr), reg, log).WithHealth(checks.Live, checks.Ready)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return srv.Listen()
		},
		// A failed metrics shutdown must not turn a clean exit into exit code 1.
		OnStop: func(ctx context.Context) error {
			_ = srv.Shutdown(ctx)
			return nil
		},
	})
}

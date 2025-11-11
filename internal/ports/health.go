package ports

import "context"

// HealthPort exposes health checks for liveness and readiness.
type HealthPort interface {
	Live() error
	Ready() error
}

// RuntimeStatus exposes minimal runtime status used for liveness.
type RuntimeStatus interface {
	IsRunning() bool
}

// DBPinger abstracts a minimal database Ping method used for readiness.
type DBPinger interface {
	Ping(ctx context.Context) error
}

package fxmodule

import (
	"context"

	"go.uber.org/fx"
)

// Loop runs a long-running function on its own context, because a hook's
// context ends with the hook.
type Loop struct {
	run    func(ctx context.Context)
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewLoop returns a Loop that runs run when started.
func NewLoop(run func(ctx context.Context)) *Loop {
	ctx, cancel := context.WithCancel(context.Background())
	return &Loop{run: run, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

// Start runs the loop in a goroutine.
func (l *Loop) Start() {
	go func() {
		defer close(l.done)
		l.run(l.ctx)
	}()
}

// Stop cancels the loop and waits for it to return, at most until ctx ends.
func (l *Loop) Stop(ctx context.Context) error {
	l.cancel()
	select {
	case <-l.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// LoopHook runs run for the app's lifetime. The stop waits for run to return.
func LoopHook(run func(ctx context.Context)) fx.Hook {
	l := NewLoop(run)
	return fx.Hook{
		OnStart: func(context.Context) error {
			l.Start()
			return nil
		},
		OnStop: l.Stop,
	}
}

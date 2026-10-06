// Package scheduler runs a periodic job in at most one process at a time.
package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// JobLocker is a lock shared by every process that runs the job.
type JobLocker interface {
	// TryLock does not wait. When ok, the caller must call unlock.
	TryLock(ctx context.Context) (unlock func(), ok bool, err error)
}

type Scheduler struct {
	name     string
	job      func(ctx context.Context) error
	locker   JobLocker
	interval time.Duration
	log      *zap.SugaredLogger
	now      func() time.Time
}

// New schedules job every interval; name labels its log lines.
func New(name string, job func(ctx context.Context) error, locker JobLocker, interval time.Duration, log *zap.SugaredLogger) *Scheduler {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Scheduler{name: name, job: job, locker: locker, interval: interval, log: log.With("job", name), now: time.Now}
}

// Run runs the job now and then every interval, until ctx is done.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick runs the job once if no other process holds the lock. A run may take at most one interval.
func (s *Scheduler) Tick(ctx context.Context) {
	unlock, ok, err := s.locker.TryLock(ctx)
	if err != nil {
		s.log.Warnw("scheduled job: lock failed", "error", err)
		return
	}
	if !ok {
		s.log.Debug("scheduled job: another process holds the lock")
		return
	}
	defer unlock()

	runCtx, cancel := context.WithTimeout(ctx, s.interval)
	defer cancel()
	started := s.now()
	if err := s.job(runCtx); err != nil {
		s.log.Errorw("scheduled job failed", "error", err, "took", s.now().Sub(started))
		return
	}
	s.log.Debugw("scheduled job done", "took", s.now().Sub(started))
}

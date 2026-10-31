package bot

import (
	"context"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// messageOpGap is the least time between two summary writes to Discord.
	messageOpGap = time.Second
	// boundarySlack runs a pass just after a boundary, because the upcoming
	// reservations query keeps a reservation until its end has passed.
	boundarySlack   = time.Second
	maxSummaryDelay = 24 * time.Hour
	firstRetryDelay = 30 * time.Second
	maxRetryDelay   = 15 * time.Minute
)

// reconcileResult tells the worker when to look at a guild again.
type reconcileResult struct {
	// Next is the next start or end of a reservation in the summary. Zero means none.
	Next time.Time
	// Again asks for another pass after the delay, e.g. when a pass did only part of the work.
	Again time.Duration
	Err   error
}

// summaryWorker updates the summaries of one guild at a time, so the bot never
// sends Discord a burst. A guild is queued at most once; a trigger during its
// pass queues it again. Each guild has one timer for its next boundary or retry.
type summaryWorker struct {
	reconcile func(ctx context.Context, guildID string) reconcileResult
	debounce  *debouncer
	now       func() time.Time
	afterFunc func(d time.Duration, f func()) *time.Timer
	log       *zap.SugaredLogger

	mu      sync.Mutex
	queue   []string
	queued  map[string]bool
	timers  map[string]*time.Timer
	backoff map[string]time.Duration
	wake    chan struct{}
}

func newSummaryWorker(reconcile func(ctx context.Context, guildID string) reconcileResult, debounceDelay time.Duration) *summaryWorker {
	return &summaryWorker{
		reconcile: reconcile,
		debounce:  newDebouncer(debounceDelay),
		now:       time.Now,
		afterFunc: time.AfterFunc,
		log:       zap.NewNop().Sugar(),
		queued:    map[string]bool{},
		timers:    map[string]*time.Timer{},
		backoff:   map[string]time.Duration{},
		wake:      make(chan struct{}, 1),
	}
}

// Trigger queues the guild once the triggers for it stop for the debounce delay.
func (w *summaryWorker) Trigger(guildID string) {
	w.debounce.Trigger(guildID, func() { w.enqueue(guildID) })
}

func (w *summaryWorker) enqueue(guildID string) {
	w.mu.Lock()
	if !w.queued[guildID] {
		w.queued[guildID] = true
		w.queue = append(w.queue, guildID)
	}
	w.mu.Unlock()

	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *summaryWorker) pop() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.queue) == 0 {
		return "", false
	}
	guildID := w.queue[0]
	w.queue = w.queue[1:]
	delete(w.queued, guildID)
	return guildID, true
}

// Run reconciles the queued guilds until ctx ends.
func (w *summaryWorker) Run(ctx context.Context) {
	defer w.stopTimers()
	for {
		guildID, ok := w.pop()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-w.wake:
				continue
			}
		}

		started := w.now()
		res := w.reconcile(ctx, guildID)
		if ctx.Err() != nil {
			return
		}
		delay, scheduled := w.schedule(guildID, res)
		log := w.log.With("guild.ID", guildID, "duration_ms", w.now().Sub(started).Milliseconds())
		if scheduled {
			log = log.With("next_pass_in", delay.String())
		}
		if res.Err != nil {
			log.Warnw("summary pass failed", "error", res.Err)
			continue
		}
		log.Debug("summary pass done")
	}
}

// schedule replaces the guild's timer with the earliest of its next boundary,
// the requested pass and the retry. A guild with none of them gets no timer.
func (w *summaryWorker) schedule(guildID string, res reconcileResult) (time.Duration, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[guildID]; ok {
		t.Stop()
		delete(w.timers, guildID)
	}

	delay, ok := w.nextDelay(guildID, res)
	if !ok {
		return 0, false
	}
	w.timers[guildID] = w.afterFunc(delay, func() { w.enqueue(guildID) })
	return delay, true
}

// nextDelay needs w.mu.
func (w *summaryWorker) nextDelay(guildID string, res reconcileResult) (time.Duration, bool) {
	var delays []time.Duration
	if res.Err != nil {
		retry := firstRetryDelay
		if last, ok := w.backoff[guildID]; ok {
			retry = min(2*last, maxRetryDelay)
		}
		w.backoff[guildID] = retry
		delays = append(delays, retry)
	} else {
		delete(w.backoff, guildID)
	}
	if res.Again > 0 {
		delays = append(delays, res.Again)
	}
	if !res.Next.IsZero() {
		delays = append(delays, max(res.Next.Sub(w.now())+boundarySlack, 0))
	}
	if len(delays) == 0 {
		return 0, false
	}
	return min(slices.Min(delays), maxSummaryDelay), true
}

func (w *summaryWorker) stopTimers() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for guildID, t := range w.timers {
		t.Stop()
		delete(w.timers, guildID)
	}
}

// pacer keeps a minimum gap between the bot's own Discord writes. discordgo
// waits out a 429 by itself; the pacer keeps the bot from causing one.
type pacer struct {
	mu    sync.Mutex
	last  time.Time
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

func newPacer() *pacer {
	return &pacer{now: time.Now, sleep: sleepContext}
}

// Wait returns when gap has passed since the previous write, or when ctx ends.
func (p *pacer) Wait(ctx context.Context, gap time.Duration) error {
	p.mu.Lock()
	now := p.now()
	at := p.last.Add(gap)
	if at.Before(now) {
		at = now
	}
	p.last = at
	p.mu.Unlock()

	if wait := at.Sub(now); wait > 0 {
		return p.sleep(ctx, wait)
	}
	return ctx.Err()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

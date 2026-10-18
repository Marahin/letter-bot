package bot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var workerNow = time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

// recordedWorker reconciles by recording the guild ids and runs no timer.
type recordedWorker struct {
	*summaryWorker

	recMu   sync.Mutex
	passes  []string
	delays  []time.Duration
	results map[string]reconcileResult
}

func newRecordedWorker() *recordedWorker {
	r := &recordedWorker{results: map[string]reconcileResult{}}
	r.summaryWorker = newSummaryWorker(func(_ context.Context, guildID string) reconcileResult {
		r.recMu.Lock()
		defer r.recMu.Unlock()
		r.passes = append(r.passes, guildID)
		return r.results[guildID]
	}, time.Millisecond)
	r.now = func() time.Time { return workerNow }
	r.afterFunc = func(d time.Duration, f func()) *time.Timer {
		r.recMu.Lock()
		r.delays = append(r.delays, d)
		r.recMu.Unlock()
		return time.AfterFunc(time.Hour, f)
	}
	return r
}

func (r *recordedWorker) recordedPasses() []string {
	r.recMu.Lock()
	defer r.recMu.Unlock()
	return append([]string(nil), r.passes...)
}

// runOnce reconciles what is queued now, then returns.
func (r *recordedWorker) runOnce(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()
	require.Eventually(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return len(r.queue) == 0
	}, 5*time.Second, time.Millisecond)
	cancel()
	<-done
}

func TestSummaryWorker_DedupesQueuedGuilds(t *testing.T) {
	// given
	w := newRecordedWorker()
	w.enqueue("g1")
	w.enqueue("g2")
	w.enqueue("g1")

	// when
	w.runOnce(t)

	// then
	assert.Equal(t, []string{"g1", "g2"}, w.recordedPasses())
}

func TestSummaryWorker_TriggerDebounces(t *testing.T) {
	// given
	w := newRecordedWorker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// when
	w.Trigger("g1")
	w.Trigger("g1")

	// then
	require.Eventually(t, func() bool { return len(w.recordedPasses()) == 1 }, 5*time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, []string{"g1"}, w.recordedPasses())
}

func TestSummaryWorker_TriggerDuringAPassRunsOnceMore(t *testing.T) {
	// given
	w := newRecordedWorker()
	release := make(chan struct{})
	inPass := make(chan struct{})
	passes := 0
	w.reconcile = func(context.Context, string) reconcileResult {
		passes++
		if passes == 1 {
			close(inPass)
			<-release
		}
		return reconcileResult{}
	}
	w.enqueue("g1")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	<-inPass

	// when
	w.enqueue("g1")
	w.enqueue("g1")
	close(release)

	// then
	require.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return len(w.queue) == 0 && !w.queued["g1"]
	}, 5*time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, 2, passes)
}

func TestSummaryWorker_ScheduleTakesTheEarliest(t *testing.T) {
	cases := map[string]struct {
		res      reconcileResult
		expected time.Duration
	}{
		"boundary plus slack": {reconcileResult{Next: workerNow.Add(time.Hour)}, time.Hour + boundarySlack},
		"again before the boundary": {
			reconcileResult{Next: workerNow.Add(time.Hour), Again: 5 * time.Second}, 5 * time.Second,
		},
		"retry before the boundary": {
			reconcileResult{Next: workerNow.Add(time.Hour), Err: errors.New("down")}, firstRetryDelay,
		},
		"boundary before the retry": {
			reconcileResult{Next: workerNow.Add(time.Second), Err: errors.New("down")}, 2 * time.Second,
		},
		"capped at a day": {reconcileResult{Next: workerNow.Add(72 * time.Hour)}, maxSummaryDelay},
		"past boundary":   {reconcileResult{Next: workerNow.Add(-time.Hour)}, 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			w := newRecordedWorker()

			// when
			delay, ok := w.schedule("g1", tc.res)
			defer w.stopTimers()

			// then
			assert.True(t, ok)
			assert.Equal(t, tc.expected, delay)
			assert.Equal(t, []time.Duration{tc.expected}, w.delays)
		})
	}
}

func TestSummaryWorker_NothingToWaitForStopsTheTimer(t *testing.T) {
	// given
	w := newRecordedWorker()
	_, _ = w.schedule("g1", reconcileResult{Next: workerNow.Add(time.Hour)})
	timer := w.timers["g1"]

	// when
	_, ok := w.schedule("g1", reconcileResult{})

	// then
	assert.False(t, ok)
	assert.NotContains(t, w.timers, "g1")
	assert.False(t, timer.Stop(), "the old timer was stopped")
}

func TestSummaryWorker_BackoffDoublesCapsAndResets(t *testing.T) {
	// given
	w := newRecordedWorker()
	defer w.stopTimers()
	failed := reconcileResult{Err: errors.New("down")}
	delays := make([]time.Duration, 0, 7)

	// when
	for range 7 {
		d, _ := w.schedule("g1", failed)
		delays = append(delays, d)
	}
	_, okAfterSuccess := w.schedule("g1", reconcileResult{})
	afterReset, _ := w.schedule("g1", failed)

	// then
	assert.Equal(t, []time.Duration{
		30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, maxRetryDelay, maxRetryDelay,
	}, delays)
	assert.False(t, okAfterSuccess)
	assert.Equal(t, firstRetryDelay, afterReset)
}

func TestSummaryWorker_TimerQueuesTheGuild(t *testing.T) {
	// given
	w := newRecordedWorker()
	w.afterFunc = time.AfterFunc
	w.now = time.Now
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// when
	_, _ = w.schedule("g1", reconcileResult{Again: time.Millisecond})

	// then
	require.Eventually(t, func() bool { return len(w.recordedPasses()) == 1 }, 5*time.Second, time.Millisecond)
}

func TestSummaryWorker_RunStopsTheTimers(t *testing.T) {
	// given
	w := newRecordedWorker()
	w.results["g1"] = reconcileResult{Next: workerNow.Add(time.Hour)}
	w.enqueue("g1")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	require.Eventually(t, func() bool {
		w.recMu.Lock()
		defer w.recMu.Unlock()
		return len(w.delays) == 1
	}, 5*time.Second, time.Millisecond)

	// when
	cancel()
	<-done

	// then
	assert.Empty(t, w.timers)
}

func TestPacer_KeepsTheGapBetweenWrites(t *testing.T) {
	// given
	now := workerNow
	var slept []time.Duration
	p := &pacer{
		now: func() time.Time { return now },
		sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	}
	ctx := context.Background()

	// when
	first := p.Wait(ctx, time.Second)
	second := p.Wait(ctx, time.Second)
	third := p.Wait(ctx, 5*time.Second)
	now = now.Add(time.Minute)
	afterAPause := p.Wait(ctx, time.Second)

	// then
	require.NoError(t, errors.Join(first, second, third, afterAPause))
	assert.Equal(t, []time.Duration{time.Second, 6 * time.Second}, slept)
}

func TestPacer_StopsWithTheContext(t *testing.T) {
	// given
	p := newPacer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, p.Wait(ctx, 0), context.Canceled)

	// when
	err := p.Wait(ctx, time.Hour)

	// then
	assert.ErrorIs(t, err, context.Canceled)
}

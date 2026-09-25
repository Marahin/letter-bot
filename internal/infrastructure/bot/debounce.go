package bot

import (
	"sync"
	"time"
)

// debouncer runs fn once per key after the key has been quiet for delay.
type debouncer struct {
	delay   time.Duration
	mu      sync.Mutex
	pending map[string]*time.Timer
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{delay: delay, pending: map[string]*time.Timer{}}
}

func (d *debouncer) Trigger(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if t, ok := d.pending[key]; ok {
		t.Stop()
	}
	d.pending[key] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.pending, key)
		d.mu.Unlock()
		fn()
	})
}

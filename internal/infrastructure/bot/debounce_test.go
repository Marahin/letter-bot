package bot

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDebouncer_CoalescesPerKey(t *testing.T) {
	// given
	d := newDebouncer(20 * time.Millisecond)
	var a, b atomic.Int32

	// when
	for range 5 {
		d.Trigger("a", func() { a.Add(1) })
	}
	d.Trigger("b", func() { b.Add(1) })

	// then
	assert.Eventually(t, func() bool { return a.Load() == 1 && b.Load() == 1 }, time.Second, 5*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	assert.Equal(t, int32(1), a.Load())
	assert.Equal(t, int32(1), b.Load())
}

func TestDebouncer_RunsAgainAfterFiring(t *testing.T) {
	// given
	d := newDebouncer(5 * time.Millisecond)
	var calls atomic.Int32
	d.Trigger("a", func() { calls.Add(1) })
	assert.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)

	// when
	d.Trigger("a", func() { calls.Add(1) })

	// then
	assert.Eventually(t, func() bool { return calls.Load() == 2 }, time.Second, time.Millisecond)
}

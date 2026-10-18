package experience

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/experience"
)

var t0 = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func snap(exp int64, observed, lastSeen time.Time) *experience.Snapshot {
	return &experience.Snapshot{Experience: exp, ObservedAt: observed, LastSeenAt: lastSeen}
}

func TestStartSnapshot(t *testing.T) {
	fresh := snap(100, t0.Add(-5*time.Hour), t0.Add(-10*time.Minute))
	seenAfterStart := snap(100, t0.Add(-5*time.Hour), t0.Add(10*time.Minute))
	edge := snap(100, t0.Add(-5*time.Hour), t0.Add(-MaxStartGap))
	stale := snap(100, t0.Add(-72*time.Hour), t0.Add(-MaxStartGap-time.Minute))
	soonAfter := snap(200, t0.Add(10*time.Minute), t0.Add(10*time.Minute))
	atFallbackEdge := snap(200, t0.Add(StartFallback), t0.Add(StartFallback))
	tooLate := snap(200, t0.Add(StartFallback+time.Minute), t0.Add(StartFallback+time.Minute))
	atStart := snap(200, t0, t0)

	tests := []struct {
		name       string
		atOrBefore *experience.Snapshot
		firstAfter *experience.Snapshot
		want       *experience.Snapshot
	}{
		{name: "fresh snapshot before start", atOrBefore: fresh, firstAfter: soonAfter, want: fresh},
		{name: "snapshot seen again after start", atOrBefore: seenAfterStart, want: seenAfterStart},
		{name: "seen exactly MaxStartGap before start", atOrBefore: edge, want: edge},
		{name: "stale snapshot falls back", atOrBefore: stale, firstAfter: soonAfter, want: soonAfter},
		{name: "no snapshot before falls back", firstAfter: soonAfter, want: soonAfter},
		{name: "fallback at the edge", firstAfter: atFallbackEdge, want: atFallbackEdge},
		{name: "fallback too late", atOrBefore: stale, firstAfter: tooLate},
		{name: "fallback must be after start", firstAfter: atStart},
		{name: "nothing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := StartSnapshot(tt.atOrBefore, tt.firstAfter, t0)

			// then
			assert.Same(t, tt.want, got)
		})
	}
}

func TestEndSnapshot(t *testing.T) {
	end := t0.Add(2 * time.Hour)
	seenAtCutoff := snap(300, end.Add(-3*time.Hour), end.Add(15*time.Minute))
	seenAtEnd := snap(300, end.Add(-3*time.Hour), end)
	notSeenAfterEnd := snap(300, end.Add(-3*time.Hour), end.Add(-time.Minute))

	// when / then
	assert.Same(t, seenAtCutoff, EndSnapshot(seenAtCutoff, end))
	assert.Same(t, seenAtEnd, EndSnapshot(seenAtEnd, end))
	assert.Nil(t, EndSnapshot(notSeenAfterEnd, end))
	assert.Nil(t, EndSnapshot(nil, end))
}

func TestComputeGain(t *testing.T) {
	tests := []struct {
		name       string
		start, end *experience.Snapshot
		wantStatus experience.Status
		wantGain   *int64
		wantStart  *int64
		wantEnd    *int64
	}{
		{name: "gain", start: snap(1000, t0, t0), end: snap(1500, t0, t0), wantStatus: experience.StatusOK, wantGain: ptr(500), wantStart: ptr(1000), wantEnd: ptr(1500)},
		{name: "no change", start: snap(1000, t0, t0), end: snap(1000, t0, t0), wantStatus: experience.StatusOK, wantGain: ptr(0), wantStart: ptr(1000), wantEnd: ptr(1000)},
		{name: "death makes it negative", start: snap(1000, t0, t0), end: snap(900, t0, t0), wantStatus: experience.StatusOK, wantGain: ptr(-100), wantStart: ptr(1000), wantEnd: ptr(900)},
		{name: "no start", end: snap(1500, t0, t0), wantStatus: experience.StatusNoData, wantEnd: ptr(1500)},
		{name: "no end", start: snap(1000, t0, t0), wantStatus: experience.StatusNoData, wantStart: ptr(1000)},
		{name: "nothing", wantStatus: experience.StatusNoData},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			res := ComputeGain(tt.start, tt.end)

			// then
			require.Equal(t, tt.wantStatus, res.Status)
			assert.Equal(t, tt.wantGain, res.Gain)
			assert.Equal(t, tt.wantStart, res.Start)
			assert.Equal(t, tt.wantEnd, res.End)
		})
	}
}

func ptr(v int64) *int64 { return &v }

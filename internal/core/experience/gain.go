package experience

import (
	"time"

	"spot-assistant/internal/core/dto/experience"
)

const (
	// StartFallback is how long after start_at a first sighting still counts as the start value.
	StartFallback = 20 * time.Minute
	// MaxStartGap is how long before start_at the start snapshot must still have been seen. An older
	// sighting means the character was untracked or out of the top 1000 in between, so the value is stale.
	MaxStartGap = 2 * time.Hour
)

// Result is the experience of one character over one reservation.
type Result struct {
	Start  *int64
	End    *int64
	Gain   *int64
	Status experience.Status
}

// StartSnapshot picks the start value: the newest snapshot at or before start_at if it was still seen
// within MaxStartGap of it, else the first snapshot within StartFallback after start_at.
func StartSnapshot(atOrBefore, firstAfter *experience.Snapshot, startAt time.Time) *experience.Snapshot {
	if atOrBefore != nil && !atOrBefore.LastSeenAt.Before(startAt.Add(-MaxStartGap)) {
		return atOrBefore
	}
	if firstAfter != nil && firstAfter.ObservedAt.After(startAt) && !firstAfter.ObservedAt.After(startAt.Add(StartFallback)) {
		return firstAfter
	}
	return nil
}

// EndSnapshot accepts the newest snapshot at or before the end cut-off run only if it was seen at or
// after end_at, so its value is the one the first run after the reservation saw.
func EndSnapshot(atOrBeforeCutoff *experience.Snapshot, endAt time.Time) *experience.Snapshot {
	if atOrBeforeCutoff == nil || atOrBeforeCutoff.LastSeenAt.Before(endAt) {
		return nil
	}
	return atOrBeforeCutoff
}

// ComputeGain returns end - start, or no data when either value is missing. A death makes it negative.
func ComputeGain(start, end *experience.Snapshot) Result {
	var res Result
	if start != nil {
		v := start.Experience
		res.Start = &v
	}
	if end != nil {
		v := end.Experience
		res.End = &v
	}
	if res.Start == nil || res.End == nil {
		res.Status = experience.StatusNoData
		return res
	}
	gain := *res.End - *res.Start
	res.Gain = &gain
	res.Status = experience.StatusOK
	return res
}

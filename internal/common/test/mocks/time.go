package mocks

import "time"

// TimeMatchedCloseTo returns true if the given time is within 2 milliseconds of the current time.
func TimeMatchedCloseTo(t time.Time) bool {
	return NewMatcherForTimeAndTolerance(
		t, 2*time.Millisecond,
	)(time.Now())
}

// NewMatcherForTimeAndTolerance returns a function that takes a time argument and returns a boolean depending whether the input is within a tolerance duration of the original time.
func NewMatcherForTimeAndTolerance(t time.Time, tolerance time.Duration) func(time.Time) bool {
	return func(t2 time.Time) bool {
		var diff time.Duration

		if t.After(t2) {
			diff = t.Sub(t2)
		} else {
			diff = t2.Sub(t)
		}

		return diff < tolerance
	}
}

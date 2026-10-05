package reservationforms

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/reservation"
)

// ErrTimeFormat means a time of day that ParseClock cannot read.
var ErrTimeFormat = errors.New("use HH:MM, e.g. 18:30")

var clockSeparators = strings.NewReplacer(".", ":", ";", ":", ",", ":", "h", ":")

// ParseClock reads a time of day: H, HH, H:MM, HH:MM, HHMM, with ".", ";", ","
// or "h" in place of ":". 24:MM is 00:MM.
func ParseClock(s string) (hour, minute int, err error) {
	s = clockSeparators.Replace(strings.ToLower(strings.TrimSpace(s)))
	hourText, minuteText, found := strings.Cut(s, ":")
	if !found {
		switch len(s) {
		case 1, 2:
			hourText, minuteText = s, "00"
		case 3, 4:
			hourText, minuteText = s[:len(s)-2], s[len(s)-2:]
		default:
			return 0, 0, ErrTimeFormat
		}
	}
	if minuteText == "" {
		minuteText = "00"
	}
	if len(hourText) < 1 || len(hourText) > 2 || len(minuteText) != 2 || !isDigits(hourText) || !isDigits(minuteText) {
		return 0, 0, ErrTimeFormat
	}
	hour, _ = strconv.Atoi(hourText)
	minute, _ = strconv.Atoi(minuteText)
	if hour > 24 || minute > 59 {
		return 0, 0, ErrTimeFormat
	}
	return hour % 24, minute, nil
}

// NextWindow places a new reservation: the start is the next occurrence of the
// time of day, the current minute included. The end is the first occurrence
// after the start.
func NextWindow(start, end string, now time.Time) (time.Time, time.Time, error) {
	sh, sm, err := ParseClock(start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	eh, em, err := ParseClock(end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	startAt := atClock(now, 0, sh, sm)
	if startAt.Before(now.Truncate(time.Minute)) {
		startAt = atClock(now, 1, sh, sm)
	}
	return startAt, endAfter(startAt, eh, em), nil
}

// EditWindow places a changed reservation. An unchanged start keeps its date, so
// an ongoing reservation keeps its start. A changed start is the occurrence
// nearest to the current start that is not in the past.
func EditWindow(start, end string, existing reservation.Reservation, now time.Time) (time.Time, time.Time, error) {
	sh, sm, err := ParseClock(start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	eh, em, err := ParseClock(end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	current := existing.StartAt.In(now.Location())
	startAt := current
	if current.Hour() != sh || current.Minute() != sm {
		startAt = nearestFuture(current, sh, sm, now.Truncate(time.Minute))
	}
	return startAt, endAfter(startAt, eh, em), nil
}

func nearestFuture(anchor time.Time, hour, minute int, notBefore time.Time) time.Time {
	var best time.Time
	for days := -1; days <= 2; days++ {
		candidate := atClock(anchor, days, hour, minute)
		if candidate.Before(notBefore) {
			continue
		}
		if best.IsZero() || absDuration(candidate.Sub(anchor)) < absDuration(best.Sub(anchor)) {
			best = candidate
		}
	}
	return best
}

func endAfter(startAt time.Time, hour, minute int) time.Time {
	endAt := atClock(startAt, 0, hour, minute)
	if !endAt.After(startAt) {
		endAt = atClock(startAt, 1, hour, minute)
	}
	return endAt
}

func atClock(day time.Time, addDays, hour, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day()+addDays, hour, minute, 0, 0, day.Location())
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

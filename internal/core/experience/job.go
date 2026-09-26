// Package experience records experience highscore snapshots of tracked characters and turns them into
// the experience gained per reservation.
package experience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/world"
	"spot-assistant/internal/ports"
)

const (
	// MaxPages is the TibiaData page count of the top 1000.
	MaxPages = 20
	// PageSize is the row count of every highscore page but the last.
	PageSize = 50
	// TrackingWindow keeps a character tracked this long after its reservation ended.
	TrackingWindow = 24 * time.Hour
	// AttributionWindow gives up on reservations that ended longer ago than this.
	AttributionWindow = 48 * time.Hour
)

// ErrIncompletePage is a highscore page with fewer rows than it must have: TibiaData served a
// broken or partial list, and a run on it would miss characters that are in the top 1000.
var ErrIncompletePage = errors.New("incomplete highscore page")

type Job struct {
	api  ports.HighscoreAPI
	repo ports.ExperienceRepository
	// endFallback is how long after the end cut-off run a later run may stand in for it.
	endFallback time.Duration
	log         *zap.SugaredLogger
}

// New builds the job that runs every interval.
func New(api ports.HighscoreAPI, repo ports.ExperienceRepository, interval time.Duration, log *zap.SugaredLogger) *Job {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	// Two intervals: the next run's data time drifts with the highscore age, so it can land a bit past one.
	return &Job{api: api, repo: repo, endFallback: 2 * interval, log: log}
}

// RunOnce collects and attributes every tracked world. A failing world does not stop the others.
func (j *Job) RunOnce(ctx context.Context, now time.Time) error {
	worlds, err := j.repo.ListTrackedWorlds(ctx)
	if err != nil {
		return fmt.Errorf("list tracked worlds: %w", err)
	}

	var errs []error
	for _, w := range worlds {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err := j.Collect(ctx, w, now); err != nil {
			errs = append(errs, fmt.Errorf("collect %s: %w", w, err))
		}
		// Attribution only reads stored runs, so it still makes progress after a failed collect.
		if err := j.Attribute(ctx, w, now); err != nil {
			errs = append(errs, fmt.Errorf("attribute %s: %w", w, err))
		}
	}
	return errors.Join(errs...)
}

// Collect reads the world's highscores and stores the tracked characters whose experience changed.
// A failed page aborts the run without a run row, so no end cut-off is based on partial data.
func (j *Job) Collect(ctx context.Context, worldName string, now time.Time) error {
	keys, err := j.repo.ListTrackedCharacterKeys(ctx, worldName, now.Add(-TrackingWindow))
	if err != nil {
		return fmt.Errorf("list tracked characters: %w", err)
	}
	if len(keys) == 0 {
		j.log.Debugw("no tracked characters, skipping highscores", "world", worldName)
		return nil
	}

	entries, observedAt, pages, err := j.fetch(ctx, worldName)
	if err != nil {
		return err
	}

	found, foundKeys := trackedEntries(entries, keys)

	latest, err := j.repo.LatestSnapshots(ctx, worldName, foundKeys)
	if err != nil {
		return fmt.Errorf("latest snapshots: %w", err)
	}

	result := experience.RunResult{Run: experience.Run{
		World:      worldName,
		ObservedAt: observedAt,
		FetchedAt:  now,
		Pages:      pages,
		Rows:       len(entries),
	}}
	result.Inserted, result.SeenIDs = diff(worldName, observedAt, foundKeys, found, latest)

	if err := j.repo.SaveRun(ctx, result); err != nil {
		return fmt.Errorf("save run: %w", err)
	}
	j.log.Infow("highscores collected", "world", worldName, "pages", pages, "rows", len(entries),
		"tracked", len(keys), "found", len(foundKeys), "inserted", len(result.Inserted), "observedAt", observedAt)
	return nil
}

// trackedEntries keeps the first highscore entry of each tracked character, in highscore order.
func trackedEntries(entries []world.HighscoreEntry, keys []string) (map[string]world.HighscoreEntry, []string) {
	tracked := make(map[string]bool, len(keys))
	for _, k := range keys {
		tracked[k] = true
	}
	found := map[string]world.HighscoreEntry{}
	var foundKeys []string
	for _, e := range entries {
		key := experience.CharacterKey(e.Name)
		if !tracked[key] {
			continue
		}
		if _, dup := found[key]; dup {
			continue
		}
		found[key] = e
		foundKeys = append(foundKeys, key)
	}
	return found, foundKeys
}

// diff returns the snapshots to insert (new or changed value) and the ids of snapshots seen again unchanged.
func diff(worldName string, observedAt time.Time, keys []string, found map[string]world.HighscoreEntry,
	latest map[string]experience.Snapshot) ([]experience.Snapshot, []int64) {
	var inserted []experience.Snapshot
	var seen []int64
	for _, key := range keys {
		e := found[key]
		prev, ok := latest[key]
		switch {
		case ok && prev.Experience == e.Value && prev.Level == e.Level:
			if observedAt.After(prev.LastSeenAt) {
				seen = append(seen, prev.ID)
			}
		case ok && !observedAt.After(prev.LastSeenAt):
			// Older data than what is stored (a stale TibiaData cache); never rewrite history.
		default:
			inserted = append(inserted, experience.Snapshot{
				World:         worldName,
				CharacterKey:  key,
				CharacterName: e.Name,
				Level:         e.Level,
				Experience:    e.Value,
				Vocation:      e.Vocation,
				ObservedAt:    observedAt,
				LastSeenAt:    observedAt,
			})
		}
	}
	return inserted, seen
}

// fetch reads pages 1..min(MaxPages, total pages). observedAt is the oldest page's data time.
// An empty page, or a short page before the last one, fails the whole run.
func (j *Job) fetch(ctx context.Context, worldName string) ([]world.HighscoreEntry, time.Time, int, error) {
	var entries []world.HighscoreEntry
	var observedAt time.Time
	total := 1
	page := 1
	for ; page <= total; page++ {
		res, err := j.api.GetHighscoresPage(ctx, worldName, page)
		if err != nil {
			return nil, time.Time{}, 0, fmt.Errorf("highscores page %d: %w", page, err)
		}
		if n := len(res.Entries); n == 0 || (page < res.TotalPages && n < PageSize) {
			return nil, time.Time{}, 0, fmt.Errorf("highscores page %d of %d has %d rows: %w", page, res.TotalPages, n, ErrIncompletePage)
		}
		if page == 1 {
			total = min(MaxPages, res.TotalPages)
		}
		entries = append(entries, res.Entries...)
		if page == 1 || res.ObservedAt.Before(observedAt) {
			observedAt = res.ObservedAt
		}
	}
	return entries, observedAt, page - 1, nil
}

// Attribute stores the experience of each character of every ended reservation whose end cut-off run exists.
func (j *Job) Attribute(ctx context.Context, worldName string, now time.Time) error {
	firstRun, err := j.repo.FirstRunObservedAt(ctx, worldName)
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("first run: %w", err)
	}

	pending, err := j.repo.PendingReservations(ctx, worldName, firstRun, now.Add(-AttributionWindow), now)
	if err != nil {
		return fmt.Errorf("pending reservations: %w", err)
	}

	done := 0
	for _, r := range pending {
		stored, err := j.attributeReservation(ctx, worldName, r)
		if err != nil {
			return fmt.Errorf("reservation %d: %w", r.ID, err)
		}
		if stored {
			done++
		}
	}
	if len(pending) > 0 {
		j.log.Infow("reservation experience computed", "world", worldName, "pending", len(pending), "stored", done)
	}
	return nil
}

// attributeReservation returns false when the reservation must wait for a later run.
func (j *Job) attributeReservation(ctx context.Context, worldName string, r experience.PendingReservation) (bool, error) {
	cutoff, err := j.repo.FirstRunObservedAtOrAfter(ctx, worldName, r.EndAt)
	if errors.Is(err, ports.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("end cut-off run: %w", err)
	}
	fallback := endFallback{job: j, world: worldName, cutoff: cutoff}

	characters := experience.Characters(r.Author)
	rows := make([]experience.ReservationExperience, 0, len(characters))
	for _, c := range characters {
		start, err := j.startSnapshot(ctx, worldName, c.Key, r.StartAt)
		if err != nil {
			return false, err
		}
		atCutoff, err := optional(j.repo.SnapshotAtOrBefore(ctx, worldName, c.Key, cutoff))
		if err != nil {
			return false, fmt.Errorf("end snapshot: %w", err)
		}
		end := EndSnapshot(atCutoff, r.EndAt)
		if end == nil {
			var wait bool
			if end, wait, err = fallback.snapshot(ctx, c.Key, r.EndAt); err != nil {
				return false, err
			}
			if wait {
				return false, nil
			}
		}
		res := ComputeGain(start, end)
		rows = append(rows, experience.ReservationExperience{
			ReservationID:   r.ID,
			CharacterKey:    c.Key,
			CharacterName:   c.Name,
			StartExperience: res.Start,
			EndExperience:   res.End,
			Gain:            res.Gain,
			Status:          res.Status,
		})
	}
	if err := j.repo.InsertReservationExperience(ctx, rows); err != nil {
		return false, fmt.Errorf("insert: %w", err)
	}
	return len(rows) > 0, nil
}

// endFallback finds the end value of a character the cut-off run did not see (one missed run is
// often a TibiaData hiccup) in the next run, if that run came within endFallback of the cut-off.
type endFallback struct {
	job    *Job
	world  string
	cutoff time.Time

	looked bool
	next   time.Time
	usable bool
}

// snapshot returns wait when no run followed the cut-off run yet.
func (f *endFallback) snapshot(ctx context.Context, key string, endAt time.Time) (*experience.Snapshot, bool, error) {
	if !f.looked {
		// Run times have microsecond precision, so this is the first run strictly after the cut-off.
		next, err := f.job.repo.FirstRunObservedAtOrAfter(ctx, f.world, f.cutoff.Add(time.Microsecond))
		switch {
		case errors.Is(err, ports.ErrNotFound):
			return nil, true, nil
		case err != nil:
			return nil, false, fmt.Errorf("fallback end run: %w", err)
		}
		f.looked, f.next, f.usable = true, next, !next.After(f.cutoff.Add(f.job.endFallback))
	}
	if !f.usable {
		return nil, false, nil
	}
	end, err := optional(f.job.repo.SnapshotAtOrBefore(ctx, f.world, key, f.next))
	if err != nil {
		return nil, false, fmt.Errorf("fallback end snapshot: %w", err)
	}
	return EndSnapshot(end, endAt), false, nil
}

func (j *Job) startSnapshot(ctx context.Context, worldName, key string, startAt time.Time) (*experience.Snapshot, error) {
	before, err := optional(j.repo.SnapshotAtOrBefore(ctx, worldName, key, startAt))
	if err != nil {
		return nil, fmt.Errorf("start snapshot: %w", err)
	}
	if s := StartSnapshot(before, nil, startAt); s != nil {
		return s, nil
	}
	after, err := optional(j.repo.FirstSnapshotAfter(ctx, worldName, key, startAt, startAt.Add(StartFallback)))
	if err != nil {
		return nil, fmt.Errorf("fallback start snapshot: %w", err)
	}
	return StartSnapshot(nil, after, startAt), nil
}

func optional(s *experience.Snapshot, err error) (*experience.Snapshot, error) {
	if errors.Is(err, ports.ErrNotFound) {
		return nil, nil
	}
	return s, err
}

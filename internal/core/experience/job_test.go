package experience

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/world"
	"spot-assistant/internal/ports"
)

var (
	now     = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

func newJob(t *testing.T) (*Job, *mocks.MockHighscoreAPI, *mocks.MockExperienceRepository) {
	api := mocks.NewMockHighscoreAPI(t)
	repo := mocks.NewMockExperienceRepository(t)
	return New(api, repo, interval, nil), api, repo
}

const interval = 15 * time.Minute

func page(total int, observed time.Time, entries ...world.HighscoreEntry) *world.HighscorePage {
	return &world.HighscorePage{Entries: entries, ObservedAt: observed, TotalPages: total}
}

// full pads entries with untracked characters to a whole page.
func full(entries ...world.HighscoreEntry) []world.HighscoreEntry {
	for i := len(entries); i < PageSize; i++ {
		entries = append(entries, entry(fmt.Sprintf("Filler %d", i), 1, 1))
	}
	return entries
}

func entry(name string, level int, value int64) world.HighscoreEntry {
	return world.HighscoreEntry{Name: name, Level: level, Value: value, Vocation: "Elite Knight"}
}

func TestJob_Collect_StoresChangedTrackedCharacters(t *testing.T) {
	// given
	job, api, repo := newJob(t)
	observed := now.Add(-12 * time.Minute)
	tracked := []string{"quiet nyx", "storm quiet", "stale seen", "dark quiet", "old news", "fresh one", "same again"}
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", now.Add(-TrackingWindow)).Return(tracked, nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 1).Return(page(2, observed, full(
		entry("Quiet Nyx", 500, 1000),
		entry("Untracked Guy", 900, 9000),
		entry("Storm Quiet", 400, 800),
		entry("Stale Seen", 350, 750),
	)...), nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 2).Return(page(2, observed,
		entry("Dark Quiet", 300, 700),
		entry("Old News", 200, 600),
		entry("Fresh One", 100, 500),
		entry("Same Again", 100, 400),
		entry("quiet nyx", 1, 1),
	), nil)
	repo.EXPECT().LatestSnapshots(mock.Anything, "Celesta", tracked).
		Return(map[string]experience.Snapshot{
			"quiet nyx":   {ID: 1, Level: 500, Experience: 1000, ObservedAt: observed.Add(-time.Hour), LastSeenAt: observed.Add(-15 * time.Minute)},
			"storm quiet": {ID: 2, Level: 399, Experience: 790, ObservedAt: observed.Add(-time.Hour), LastSeenAt: observed.Add(-15 * time.Minute)},
			"dark quiet":  {ID: 3, Level: 300, Experience: 690, ObservedAt: observed.Add(time.Minute), LastSeenAt: observed.Add(time.Minute)},
			// Its value was seen after this (stale) page's data time: the change is older than what is stored.
			"stale seen": {ID: 5, Level: 350, Experience: 740, ObservedAt: observed.Add(-time.Hour), LastSeenAt: observed.Add(5 * time.Minute)},
			"same again": {ID: 4, Level: 100, Experience: 400, ObservedAt: observed.Add(-time.Hour), LastSeenAt: observed},
		}, nil)
	var saved experience.RunResult
	repo.EXPECT().SaveRun(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, r experience.RunResult) error {
		saved = r
		return nil
	})

	// when
	err := job.Collect(context.Background(), "Celesta", now)

	// then
	require.NoError(t, err)
	assert.Equal(t, experience.Run{World: "Celesta", ObservedAt: observed, FetchedAt: now, Pages: 2, Rows: PageSize + 5}, saved.Run)
	assert.Equal(t, []int64{1}, saved.SeenIDs)
	require.Len(t, saved.Inserted, 3)
	assert.Equal(t, experience.Snapshot{World: "Celesta", CharacterKey: "storm quiet", CharacterName: "Storm Quiet", Level: 400, Experience: 800,
		Vocation: "Elite Knight", ObservedAt: observed, LastSeenAt: observed}, saved.Inserted[0])
	assert.Equal(t, "old news", saved.Inserted[1].CharacterKey)
	assert.Equal(t, "fresh one", saved.Inserted[2].CharacterKey)
}

func TestJob_Collect_StopsAtMaxPages(t *testing.T) {
	// given
	job, api, repo := newJob(t)
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return([]string{"nobody"}, nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", mock.Anything).Return(page(25, now, full()...), nil).Times(MaxPages)
	repo.EXPECT().LatestSnapshots(mock.Anything, "Celesta", []string(nil)).Return(map[string]experience.Snapshot{}, nil)
	repo.EXPECT().SaveRun(mock.Anything, mock.MatchedBy(func(r experience.RunResult) bool {
		return r.Run.Pages == MaxPages && r.Run.ObservedAt.Equal(now) && len(r.Inserted) == 0
	})).Return(nil)

	// when
	err := job.Collect(context.Background(), "Celesta", now)

	// then
	assert.NoError(t, err)
}

func TestJob_Collect_ObservedAtIsTheOldestPage(t *testing.T) {
	// given
	job, api, repo := newJob(t)
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return([]string{"nobody"}, nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 1).Return(page(3, now.Add(-6*time.Minute), full()...), nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 2).Return(page(3, now.Add(-20*time.Minute), full()...), nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 3).Return(page(3, now, entry("Last", 1, 1)), nil)
	repo.EXPECT().LatestSnapshots(mock.Anything, "Celesta", mock.Anything).Return(map[string]experience.Snapshot{}, nil)
	repo.EXPECT().SaveRun(mock.Anything, mock.MatchedBy(func(r experience.RunResult) bool {
		return r.Run.ObservedAt.Equal(now.Add(-20*time.Minute)) && r.Run.Pages == 3 && r.Run.Rows == 2*PageSize+1
	})).Return(nil)

	// when
	err := job.Collect(context.Background(), "Celesta", now)

	// then
	assert.NoError(t, err)
}

func TestJob_Collect_NoTrackedCharactersSkipsTheAPI(t *testing.T) {
	// given
	job, _, repo := newJob(t)
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return(nil, nil)

	// when
	err := job.Collect(context.Background(), "Celesta", now)

	// then
	assert.NoError(t, err)
}

func TestJob_Collect_PageErrorWritesNoRun(t *testing.T) {
	// given
	job, api, repo := newJob(t)
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return([]string{"quiet nyx"}, nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 1).Return(page(3, now, full(entry("Quiet Nyx", 1, 1))...), nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 2).Return(nil, ports.ErrUpstreamUnavailable)

	// when
	err := job.Collect(context.Background(), "Celesta", now)

	// then
	assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
	assert.ErrorContains(t, err, "page 2")
}

func TestJob_Collect_IncompletePageWritesNoRun(t *testing.T) {
	tests := []struct {
		name  string
		pages []*world.HighscorePage
	}{
		{name: "short page before the last", pages: []*world.HighscorePage{page(3, now, full()...), page(3, now, entry("A", 1, 1))}},
		{name: "empty first page", pages: []*world.HighscorePage{page(0, now)}},
		{name: "empty last page", pages: []*world.HighscorePage{page(2, now, full()...), page(2, now)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			job, api, repo := newJob(t)
			repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return([]string{"a"}, nil)
			for i, p := range tt.pages {
				api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", i+1).Return(p, nil)
			}

			// when
			err := job.Collect(context.Background(), "Celesta", now)

			// then
			assert.ErrorIs(t, err, ErrIncompletePage)
		})
	}
}

func TestJob_Collect_RepositoryErrors(t *testing.T) {
	t.Run("tracked characters", func(t *testing.T) {
		// given
		job, _, repo := newJob(t)
		repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return(nil, errBoom)

		// when / then
		assert.ErrorIs(t, job.Collect(context.Background(), "Celesta", now), errBoom)
	})
	t.Run("latest snapshots", func(t *testing.T) {
		// given
		job, api, repo := newJob(t)
		repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return([]string{"a"}, nil)
		api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 1).Return(page(1, now, entry("A", 1, 1)), nil)
		repo.EXPECT().LatestSnapshots(mock.Anything, "Celesta", mock.Anything).Return(nil, errBoom)

		// when / then
		assert.ErrorIs(t, job.Collect(context.Background(), "Celesta", now), errBoom)
	})
	t.Run("save run", func(t *testing.T) {
		// given
		job, api, repo := newJob(t)
		repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return([]string{"a"}, nil)
		api.EXPECT().GetHighscoresPage(mock.Anything, "Celesta", 1).Return(page(1, now, entry("A", 1, 1)), nil)
		repo.EXPECT().LatestSnapshots(mock.Anything, "Celesta", mock.Anything).Return(map[string]experience.Snapshot{}, nil)
		repo.EXPECT().SaveRun(mock.Anything, mock.Anything).Return(errBoom)

		// when / then
		assert.ErrorIs(t, job.Collect(context.Background(), "Celesta", now), errBoom)
	})
}

func TestJob_Attribute_StoresGainPerCharacter(t *testing.T) {
	// given
	job, _, repo := newJob(t)
	firstRun := now.Add(-10 * time.Hour)
	start := now.Add(-4 * time.Hour)
	end := now.Add(-2 * time.Hour)
	cutoff := end.Add(12 * time.Minute)
	repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(firstRun, nil)
	repo.EXPECT().PendingReservations(mock.Anything, "Celesta", firstRun, now.Add(-AttributionWindow), now).Return([]experience.PendingReservation{
		{ID: 7, Author: "Quiet Nyx/Storm Quiet/ dark quiet /Quiet Nyx/Nobody/Missed Once", StartAt: start, EndAt: end},
		{ID: 8, Author: "Quiet Nyx", StartAt: now.Add(-time.Hour), EndAt: now.Add(-5 * time.Minute)},
	}, nil)
	repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", end).Return(cutoff, nil)
	repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", now.Add(-5*time.Minute)).Return(time.Time{}, ports.ErrNotFound)

	// Quiet Nyx: covered start, seen at cut-off.
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "quiet nyx", start).
		Return(snap(1000, start.Add(-time.Hour), start.Add(-5*time.Minute)), nil)
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "quiet nyx", cutoff).
		Return(snap(1600, cutoff, cutoff), nil)
	// Storm Quiet: stale start, first seen 10 minutes after start.
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "storm quiet", start).
		Return(snap(100, start.Add(-72*time.Hour), start.Add(-70*time.Hour)), nil)
	repo.EXPECT().FirstSnapshotAfter(mock.Anything, "Celesta", "storm quiet", start, start.Add(StartFallback)).
		Return(snap(500, start.Add(10*time.Minute), start.Add(time.Hour)), nil)
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "storm quiet", cutoff).
		Return(snap(450, start.Add(time.Hour), cutoff), nil)
	// Dark Quiet: dropped out of the top 1000 before the end.
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "dark quiet", start).
		Return(snap(300, start.Add(-time.Hour), start), nil)
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "dark quiet", cutoff).
		Return(snap(300, start.Add(-time.Hour), end.Add(-time.Hour)), nil)
	// The cut-off run did not see Dark Quiet, so the next run stands in; it did not see it either.
	next := cutoff.Add(interval)
	repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", cutoff.Add(time.Microsecond)).Return(next, nil).Once()
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "dark quiet", next).
		Return(snap(300, start.Add(-time.Hour), end.Add(-time.Hour)), nil)
	// Nobody: never in the highscores.
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "nobody", mock.Anything).Return(nil, ports.ErrNotFound)
	repo.EXPECT().FirstSnapshotAfter(mock.Anything, "Celesta", "nobody", start, start.Add(StartFallback)).Return(nil, ports.ErrNotFound)
	// Missed Once: missing from the cut-off run only.
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "missed once", start).
		Return(snap(2000, start.Add(-time.Hour), start), nil)
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "missed once", cutoff).
		Return(snap(2000, start.Add(-time.Hour), start.Add(time.Hour)), nil)
	repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "missed once", next).
		Return(snap(2500, next, next), nil)

	var rows []experience.ReservationExperience
	repo.EXPECT().InsertReservationExperience(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, r []experience.ReservationExperience) error {
		rows = r
		return nil
	})

	// when
	err := job.Attribute(context.Background(), "Celesta", now)

	// then
	require.NoError(t, err)
	assert.Equal(t, []experience.ReservationExperience{
		{ReservationID: 7, CharacterKey: "quiet nyx", CharacterName: "Quiet Nyx", StartExperience: ptr(1000), EndExperience: ptr(1600), Gain: ptr(600), Status: experience.StatusOK},
		{ReservationID: 7, CharacterKey: "storm quiet", CharacterName: "Storm Quiet", StartExperience: ptr(500), EndExperience: ptr(450), Gain: ptr(-50), Status: experience.StatusOK},
		{ReservationID: 7, CharacterKey: "dark quiet", CharacterName: "dark quiet", StartExperience: ptr(300), Status: experience.StatusNoData},
		{ReservationID: 7, CharacterKey: "nobody", CharacterName: "Nobody", Status: experience.StatusNoData},
		{ReservationID: 7, CharacterKey: "missed once", CharacterName: "Missed Once", StartExperience: ptr(2000), EndExperience: ptr(2500), Gain: ptr(500), Status: experience.StatusOK},
	}, rows)
}

func TestJob_Attribute_EndFallback(t *testing.T) {
	start := now.Add(-3 * time.Hour)
	end := now.Add(-2 * time.Hour)
	cutoff := end.Add(5 * time.Minute)
	setup := func(t *testing.T) (*Job, *mocks.MockExperienceRepository) {
		job, _, repo := newJob(t)
		repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now.Add(-10*time.Hour), nil)
		repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).
			Return([]experience.PendingReservation{{ID: 9, Author: "Gone", StartAt: start, EndAt: end}}, nil)
		repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", end).Return(cutoff, nil)
		repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "gone", start).Return(snap(10, start.Add(-time.Hour), start), nil)
		repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "gone", cutoff).Return(snap(10, start.Add(-time.Hour), start), nil)
		return job, repo
	}

	t.Run("waits for the next run", func(t *testing.T) {
		// given
		job, repo := setup(t)
		repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", cutoff.Add(time.Microsecond)).Return(time.Time{}, ports.ErrNotFound)

		// when
		err := job.Attribute(context.Background(), "Celesta", now)

		// then
		assert.NoError(t, err)
		repo.AssertNotCalled(t, "InsertReservationExperience", mock.Anything, mock.Anything)
	})
	t.Run("a next run past the window gives no data", func(t *testing.T) {
		// given
		job, repo := setup(t)
		repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", cutoff.Add(time.Microsecond)).Return(cutoff.Add(2*interval+time.Second), nil)
		repo.EXPECT().InsertReservationExperience(mock.Anything, []experience.ReservationExperience{
			{ReservationID: 9, CharacterKey: "gone", CharacterName: "Gone", StartExperience: ptr(10), Status: experience.StatusNoData},
		}).Return(nil)

		// when / then
		assert.NoError(t, job.Attribute(context.Background(), "Celesta", now))
	})
	t.Run("errors", func(t *testing.T) {
		// given
		job, repo := setup(t)
		repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", cutoff.Add(time.Microsecond)).Return(time.Time{}, errBoom).Once()
		job2, repo2 := setup(t)
		repo2.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", cutoff.Add(time.Microsecond)).Return(cutoff.Add(interval), nil)
		repo2.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "gone", cutoff.Add(interval)).Return(nil, errBoom)

		// when
		err := job.Attribute(context.Background(), "Celesta", now)
		err2 := job2.Attribute(context.Background(), "Celesta", now)

		// then
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorIs(t, err2, errBoom)
	})
}

func TestJob_Attribute_NoRunYet(t *testing.T) {
	// given
	job, _, repo := newJob(t)
	repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(time.Time{}, ports.ErrNotFound)

	// when / then
	assert.NoError(t, job.Attribute(context.Background(), "Celesta", now))
}

func TestJob_Attribute_EmptyAuthorStoresNothing(t *testing.T) {
	// given
	job, _, repo := newJob(t)
	repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now.Add(-time.Hour), nil)
	repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).
		Return([]experience.PendingReservation{{ID: 1, Author: " / ", EndAt: now.Add(-time.Minute)}}, nil)
	repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", mock.Anything).Return(now, nil)
	repo.EXPECT().InsertReservationExperience(mock.Anything, []experience.ReservationExperience{}).Return(nil)

	// when / then
	assert.NoError(t, job.Attribute(context.Background(), "Celesta", now))
}

func TestJob_Attribute_RepositoryErrors(t *testing.T) {
	pending := []experience.PendingReservation{{ID: 1, Author: "A", StartAt: now.Add(-2 * time.Hour), EndAt: now.Add(-time.Hour)}}
	tests := []struct {
		name  string
		setup func(repo *mocks.MockExperienceRepository)
	}{
		{name: "first run", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(time.Time{}, errBoom)
		}},
		{name: "pending", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now, nil)
			repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).Return(nil, errBoom)
		}},
		{name: "cut-off", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now, nil)
			repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).Return(pending, nil)
			repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", mock.Anything).Return(time.Time{}, errBoom)
		}},
		{name: "start", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now, nil)
			repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).Return(pending, nil)
			repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", mock.Anything).Return(now, nil)
			repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "a", mock.Anything).Return(nil, errBoom)
		}},
		{name: "fallback start", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now, nil)
			repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).Return(pending, nil)
			repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", mock.Anything).Return(now, nil)
			repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "a", mock.Anything).Return(nil, ports.ErrNotFound)
			repo.EXPECT().FirstSnapshotAfter(mock.Anything, "Celesta", "a", mock.Anything, mock.Anything).Return(nil, errBoom)
		}},
		{name: "end", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now, nil)
			repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).Return(pending, nil)
			repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", mock.Anything).Return(now, nil)
			repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "a", pending[0].StartAt).Return(snap(1, now.Add(-3*time.Hour), now), nil)
			repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "a", now).Return(nil, errBoom)
		}},
		{name: "insert", setup: func(repo *mocks.MockExperienceRepository) {
			repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(now, nil)
			repo.EXPECT().PendingReservations(mock.Anything, "Celesta", mock.Anything, mock.Anything, mock.Anything).Return(pending, nil)
			repo.EXPECT().FirstRunObservedAtOrAfter(mock.Anything, "Celesta", mock.Anything).Return(now, nil)
			repo.EXPECT().SnapshotAtOrBefore(mock.Anything, "Celesta", "a", mock.Anything).Return(snap(1, now.Add(-3*time.Hour), now), nil)
			repo.EXPECT().InsertReservationExperience(mock.Anything, mock.Anything).Return(errBoom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			job, _, repo := newJob(t)
			tt.setup(repo)

			// when
			err := job.Attribute(context.Background(), "Celesta", now)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestJob_RunOnce_ContinuesAfterAFailingWorld(t *testing.T) {
	// given
	job, api, repo := newJob(t)
	repo.EXPECT().ListTrackedWorlds(mock.Anything).Return([]string{"Antica", "Celesta"}, nil)
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Antica", mock.Anything).Return([]string{"a"}, nil)
	api.EXPECT().GetHighscoresPage(mock.Anything, "Antica", 1).Return(nil, errBoom)
	repo.EXPECT().FirstRunObservedAt(mock.Anything, "Antica").Return(time.Time{}, ports.ErrNotFound)
	repo.EXPECT().ListTrackedCharacterKeys(mock.Anything, "Celesta", mock.Anything).Return(nil, nil)
	repo.EXPECT().FirstRunObservedAt(mock.Anything, "Celesta").Return(time.Time{}, errBoom)

	// when
	err := job.RunOnce(context.Background(), now)

	// then
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "collect Antica")
	assert.ErrorContains(t, err, "attribute Celesta")
}

func TestJob_RunOnce_Errors(t *testing.T) {
	t.Run("list worlds", func(t *testing.T) {
		// given
		job, _, repo := newJob(t)
		repo.EXPECT().ListTrackedWorlds(mock.Anything).Return(nil, errBoom)

		// when / then
		assert.ErrorIs(t, job.RunOnce(context.Background(), now), errBoom)
	})
	t.Run("cancelled", func(t *testing.T) {
		// given
		job, _, repo := newJob(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		repo.EXPECT().ListTrackedWorlds(mock.Anything).Return([]string{"Celesta"}, nil)

		// when / then
		assert.ErrorIs(t, job.RunOnce(ctx, now), context.Canceled)
	})
	t.Run("nothing to do", func(t *testing.T) {
		// given
		job, _, repo := newJob(t)
		repo.EXPECT().ListTrackedWorlds(mock.Anything).Return(nil, nil)

		// when / then
		assert.NoError(t, job.RunOnce(context.Background(), now))
	})
}

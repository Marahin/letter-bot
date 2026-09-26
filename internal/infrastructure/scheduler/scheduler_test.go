package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/test/mocks"
)

func TestScheduler_Tick_RunsTheJobUnderTheLock(t *testing.T) {
	// given
	job := mocks.NewMockExperienceJob(t)
	locker := mocks.NewMockJobLocker(t)
	unlocked := false
	locker.EXPECT().TryLock(mock.Anything).Return(func() { unlocked = true }, true, nil)
	job.EXPECT().RunOnce(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ time.Time) error {
		_, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline)
		assert.False(t, unlocked)
		return nil
	})

	// when
	New(job, locker, time.Minute, nil).Tick(context.Background())

	// then
	assert.True(t, unlocked)
}

func TestScheduler_Tick_UnlocksAfterAFailedRun(t *testing.T) {
	// given
	job := mocks.NewMockExperienceJob(t)
	locker := mocks.NewMockJobLocker(t)
	unlocked := false
	locker.EXPECT().TryLock(mock.Anything).Return(func() { unlocked = true }, true, nil)
	job.EXPECT().RunOnce(mock.Anything, mock.Anything).Return(errors.New("boom"))

	// when
	New(job, locker, time.Minute, nil).Tick(context.Background())

	// then
	assert.True(t, unlocked)
}

func TestScheduler_Tick_SkipsWhenLockIsHeldElsewhere(t *testing.T) {
	// given
	job := mocks.NewMockExperienceJob(t)
	locker := mocks.NewMockJobLocker(t)
	locker.EXPECT().TryLock(mock.Anything).Return(nil, false, nil)

	// when
	New(job, locker, time.Minute, nil).Tick(context.Background())

	// then
	job.AssertNotCalled(t, "RunOnce", mock.Anything, mock.Anything)
}

func TestScheduler_Tick_SkipsWhenLockFails(t *testing.T) {
	// given
	job := mocks.NewMockExperienceJob(t)
	locker := mocks.NewMockJobLocker(t)
	locker.EXPECT().TryLock(mock.Anything).Return(nil, false, errors.New("db down"))

	// when
	New(job, locker, time.Minute, nil).Tick(context.Background())

	// then
	job.AssertNotCalled(t, "RunOnce", mock.Anything, mock.Anything)
}

func TestScheduler_Run_RunsImmediatelyAndStopsWithTheContext(t *testing.T) {
	// given
	job := mocks.NewMockExperienceJob(t)
	locker := mocks.NewMockJobLocker(t)
	ctx, cancel := context.WithCancel(context.Background())
	runs := 0
	locker.EXPECT().TryLock(mock.Anything).Return(func() {}, true, nil)
	job.EXPECT().RunOnce(mock.Anything, mock.Anything).RunAndReturn(func(context.Context, time.Time) error {
		runs++
		if runs == 2 {
			cancel()
		}
		return nil
	})
	done := make(chan struct{})

	// when
	go func() {
		New(job, locker, 10*time.Millisecond, nil).Run(ctx)
		close(done)
	}()

	// then
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
	assert.Equal(t, 2, runs)
}

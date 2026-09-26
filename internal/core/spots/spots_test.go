package spots

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

func newService(t *testing.T) (*Service, *mocks.MockSpotRepository, *mocks.MockBotNotifier) {
	repo := mocks.NewMockSpotRepository(t)
	notifier := mocks.NewMockBotNotifier(t)
	return New(repo, notifier, nil), repo, notifier
}

func guildSpots() []*spot.Spot {
	archivedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return []*spot.Spot{
		{ID: 1, Name: "Dragon Lords", GuildID: guildID},
		{ID: 3, Name: "Empty (old)", GuildID: guildID, ArchivedAt: &archivedAt},
		{ID: 4, Name: "Hero Cave", GuildID: guildID},
		{ID: 9, Name: "Dragons Old", GuildID: guildID, ArchivedAt: &archivedAt},
	}
}

func TestService_List_ActiveTabWithCounts(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().SelectGuildSpots(ctx, guildID, true).Return(guildSpots(), nil)
	repo.EXPECT().SelectGuildSpotReservationCounts(ctx, guildID).Return(map[int64]spot.ReservationCounts{4: {Total: 5, Upcoming: 2}}, nil)

	// when
	list, err := s.List(ctx, guildID, spot.ListFilter{})

	// then
	require.NoError(t, err)
	require.Len(t, list.Spots, 2)
	assert.Equal(t, "Dragon Lords", list.Spots[0].Name)
	assert.Zero(t, list.Spots[0].Reservations)
	assert.Equal(t, spot.ReservationCounts{Total: 5, Upcoming: 2}, list.Spots[1].Reservations)
	assert.Equal(t, 2, list.ActiveCount)
	assert.Equal(t, 2, list.ArchivedCount)
	assert.Equal(t, 4, list.Total())
}

func TestService_List_ArchivedTabFiltersByQuery(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().SelectGuildSpots(ctx, guildID, true).Return(guildSpots(), nil)
	repo.EXPECT().SelectGuildSpotReservationCounts(ctx, guildID).Return(map[int64]spot.ReservationCounts{}, nil)

	// when
	list, err := s.List(ctx, guildID, spot.ListFilter{Archived: true, Query: "  DRAGON "})

	// then
	require.NoError(t, err)
	require.Len(t, list.Spots, 1)
	assert.Equal(t, int64(9), list.Spots[0].ID)
	assert.Equal(t, 2, list.ArchivedCount)
}

func TestService_List_Errors(t *testing.T) {
	t.Run("spots", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpots(ctx, guildID, true).Return(nil, errors.New("boom"))

		// when
		_, err := s.List(ctx, guildID, spot.ListFilter{})

		// then
		assert.ErrorContains(t, err, "boom")
	})
	t.Run("counts", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpots(ctx, guildID, true).Return(guildSpots(), nil)
		repo.EXPECT().SelectGuildSpotReservationCounts(ctx, guildID).Return(nil, errors.New("boom"))

		// when
		_, err := s.List(ctx, guildID, spot.ListFilter{})

		// then
		assert.ErrorContains(t, err, "boom")
	})
}

func TestService_Create_TrimsAndNotifies(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().InsertSpot(ctx, guildID, "Hero Cave").Return(&spot.Spot{ID: 12, Name: "Hero Cave"}, nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	created, err := s.Create(ctx, guildID, "  Hero Cave \t")

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(12), created.ID)
}

func TestService_Create_NotifierFailureIsNotAnError(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().InsertSpot(ctx, guildID, "Hero Cave").Return(&spot.Spot{ID: 12}, nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(errors.New("conn closed"))

	// when
	_, err := s.Create(ctx, guildID, "Hero Cave")

	// then
	assert.NoError(t, err)
}

func TestService_Create_Validation(t *testing.T) {
	cases := map[string]struct {
		name string
		want error
	}{
		"empty":    {name: "   ", want: ErrNameEmpty},
		"too long": {name: strings.Repeat("ż", MaxNameLength+1), want: ErrNameTooLong},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			s, _, _ := newService(t)

			// when
			_, err := s.Create(context.Background(), guildID, tc.name)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestService_Create_AcceptsMaxLengthInRunes(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	name := strings.Repeat("ż", MaxNameLength)
	repo.EXPECT().InsertSpot(ctx, guildID, name).Return(&spot.Spot{ID: 1}, nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	_, err := s.Create(ctx, guildID, name)

	// then
	assert.NoError(t, err)
}

func TestService_Create_Duplicate(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().InsertSpot(ctx, guildID, "Hero Cave").Return(nil, fmt.Errorf("%w: unique", ports.ErrDuplicate))

	// when
	_, err := s.Create(ctx, guildID, "Hero Cave")

	// then
	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestService_Rename(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().RenameSpot(ctx, guildID, int64(4), "Hero Cave (north)").Return(nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	err := s.Rename(ctx, guildID, 4, " Hero Cave (north) ")

	// then
	assert.NoError(t, err)
}

func TestService_Rename_Errors(t *testing.T) {
	cases := map[string]struct {
		name    string
		repoErr error
		want    error
	}{
		"empty":     {name: "", want: ErrNameEmpty},
		"duplicate": {name: "Taken", repoErr: ports.ErrDuplicate, want: ErrDuplicateName},
		"not found": {name: "Other", repoErr: ports.ErrNotFound, want: ErrNotFound},
		"db":        {name: "Other", repoErr: errors.New("boom"), want: nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			s, repo, _ := newService(t)
			if tc.name != "" {
				repo.EXPECT().RenameSpot(ctx, guildID, int64(4), tc.name).Return(tc.repoErr)
			}

			// when
			err := s.Rename(ctx, guildID, 4, tc.name)

			// then
			require.Error(t, err)
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			}
		})
	}
}

func TestService_Remove_DeletesSpotWithoutReservations(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(7)).Return(&spot.Spot{ID: 7}, nil)
	repo.EXPECT().DeleteSpot(ctx, guildID, int64(7)).Return(nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	outcome, err := s.Remove(ctx, guildID, 7)

	// then
	require.NoError(t, err)
	assert.Equal(t, spot.RemoveDeleted, outcome)
}

func TestService_Remove_ArchivesSpotWithReservations(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1}, nil)
	repo.EXPECT().DeleteSpot(ctx, guildID, int64(1)).Return(ports.ErrNotFound)
	repo.EXPECT().ArchiveSpot(ctx, guildID, int64(1)).Return(nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	outcome, err := s.Remove(ctx, guildID, 1)

	// then
	require.NoError(t, err)
	assert.Equal(t, spot.RemoveArchived, outcome)
}

func TestService_Remove_AlreadyArchivedWithReservationsIsANoOp(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	archivedAt := time.Now()
	repo.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(3)).Return(&spot.Spot{ID: 3, ArchivedAt: &archivedAt}, nil)
	repo.EXPECT().DeleteSpot(ctx, guildID, int64(3)).Return(ports.ErrNotFound)

	// when
	outcome, err := s.Remove(ctx, guildID, 3)

	// then
	require.NoError(t, err)
	assert.Equal(t, spot.RemoveArchived, outcome)
}

func TestService_Remove_Errors(t *testing.T) {
	t.Run("unknown spot", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(99)).Return(nil, ports.ErrNotFound)

		// when
		_, err := s.Remove(ctx, guildID, 99)

		// then
		assert.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("delete fails", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1}, nil)
		repo.EXPECT().DeleteSpot(ctx, guildID, int64(1)).Return(errors.New("boom"))

		// when
		_, err := s.Remove(ctx, guildID, 1)

		// then
		assert.ErrorContains(t, err, "boom")
	})
	t.Run("archive fails", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpotByID(ctx, guildID, int64(1)).Return(&spot.Spot{ID: 1}, nil)
		repo.EXPECT().DeleteSpot(ctx, guildID, int64(1)).Return(ports.ErrNotFound)
		repo.EXPECT().ArchiveSpot(ctx, guildID, int64(1)).Return(ports.ErrNotFound)

		// when
		_, err := s.Remove(ctx, guildID, 1)

		// then
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestService_Restore(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().RestoreSpot(ctx, guildID, int64(3)).Return(nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	err := s.Restore(ctx, guildID, 3)

	// then
	assert.NoError(t, err)
}

func TestService_Restore_NameClash(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().RestoreSpot(ctx, guildID, int64(3)).Return(fmt.Errorf("%w: unique", ports.ErrDuplicate))

	// when
	err := s.Restore(ctx, guildID, 3)

	// then
	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestService_ImportDefaults(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames).Return(int64(len(DefaultNames)), nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	added, err := s.ImportDefaults(ctx, guildID)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(len(DefaultNames)), added)
}

func TestService_ImportDefaults_NothingAddedSendsNoSignal(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames).Return(int64(0), nil)

	// when
	added, err := s.ImportDefaults(ctx, guildID)

	// then
	require.NoError(t, err)
	assert.Zero(t, added)
}

func TestService_ImportDefaults_Error(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames).Return(int64(0), errors.New("boom"))

	// when
	_, err := s.ImportDefaults(ctx, guildID)

	// then
	assert.ErrorContains(t, err, "boom")
}

func TestDefaultNames_AreValidAndUnique(t *testing.T) {
	// given
	seen := map[string]bool{}

	// when / then
	require.NotEmpty(t, DefaultNames)
	for _, n := range DefaultNames {
		normalized, err := normalizeName(n)
		require.NoError(t, err, n)
		assert.Equal(t, n, normalized)
		key := strings.ToLower(n)
		assert.False(t, seen[key], "duplicate default %q", n)
		seen[key] = true
	}
}

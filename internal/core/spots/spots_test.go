package spots

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/regions"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

func newService(t *testing.T) (*Service, *mocks.MockSpotRepository, *mocks.MockBotNotifier) {
	repo := mocks.NewMockSpotRepository(t)
	notifier := mocks.NewMockBotNotifier(t)
	return New(repo, notifier, nil), repo, notifier
}

func TestService_List_PassesTheTrimmedFilterAndTabCounts(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	listed := []spot.Listed{{Spot: spot.Spot{ID: 9, Name: "Dragons Old"}, Reservations: spot.ReservationCounts{Total: 5, Upcoming: 2}}}
	repo.EXPECT().SelectGuildSpotList(ctx, guildID, spot.ListFilter{Archived: true, Query: "DRAGON"}).Return(listed, nil)
	repo.EXPECT().CountGuildSpots(ctx, guildID).Return(2, 3, nil)

	// when
	list, err := s.List(ctx, guildID, spot.ListFilter{Archived: true, Query: "  DRAGON "})

	// then
	require.NoError(t, err)
	assert.Equal(t, listed, list.Spots)
	assert.Equal(t, 2, list.ActiveCount)
	assert.Equal(t, 3, list.ArchivedCount)
	assert.Equal(t, 5, list.Total())
}

func TestService_List_NoMatchIsAnEmptyList(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().SelectGuildSpotList(ctx, guildID, spot.ListFilter{}).Return(nil, nil)
	repo.EXPECT().CountGuildSpots(ctx, guildID).Return(0, 0, nil)

	// when
	list, err := s.List(ctx, guildID, spot.ListFilter{})

	// then
	require.NoError(t, err)
	assert.NotNil(t, list.Spots)
	assert.Empty(t, list.Spots)
}

func TestService_List_Errors(t *testing.T) {
	t.Run("spots", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpotList(ctx, guildID, spot.ListFilter{}).Return(nil, errors.New("boom"))

		// when
		_, err := s.List(ctx, guildID, spot.ListFilter{})

		// then
		assert.ErrorContains(t, err, "boom")
	})
	t.Run("counts", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, repo, _ := newService(t)
		repo.EXPECT().SelectGuildSpotList(ctx, guildID, spot.ListFilter{}).Return(nil, nil)
		repo.EXPECT().CountGuildSpots(ctx, guildID).Return(0, 0, errors.New("boom"))

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
	repo.EXPECT().InsertSpot(ctx, guildID, spot.Fields{Name: "Hero Cave", Region: "edron", ImageURL: "https://img.test/hc.png"}).
		Return(&spot.Spot{ID: 12, Name: "Hero Cave"}, nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	created, err := s.Create(ctx, guildID, spot.Fields{Name: "  Hero Cave \t", Region: " edron ", ImageURL: " https://img.test/hc.png "})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(12), created.ID)
}

func TestService_Create_NotifierFailureIsNotAnError(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().InsertSpot(ctx, guildID, spot.Fields{Name: "Hero Cave"}).Return(&spot.Spot{ID: 12}, nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(errors.New("conn closed"))

	// when
	_, err := s.Create(ctx, guildID, spot.Fields{Name: "Hero Cave"})

	// then
	assert.NoError(t, err)
}

func TestService_Create_Validation(t *testing.T) {
	cases := map[string]struct {
		fields spot.Fields
		want   error
	}{
		"empty":                {fields: spot.Fields{Name: "   "}, want: ErrNameEmpty},
		"too long":             {fields: spot.Fields{Name: strings.Repeat("ż", MaxNameLength+1)}, want: ErrNameTooLong},
		"unknown region":       {fields: spot.Fields{Name: "Hero Cave", Region: "atlantis"}, want: ErrUnknownRegion},
		"other is not a key":   {fields: spot.Fields{Name: "Hero Cave", Region: "other"}, want: ErrUnknownRegion},
		"image without scheme": {fields: spot.Fields{Name: "Hero Cave", ImageURL: "img.test/a.png"}, want: ErrImageURLInvalid},
		"image script":         {fields: spot.Fields{Name: "Hero Cave", ImageURL: "javascript:alert(1)"}, want: ErrImageURLInvalid},
		"image without host":   {fields: spot.Fields{Name: "Hero Cave", ImageURL: "https:///a.png"}, want: ErrImageURLInvalid},
		"image with space":     {fields: spot.Fields{Name: "Hero Cave", ImageURL: "https://img.test/a b.png"}, want: ErrImageURLInvalid},
		"image too long":       {fields: spot.Fields{Name: "Hero Cave", ImageURL: "https://img.test/" + strings.Repeat("a", MaxImageURLLength)}, want: ErrImageURLTooLong},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			s, _, _ := newService(t)

			// when
			_, err := s.Create(context.Background(), guildID, tc.fields)

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
	image := "http://img.test/" + strings.Repeat("a", MaxImageURLLength-len("http://img.test/"))
	repo.EXPECT().InsertSpot(ctx, guildID, spot.Fields{Name: name, ImageURL: image}).Return(&spot.Spot{ID: 1}, nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	_, err := s.Create(ctx, guildID, spot.Fields{Name: name, ImageURL: image})

	// then
	assert.NoError(t, err)
}

func TestService_Create_Duplicate(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().InsertSpot(ctx, guildID, spot.Fields{Name: "Hero Cave"}).Return(nil, fmt.Errorf("%w: unique", ports.ErrDuplicate))

	// when
	_, err := s.Create(ctx, guildID, spot.Fields{Name: "Hero Cave"})

	// then
	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestService_Update(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, notifier := newService(t)
	repo.EXPECT().UpdateSpot(ctx, guildID, int64(4), spot.Fields{Name: "Hero Cave (north)", Region: "edron"}).Return(nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	err := s.Update(ctx, guildID, 4, spot.Fields{Name: " Hero Cave (north) ", Region: "edron", ImageURL: "  "})

	// then
	assert.NoError(t, err)
}

func TestService_Update_Errors(t *testing.T) {
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
				repo.EXPECT().UpdateSpot(ctx, guildID, int64(4), spot.Fields{Name: tc.name}).Return(tc.repoErr)
			}

			// when
			err := s.Update(ctx, guildID, 4, spot.Fields{Name: tc.name})

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
	var regionsOf []string
	repo.EXPECT().InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames, mock.Anything).
		Run(func(_ context.Context, _ string, _ []string, r []string) { regionsOf = r }).
		Return(int64(len(DefaultNames)), nil)
	notifier.EXPECT().SummaryChanged(ctx, guildID).Return(nil)

	// when
	added, err := s.ImportDefaults(ctx, guildID)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(len(DefaultNames)), added)
	require.Len(t, regionsOf, len(DefaultNames))
	for i, name := range DefaultNames {
		assert.Equal(t, regions.DefaultSpotRegion[name], regionsOf[i], name)
	}
}

func TestService_ImportDefaults_NothingAddedSendsNoSignal(t *testing.T) {
	// given
	ctx := context.Background()
	s, repo, _ := newService(t)
	repo.EXPECT().InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames, mock.Anything).Return(int64(0), nil)

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
	repo.EXPECT().InsertSpotsIgnoreDuplicates(ctx, guildID, DefaultNames, mock.Anything).Return(int64(0), errors.New("boom"))

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

func TestDefaultSpotRegion_PlacesOnlyDefaultNamesInCatalogRegions(t *testing.T) {
	for name, key := range regions.DefaultSpotRegion {
		assert.Contains(t, DefaultNames, name)
		assert.True(t, regions.Valid(key), "%s: %s", name, key)
	}
}

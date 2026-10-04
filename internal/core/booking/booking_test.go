package booking

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"spot-assistant/internal/core/dto/book"
	guild2 "spot-assistant/internal/core/dto/guild"
	member2 "spot-assistant/internal/core/dto/member"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"

	"spot-assistant/internal/common/test/mocks"
)

func TestFindAvailableSpotsWithNoFilter(t *testing.T) {
	// given
	is := assert.New(t)
	mockSpotRepo := mocks.NewMockSpotRepository(t)
	adapter := NewAdapter(mockSpotRepo, mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))
	spots := []*spot.Spot{
		{
			Name: "test-1",
		},
		{
			Name: "test-2",
		},
	}
	mockSpotRepo.On("SelectGuildSpotsLike", context.Background(), "guild-1", "").Return(spots, nil)

	// when
	res, err := adapter.FindAvailableSpots("guild-1", "")

	// assert
	is.Nil(err)
	is.NotNil(res)
	is.NotEmpty(res)
	for _, sp := range spots {
		is.Contains(res, sp.Name)
	}
}

func TestFindAvailableSpotsWithFilter(t *testing.T) {
	// given
	is := assert.New(t)
	mockSpotRepo := mocks.NewMockSpotRepository(t)
	adapter := NewAdapter(mockSpotRepo, mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))
	spots := []*spot.Spot{
		{
			Name: "test-2",
		},
	}
	mockSpotRepo.On("SelectGuildSpotsLike", context.Background(), "guild-1", "2").Return(spots, nil)

	// when
	res, err := adapter.FindAvailableSpots("guild-1", "2")

	// assert
	is.Nil(err)
	is.NotNil(res)
	is.NotEmpty(res)
	is.Len(res, 1)
	is.Equal(res[0], spots[0].Name)
}

func TestFindAvailableSpots_ReturnsEmpty_WhenNoMatch(t *testing.T) {
	// given
	is := assert.New(t)
	mockSpotRepo := mocks.NewMockSpotRepository(t)
	adapter := NewAdapter(mockSpotRepo, mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))
	mockSpotRepo.On("SelectGuildSpotsLike", context.Background(), "guild-1", "nonexistent").Return([]*spot.Spot{}, nil)

	// when
	res, err := adapter.FindAvailableSpots("guild-1", "nonexistent")

	// assert
	is.Nil(err)
	is.NotNil(res)
	is.Empty(res)
}

func TestFindAvailableSpots_PropagatesError_WhenRepoFails(t *testing.T) {
	// given
	is := assert.New(t)
	mockSpotRepo := mocks.NewMockSpotRepository(t)
	adapter := NewAdapter(mockSpotRepo, mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))
	mockSpotRepo.On("SelectGuildSpotsLike", context.Background(), "guild-1", "error").Return(nil, errors.New("db error"))

	// when
	res, err := adapter.FindAvailableSpots("guild-1", "error")

	// assert
	is.NotNil(err)
	is.Contains(err.Error(), "db error")
	is.Empty(res)
}

func TestGetSuggestedHoursWithNoFilter(t *testing.T) {
	// given
	tBase := time.Date(2023, 8, 19, 15, 0, 0, 0, time.Now().Location())
	is := assert.New(t)
	adapter := NewAdapter(mocks.NewMockSpotRepository(t), mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))

	// when
	res := adapter.GetSuggestedHours(tBase, "")

	// assert
	is.NotEmpty(res)
	is.Contains(strings.Join(res, " "), "15:30", "16:00", "16:30")
	for _, stringifiedHour := range res {
		is.Regexp(HourRegex, stringifiedHour)
	}
}

func TestGetSuggestedHoursWithFilter(t *testing.T) {
	// given
	tBase := time.Date(2023, 8, 19, 15, 0, 0, 0, time.Now().Location())
	is := assert.New(t)
	adapter := NewAdapter(mocks.NewMockSpotRepository(t), mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))

	// when
	res := adapter.GetSuggestedHours(tBase, "30")

	// assert
	is.NotEmpty(res)
	is.Contains(strings.Join(res, " "), "15:30", "16:00", "16:30")
	for _, stringifiedHour := range res {
		is.Regexp(regexp.MustCompile(`(\d{2}:\d{2})`), stringifiedHour)
	}
}

func TestGetSuggestedHoursWithFilterWithSpecificHour(t *testing.T) {
	// given
	tBase := time.Date(2023, 8, 19, 15, 0, 0, 0, time.Now().Location())
	is := assert.New(t)
	adapter := NewAdapter(mocks.NewMockSpotRepository(t), mocks.NewMockReservationRepository(t), mocks.NewMockCommunicationService(t))

	// when
	res := adapter.GetSuggestedHours(tBase, "15:20")

	// assert
	is.NotEmpty(res)
	is.Contains(strings.Join(res, " "), "15:20", "15:30", "16:00", "16:30")
	for _, stringifiedHour := range res {
		is.Regexp(regexp.MustCompile(`(\d{2}:\d{2})`), stringifiedHour)
	}
}

func TestUnbook(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	rsv := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              1,
			Author:          "test-nick",
			AuthorDiscordID: "test-member",
			StartAt:         time.Now(),
			EndAt:           time.Now().Add(2 * time.Hour),
			GuildID:         "test-id"},
		Spot: reservation.Spot{},
	}
	reservationService := mocks.NewMockReservationRepository(t)
	reservationService.On(
		"FindReservationWithSpot",
		mocks.ContextMock,
		rsv.Reservation.ID, guild.ID, member.ID).Return(rsv, nil)
	reservationService.On("DeletePresentMemberReservation", mocks.ContextMock, guild, member, rsv.Reservation.ID).Return(nil)
	communicationOperations := mocks.NewMockCommunicationService(t)
	adapter := NewAdapter(mocks.NewMockSpotRepository(t), reservationService, communicationOperations)

	// when
	res, err := adapter.Unbook(guild, member, rsv.Reservation.ID)

	// assert
	is.Nil(err)
	is.NotNil(res)
	is.Equal(rsv, res)
}

func TestUnbookAutocomplete(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	reservations := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				ID:              1,
				Author:          "test-nick",
				AuthorDiscordID: "test-member",
				StartAt:         time.Now(),
				EndAt:           time.Now().Add(2 * time.Hour),
				GuildID:         "test-id",
			},
			Spot: reservation.Spot{},
		}}
	reservationService := mocks.NewMockReservationRepository(t)
	reservationService.On(
		"SelectUpcomingMemberReservationsWithSpots",
		mocks.ContextMock,
		guild, member, int64(0)).Return(reservations, nil)
	adapter := NewAdapter(mocks.NewMockSpotRepository(t), reservationService, mocks.NewMockCommunicationService(t))

	// when
	res, err := adapter.UnbookAutocomplete(guild, member, "")

	// assert
	is.Nil(err)
	is.Equal(reservations, res)
}

func TestUnbookAutocompleteWithFilterMatching(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	reservations := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				ID:              1,
				Author:          "test-nick",
				AuthorDiscordID: "test-member",
				StartAt:         time.Now(),
				EndAt:           time.Now().Add(2 * time.Hour),
				GuildID:         "test-id",
			},
			Spot: reservation.Spot{
				Name: "Prison",
			},
		},
		{
			Reservation: reservation.Reservation{
				ID:              1,
				Author:          "test-nick",
				AuthorDiscordID: "test-member",
				StartAt:         time.Now(),
				EndAt:           time.Now().Add(2 * time.Hour),
				GuildID:         "test-id",
			},
			Spot: reservation.Spot{
				Name: "Library",
			},
		}}
	reservationService := mocks.NewMockReservationRepository(t)
	reservationService.On(
		"SelectUpcomingMemberReservationsWithSpots",
		mocks.ContextMock,
		guild, member, int64(0)).Return(reservations, nil)
	adapter := NewAdapter(mocks.NewMockSpotRepository(t), reservationService, mocks.NewMockCommunicationService(t))

	// when
	res, err := adapter.UnbookAutocomplete(guild, member, "Library")

	// assert
	is.Nil(err)
	is.Len(res, 1)
	is.Equal(reservations[1].Reservation.ID, res[0].Reservation.ID)
}

func TestBook(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	startAt := time.Now().Add(1 * time.Minute)
	endAt := startAt.Add(2 * time.Hour)
	spotInput := &spot.Spot{
		Name:      "test-spot",
		ID:        1,
		CreatedAt: time.Now(),
	}
	spotService := mocks.NewMockSpotRepository(t)
	spotService.On("SelectGuildSpotByName", mocks.ContextMock, guild.ID, spotInput.Name).Return(spotInput, nil)
	reservationService := mocks.NewMockReservationRepository(t)
	reservationService.On("SelectOverlappingReservations", mocks.ContextMock, spotInput.ID, startAt, endAt, guild.ID).Return([]*reservation.Reservation{}, nil)
	reservationService.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, guild, member, int64(0)).Return([]*reservation.ReservationWithSpot{}, nil)
	reservationService.On("CreateAndDeleteConflicting", mocks.ContextMock, member, guild, []*reservation.Reservation{}, spotInput.ID, startAt, endAt).Return([]*reservation.ClippedOrRemovedReservation{}, nil)
	adapter := NewAdapter(spotService, reservationService, mocks.NewMockCommunicationService(t))

	// when
	res, err := adapter.Book(book.BookRequest{
		Member:         member,
		Guild:          guild,
		Spot:           spotInput.Name,
		StartAt:        startAt,
		EndAt:          endAt,
		Overbook:       false,
		HasPermissions: false,
	})

	// assert
	is.Nil(err)
	is.NotNil(res)
}

func TestBookFailOnSpotRepo(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	startAt := time.Now().Add(1 * time.Minute)
	endAt := startAt.Add(2 * time.Hour)
	spotInput := &spot.Spot{
		Name:      "test-spot",
		ID:        1,
		CreatedAt: time.Now(),
	}
	spotService := mocks.NewMockSpotRepository(t)
	spotService.On("SelectGuildSpotByName", mocks.ContextMock, guild.ID, spotInput.Name).Return(nil, errors.New("test-error"))
	reservationService := mocks.NewMockReservationRepository(t)

	adapter := NewAdapter(spotService, reservationService, mocks.NewMockCommunicationService(t))

	// when
	_, err := adapter.Book(book.BookRequest{
		Member:         member,
		Guild:          guild,
		Spot:           spotInput.Name,
		StartAt:        startAt,
		EndAt:          endAt,
		Overbook:       false,
		HasPermissions: false,
	})

	// assert
	is.NotNil(err)
}

func TestBookFailOnUnknownSpot(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	startAt := time.Now().Add(1 * time.Minute)
	endAt := startAt.Add(2 * time.Hour)
	spotService := mocks.NewMockSpotRepository(t)
	spotService.On("SelectGuildSpotByName", mocks.ContextMock, guild.ID, "Library").Return(nil, errors.New("not found"))
	reservationService := mocks.NewMockReservationRepository(t)
	adapter := NewAdapter(spotService, reservationService, mocks.NewMockCommunicationService(t))

	// when
	res, err := adapter.Book(book.BookRequest{
		Member:         member,
		Guild:          guild,
		Spot:           "Library",
		StartAt:        startAt,
		EndAt:          endAt,
		Overbook:       false,
		HasPermissions: false,
	})

	// assert
	is.NotNil(err)
	is.Empty(res)
}

// https://github.com/Marahin/letter-bot/issues/3
func TestBookOnMultizoneCase(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:       "test-member",
		Nick:     "test-nick",
		Username: "test-username",
	}
	spotInput := &spot.Spot{
		Name:      "Prison -3",
		ID:        3,
		CreatedAt: time.Now(),
	}
	timeNow := time.Now()
	currentYear := timeNow.Year()
	currentMonth := timeNow.Month()
	currentDay := timeNow.Day()
	existingReservations := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				Author:          member.Username,
				CreatedAt:       timeNow,
				StartAt:         time.Date(currentYear, currentMonth, currentDay, 16, 0, 0, 0, time.UTC),
				EndAt:           time.Date(currentYear, currentMonth, currentDay, 17, 0, 0, 0, time.UTC),
				SpotID:          2,
				GuildID:         guild.ID,
				AuthorDiscordID: member.ID,
			},
			Spot: reservation.Spot{
				ID:   2,
				Name: "Prison -2",
			},
		},
		{
			Reservation: reservation.Reservation{
				Author:          member.Username,
				CreatedAt:       timeNow,
				StartAt:         time.Date(currentYear, currentMonth, currentDay, 21, 1, 0, 0, time.UTC),
				EndAt:           time.Date(currentYear, currentMonth, currentDay, 22, 44, 0, 0, time.UTC),
				SpotID:          1,
				GuildID:         guild.ID,
				AuthorDiscordID: member.ID,
			},
			Spot: reservation.Spot{
				ID:   1,
				Name: "Brachio",
			},
		},
	}
	startAt := time.Date(currentYear, currentMonth, currentDay, 16, 0, 0, 0, time.UTC)
	endAt := time.Date(currentYear, currentMonth, currentDay, 17, 0, 0, 0, time.UTC)
	spotService := mocks.NewMockSpotRepository(t)
	spotService.On("SelectGuildSpotByName", mocks.ContextMock, guild.ID, spotInput.Name).Return(spotInput, nil)
	reservationService := mocks.NewMockReservationRepository(t)
	reservationService.On("SelectOverlappingReservations", mocks.ContextMock, spotInput.ID, startAt, endAt, guild.ID).Return([]*reservation.Reservation{}, nil)
	reservationService.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, guild, member, int64(0)).Return(existingReservations, nil)
	reservationService.On("CreateAndDeleteConflicting", mocks.ContextMock, member, guild, []*reservation.Reservation{}, spotInput.ID, startAt, endAt).Return([]*reservation.ClippedOrRemovedReservation{}, nil)
	adapter := NewAdapter(spotService, reservationService, mocks.NewMockCommunicationService(t))

	// when
	res, err := adapter.Book(book.BookRequest{Member: member, Guild: guild, Spot: spotInput.Name, StartAt: startAt, EndAt: endAt})

	// assert
	is.Nil(err)
	is.NotNil(res)
}

func TestBookFailOnOverbookAuthorsReservation(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &guild2.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &member2.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	startAt := time.Now()
	endAt := startAt.Add(1 * time.Hour)
	spotInput := &spot.Spot{
		Name:      "test-spot",
		ID:        1,
		CreatedAt: time.Now(),
	}
	conflictingReservations := []*reservation.Reservation{
		{
			Author:          member.Username,
			CreatedAt:       time.Now(),
			StartAt:         time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
			EndAt:           time.Date(1, 1, 1, 17, 0, 0, 0, time.UTC),
			SpotID:          1,
			GuildID:         guild.ID,
			AuthorDiscordID: member.ID,
		},
	}
	spotService := mocks.NewMockSpotRepository(t)
	spotService.On("SelectGuildSpotByName", mocks.ContextMock, guild.ID, spotInput.Name).Return(spotInput, nil)
	reservationService := mocks.NewMockReservationRepository(t)
	reservationService.On("SelectOverlappingReservations", mocks.ContextMock, spotInput.ID, startAt, endAt, guild.ID).Return(conflictingReservations, nil)
	reservationService.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, guild, member, int64(0)).Return([]*reservation.ReservationWithSpot{}, nil)
	adapter := NewAdapter(spotService, reservationService, mocks.NewMockCommunicationService(t))

	// when
	res, err := adapter.Book(book.BookRequest{Member: member, Guild: guild, Spot: spotInput.Name, StartAt: startAt, EndAt: endAt, Overbook: true, HasPermissions: true})

	// assert
	is.NotNil(err)
	is.Empty(res)
}

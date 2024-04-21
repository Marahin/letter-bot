package booking

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"

	"spot-assistant/internal/common/test/mocks"
)

func TestFindAvailableSpotsWithNoFilter(t *testing.T) {
	// given
	assert := assert.New(t)
	mockSpotRepo := new(mocks.MockSpotRepo)
	adapter := NewAdapter(mockSpotRepo, new(mocks.MockReservationRepo))
	spots := []*spot.Spot{
		{
			Name: "test-1",
		},
		{
			Name: "test-2",
		},
	}
	mockSpotRepo.On("SelectAllSpots", context.Background()).Return(spots, nil)

	// when
	res, err := adapter.FindAvailableSpots("")

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.NotEmpty(res)
	for _, spot := range spots {
		assert.Contains(res, spot.Name)
	}
}

func TestFindAvailableSpotsWithFilter(t *testing.T) {
	// given
	assert := assert.New(t)
	mockSpotRepo := new(mocks.MockSpotRepo)
	adapter := NewAdapter(mockSpotRepo, new(mocks.MockReservationRepo))
	spots := []*spot.Spot{
		{
			Name: "test-1",
		},
		{
			Name: "test-2",
		},
	}
	mockSpotRepo.On("SelectAllSpots", context.Background()).Return(spots, nil)

	// when
	res, err := adapter.FindAvailableSpots("2")

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.NotEmpty(res)
	assert.Len(res, 1)
	assert.Equal(res[0], spots[1].Name)
}

func TestGetSuggestedHoursWithNoFilter(t *testing.T) {
	// given
	tBase := time.Date(2023, 8, 19, 15, 0, 0, 0, time.Now().Location())
	assert := assert.New(t)
	adapter := NewAdapter(new(mocks.MockSpotRepo), new(mocks.MockReservationRepo))

	// when
	res := adapter.GetSuggestedHours(tBase, "")

	// assert
	assert.NotEmpty(res)
	assert.Contains(strings.Join(res, " "), "15:30", "16:00", "16:30")
	for _, stringifiedHour := range res {
		assert.Regexp(HourRegex, stringifiedHour)
	}
}

func TestGetSuggestedHoursWithFilter(t *testing.T) {
	// given
	tBase := time.Date(2023, 8, 19, 15, 0, 0, 0, time.Now().Location())
	assert := assert.New(t)
	adapter := NewAdapter(new(mocks.MockSpotRepo), new(mocks.MockReservationRepo))

	// when
	res := adapter.GetSuggestedHours(tBase, "30")

	// assert
	assert.NotEmpty(res)
	assert.Contains(strings.Join(res, " "), "15:30", "16:00", "16:30")
	for _, stringifiedHour := range res {
		assert.Regexp(regexp.MustCompile(`(\d{2}:\d{2})`), stringifiedHour)
	}
}

func TestGetSuggestedHoursWithFilterWithSpecificHour(t *testing.T) {
	// given
	tBase := time.Date(2023, 8, 19, 15, 0, 0, 0, time.Now().Location())
	assert := assert.New(t)
	adapter := NewAdapter(new(mocks.MockSpotRepo), new(mocks.MockReservationRepo))

	// when
	res := adapter.GetSuggestedHours(tBase, "15:20")

	// assert
	assert.NotEmpty(res)
	assert.Contains(strings.Join(res, " "), "15:20", "15:30", "16:00", "16:30")
	for _, stringifiedHour := range res {
		assert.Regexp(regexp.MustCompile(`(\d{2}:\d{2})`), stringifiedHour)
	}
}

func TestUnbook(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	reservation := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:              1,
			Author:          "test-nick",
			AuthorDiscordID: "test-member",
			StartAt:         time.Now(),
			EndAt:           time.Now().Add(2 * time.Hour),
			GuildID:         "test-id"},
		Spot: reservation.Spot{},
	}
	reservationService := new(mocks.MockReservationRepo)
	reservationService.On(
		"FindReservationWithSpot",
		mocks.ContextMock,
		reservation.Reservation.ID, guild.ID, member.ID).Return(reservation, nil)
	reservationService.On("DeletePresentMemberReservation", mocks.ContextMock, guild, member, reservation.Reservation.ID).Return(nil)
	adapter := NewAdapter(new(mocks.MockSpotRepo), reservationService)

	// when
	res, err := adapter.Unbook(guild, member, reservation.Reservation.ID)

	// assert
	assert.Nil(err)
	assert.NotNil(res)
	assert.Equal(reservation, res)
}

func TestUnbookAutocomplete(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
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
	reservationService := new(mocks.MockReservationRepo)
	reservationService.On(
		"SelectUpcomingMemberReservationsWithSpots",
		mocks.ContextMock,
		guild, member).Return(reservations, nil)
	adapter := NewAdapter(new(mocks.MockSpotRepo), reservationService)

	// when
	res, err := adapter.UnbookAutocomplete(guild, member, "")

	// assert
	assert.Nil(err)
	assert.Equal(reservations, res)
}

func TestUnbookAutocompleteWithFilterMatching(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
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
	reservationService := new(mocks.MockReservationRepo)
	reservationService.On(
		"SelectUpcomingMemberReservationsWithSpots",
		mocks.ContextMock,
		guild, member).Return(reservations, nil)
	adapter := NewAdapter(new(mocks.MockSpotRepo), reservationService)

	// when
	res, err := adapter.UnbookAutocomplete(guild, member, "Library")

	// assert
	assert.Nil(err)
	assert.Len(res, 1)
	assert.Equal(reservations[1].Reservation.ID, res[0].Reservation.ID)
}

func TestBook(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
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
	spotService := new(mocks.MockSpotRepo)
	spotService.On("SelectAllSpots", mocks.ContextMock).Return([]*spot.Spot{spotInput}, nil)
	reservationService := new(mocks.MockReservationRepo)
	reservationService.On("SelectOverlappingReservations", mocks.ContextMock, spotInput.Name, startAt, endAt, guild.ID).Return([]*reservation.Reservation{}, nil)
	reservationService.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, guild, member).Return([]*reservation.ReservationWithSpot{}, nil)
	reservationService.On("CreateAndDeleteConflicting", mocks.ContextMock, member, guild, []*reservation.Reservation{}, spotInput.ID, startAt, endAt).Return([]*reservation.ClippedOrRemovedReservation{}, nil)
	adapter := NewAdapter(spotService, reservationService)

	// when
	res, err := adapter.Book(member, guild, spotInput.Name, startAt, endAt, false, false)

	// assert
	assert.Nil(err)
	assert.NotNil(res)
}

func TestBookFailOnSpotRepo(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
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
	spotService := new(mocks.MockSpotRepo)
	spotService.On("SelectAllSpots", mocks.ContextMock).Return([]*spot.Spot{spotInput}, errors.New("test-error"))
	reservationService := new(mocks.MockReservationRepo)

	adapter := NewAdapter(spotService, reservationService)

	// when
	_, err := adapter.Book(member, guild, spotInput.Name, startAt, endAt, false, false)

	// assert
	assert.NotNil(err)
}

func TestBookFailOnUnknownSpot(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
		ID:   "test-member",
		Nick: "test-nick",
	}
	startAt := time.Now().Add(1 * time.Minute)
	endAt := startAt.Add(2 * time.Hour)
	spotOutput := &spot.Spot{
		Name:      "Existing Spot",
		ID:        1,
		CreatedAt: time.Now(),
	}
	spotService := new(mocks.MockSpotRepo)
	spotService.On("SelectAllSpots", mocks.ContextMock).Return([]*spot.Spot{spotOutput}, nil)
	reservationService := new(mocks.MockReservationRepo)
	adapter := NewAdapter(spotService, reservationService)

	// when
	res, err := adapter.Book(member, guild, "Library", startAt, endAt, false, false)

	// assert
	assert.NotNil(err)
	assert.Empty(res)
}

// https://github.com/Marahin/letter-bot/issues/3
func TestBookOnMultizoneCase(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
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
	spotService := new(mocks.MockSpotRepo)
	spotService.On("SelectAllSpots", mocks.ContextMock).Return([]*spot.Spot{spotInput}, nil)
	reservationService := new(mocks.MockReservationRepo)
	reservationService.On("SelectOverlappingReservations", mocks.ContextMock, spotInput.Name, startAt, endAt, guild.ID).Return([]*reservation.Reservation{}, nil)
	reservationService.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, guild, member).Return(existingReservations, nil)
	reservationService.On("CreateAndDeleteConflicting", mocks.ContextMock, member, guild, []*reservation.Reservation{}, spotInput.ID, startAt, endAt).Return([]*reservation.ClippedOrRemovedReservation{}, nil)
	adapter := NewAdapter(spotService, reservationService)

	// when
	res, err := adapter.Book(member, guild, spotInput.Name, startAt, endAt, false, false)

	// assert
	assert.Nil(err)
	assert.NotNil(res)
}

func TestBookFailOnOverbookAuthorsReservation(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discord.Guild{
		ID:   "test-id",
		Name: "test-guild-name",
	}
	member := &discord.Member{
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
	spotService := new(mocks.MockSpotRepo)
	spotService.On("SelectAllSpots", mocks.ContextMock).Return([]*spot.Spot{spotInput}, nil)
	reservationService := new(mocks.MockReservationRepo)
	reservationService.On("SelectOverlappingReservations", mocks.ContextMock, spotInput.Name, startAt, endAt, guild.ID).Return(conflictingReservations, nil)
	reservationService.On("SelectUpcomingMemberReservationsWithSpots", mocks.ContextMock, guild, member).Return([]*reservation.ReservationWithSpot{}, nil)
	adapter := NewAdapter(spotService, reservationService)

	// when
	res, err := adapter.Book(member, guild, spotInput.Name, startAt, endAt, true, true)

	// assert
	assert.NotNil(err)
	assert.Empty(res)
}

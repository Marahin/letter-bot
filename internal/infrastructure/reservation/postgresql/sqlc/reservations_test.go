package sqlc

import (
	"context"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/reservation"
)

func newReservationRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "author", "created_at", "start_at", "end_at",
		"spot_id", "guild_id", "author_discord_id", "notification_sent",
	})
}

func TestCreateAndDeleteConflictingWithNoConflicting(t *testing.T) {
	// given
	assert := assert.New(t)
	testMember := &member.Member{
		ID:       "test-member-id",
		Username: "test-member-username",
		Nick:     "test-member-nick",
	}
	testGuild := &guild.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	spotId := int64(1)
	tNow := time.Now()
	startAt := time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 21, 1, 0, 0, time.UTC)
	endAt := time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 23, 1, 0, 0, time.UTC)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		testMember.Nick, testMember.ID, mocks.NewPgTimestamptzTime(startAt),
		mocks.NewPgTimestamptzTime(endAt), spotId, testGuild.ID,
	).WillReturnRows(newReservationRows().AddRow(
		int64(1), testMember.Nick, time.Now(), startAt, endAt, spotId, testGuild.ID, testMember.ID, false,
	))

	mock.ExpectCommit()
	repository := NewReservationRepository(mock)

	// when
	removed, err := repository.CreateAndDeleteConflicting(context.Background(), testMember, testGuild, make([]*reservation.Reservation, 0), spotId, startAt, endAt)

	// assert
	assert.Nil(err)
	assert.Empty(removed)
	assert.Nil(mock.ExpectationsWereMet())
}

func TestCreateAndDeleteConflictingWithOneConflicting(t *testing.T) {
	// given
	assert := assert.New(t)
	testMember := &member.Member{
		ID:       "test-member-id",
		Username: "test-member-username",
		Nick:     "test-member-nick",
	}
	testMember2 := &member.Member{
		ID:       "test-member-id-2",
		Username: "test-member-username-2",
		Nick:     "test-member-nick-2",
	}
	testGuild := &guild.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	spotId := int64(1)
	tNow := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			ID:              1,
			Author:          testMember2.Nick,
			AuthorDiscordID: testMember2.ID,
			StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 16, 0, 0, 0, time.UTC),
			EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 18, 0, 0, 0, time.UTC),
			SpotID:          spotId,
			GuildID:         testGuild.ID,
		},
	}
	reservationInput := &reservation.Reservation{
		Author:          testMember.Nick,
		AuthorDiscordID: testMember.ID,
		StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 16, 0, 0, 0, time.UTC),
		EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 17, 0, 0, 0, time.UTC),
		SpotID:          spotId,
		GuildID:         testGuild.ID,
	}
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(conflictingReservations[0].ID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		conflictingReservations[0].Author, conflictingReservations[0].AuthorDiscordID,
		mocks.NewPgTimestamptzTime(reservationInput.EndAt.Add(1*time.Minute)),
		mocks.NewPgTimestamptzTime(conflictingReservations[0].EndAt),
		conflictingReservations[0].SpotID, conflictingReservations[0].GuildID,
	).WillReturnRows(newReservationRows().AddRow(
		int64(1), testMember.Nick, time.Now(),
		reservationInput.EndAt.Add(1*time.Minute), conflictingReservations[0].EndAt,
		spotId, testGuild.ID, testMember.ID, false,
	))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		reservationInput.Author, reservationInput.AuthorDiscordID,
		mocks.NewPgTimestamptzTime(reservationInput.StartAt), mocks.NewPgTimestamptzTime(reservationInput.EndAt),
		reservationInput.SpotID, reservationInput.GuildID,
	).WillReturnRows(newReservationRows().AddRow(
		int64(2), testMember.Nick, time.Now(),
		reservationInput.StartAt, reservationInput.EndAt,
		spotId, testGuild.ID, testMember.ID, false,
	))
	mock.ExpectCommit()
	repository := NewReservationRepository(mock)

	// when
	removed, err := repository.CreateAndDeleteConflicting(context.Background(), testMember, testGuild, conflictingReservations, spotId, reservationInput.StartAt, reservationInput.EndAt)

	// assert
	assert.Nil(err)
	assert.NotEmpty(removed)
	assert.Nil(mock.ExpectationsWereMet())
	assert.Equal(conflictingReservations, collections.PoorMansMap(removed, func(r *reservation.ClippedOrRemovedReservation) *reservation.Reservation {
		return r.Original
	}))
}

func TestCreateAndDeleteConflictingWithTwoConflicting(t *testing.T) {
	// given
	assert := assert.New(t)
	testMember := &member.Member{
		ID:       "test-member-id",
		Username: "test-member-username",
		Nick:     "test-member-nick",
	}
	testMember2 := &member.Member{
		ID:       "test-member-id-2",
		Username: "test-member-username-2",
		Nick:     "test-member-nick-2",
	}
	testMember3 := &member.Member{
		ID:       "test-member-id-3",
		Username: "test-member-username-3",
		Nick:     "test-member-nick-3",
	}
	testGuild := &guild.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	spotId := int64(1)
	tNow := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			ID:              1,
			Author:          testMember2.Nick,
			AuthorDiscordID: testMember2.ID,
			StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 16, 0, 0, 0, time.UTC),
			EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 18, 0, 0, 0, time.UTC),
			SpotID:          spotId,
			GuildID:         testGuild.ID,
		},
		{
			ID:              2,
			Author:          testMember3.Nick,
			AuthorDiscordID: testMember3.ID,
			StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 18, 1, 0, 0, time.UTC),
			EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 20, 0, 0, 0, time.UTC),
			SpotID:          spotId,
			GuildID:         testGuild.ID,
		},
	}
	reservationInput := &reservation.Reservation{
		Author:          testMember.Nick,
		AuthorDiscordID: testMember.ID,
		StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 17, 0, 0, 0, time.UTC),
		EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 19, 0, 0, 0, time.UTC),
		SpotID:          spotId,
		GuildID:         testGuild.ID,
	}
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(conflictingReservations[0].ID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		conflictingReservations[0].Author, conflictingReservations[0].AuthorDiscordID,
		mocks.NewPgTimestamptzTime(conflictingReservations[0].StartAt), mocks.NewPgTimestamptzTime(reservationInput.StartAt.Add(-1*time.Minute)),
		conflictingReservations[0].SpotID, conflictingReservations[0].GuildID,
	).WillReturnRows(newReservationRows().AddRow(
		int64(3), testMember2.Nick, time.Now(), conflictingReservations[0].StartAt, reservationInput.StartAt.Add(-1*time.Minute), spotId, testGuild.ID, testMember.ID, false,
	))
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(conflictingReservations[1].ID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		conflictingReservations[1].Author, conflictingReservations[1].AuthorDiscordID,
		mocks.NewPgTimestamptzTime(reservationInput.EndAt.Add(1*time.Minute)), mocks.NewPgTimestamptzTime(conflictingReservations[1].EndAt),
		conflictingReservations[1].SpotID, conflictingReservations[1].GuildID,
	).WillReturnRows(newReservationRows().AddRow(int64(4), testMember3.Nick, time.Now(), reservationInput.EndAt.Add(1*time.Minute), conflictingReservations[1].EndAt, spotId, testGuild.ID, testMember.ID, false))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		reservationInput.Author, reservationInput.AuthorDiscordID,
		mocks.NewPgTimestamptzTime(reservationInput.StartAt), mocks.NewPgTimestamptzTime(reservationInput.EndAt),
		reservationInput.SpotID, reservationInput.GuildID,
	).WillReturnRows(newReservationRows().AddRow(int64(5), testMember.Nick, time.Now(), reservationInput.EndAt.Add(1*time.Minute), conflictingReservations[1].EndAt, spotId, testGuild.ID, testMember.ID, false))
	mock.ExpectCommit()
	repository := NewReservationRepository(mock)

	// when
	removed, err := repository.CreateAndDeleteConflicting(context.Background(), testMember, testGuild, conflictingReservations, spotId, reservationInput.StartAt, reservationInput.EndAt)

	// assert
	assert.Nil(err)
	assert.NotEmpty(removed)
	assert.Nil(mock.ExpectationsWereMet())
	assert.Equal(conflictingReservations, collections.PoorMansMap(removed, func(r *reservation.ClippedOrRemovedReservation) *reservation.Reservation {
		return r.Original
	}))
}

func TestCreateAndDeleteConflictingWithTwoConflictingButSecondOneFromTheSameAuthorAsNewReservation(t *testing.T) {
	// given
	assert := assert.New(t)
	testMember := &member.Member{
		ID:       "test-member-id",
		Username: "test-member-username",
		Nick:     "test-member-nick",
	}
	testMember2 := &member.Member{
		ID:       "test-member-id-2",
		Username: "test-member-username-2",
		Nick:     "test-member-nick-2",
	}
	testGuild := &guild.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	spotId := int64(1)
	tNow := time.Now()
	conflictingReservations := []*reservation.Reservation{
		{
			ID:              1,
			Author:          testMember2.Nick,
			AuthorDiscordID: testMember2.ID,
			StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 16, 0, 0, 0, time.UTC),
			EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 18, 0, 0, 0, time.UTC),
			SpotID:          spotId,
			GuildID:         testGuild.ID,
		},
		{
			ID:              2,
			Author:          testMember.Nick,
			AuthorDiscordID: testMember.ID,
			StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 18, 1, 0, 0, time.UTC),
			EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 20, 0, 0, 0, time.UTC),
			SpotID:          spotId,
			GuildID:         testGuild.ID,
		},
	}
	reservationInput := &reservation.Reservation{
		Author:          testMember.Nick,
		AuthorDiscordID: testMember.ID,
		StartAt:         time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 17, 0, 0, 0, time.UTC),
		EndAt:           time.Date(tNow.Year(), tNow.Month(), tNow.Day(), 19, 0, 0, 0, time.UTC),
		SpotID:          spotId,
		GuildID:         testGuild.ID,
	}
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(conflictingReservations[0].ID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		conflictingReservations[0].Author, conflictingReservations[0].AuthorDiscordID,
		mocks.NewPgTimestamptzTime(conflictingReservations[0].StartAt), mocks.NewPgTimestamptzTime(reservationInput.StartAt.Add(-1*time.Minute)),
		conflictingReservations[0].SpotID, conflictingReservations[0].GuildID,
	).WillReturnRows(newReservationRows().AddRow(
		int64(3), testMember2.Nick, time.Now(), conflictingReservations[0].StartAt, reservationInput.StartAt.Add(-1*time.Minute), spotId, testGuild.ID, testMember.ID, false))
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(conflictingReservations[1].ID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectQuery("INSERT INTO web_reservation").WithArgs(
		reservationInput.Author, reservationInput.AuthorDiscordID,
		mocks.NewPgTimestamptzTime(reservationInput.StartAt), mocks.NewPgTimestamptzTime(reservationInput.EndAt),
		reservationInput.SpotID, reservationInput.GuildID,
	).WillReturnRows(newReservationRows().AddRow(
		int64(4), testMember.Nick, time.Now(), reservationInput.StartAt, reservationInput.EndAt, spotId, testGuild.ID, testMember.ID, false,
	))
	mock.ExpectCommit()
	repository := NewReservationRepository(mock)

	// when
	removed, err := repository.CreateAndDeleteConflicting(context.Background(), testMember, testGuild, conflictingReservations, spotId, reservationInput.StartAt, reservationInput.EndAt)

	// assert
	assert.Nil(err)
	assert.NotEmpty(removed)
	assert.Nil(mock.ExpectationsWereMet())
	assert.Equal(conflictingReservations, collections.PoorMansMap(removed, func(r *reservation.ClippedOrRemovedReservation) *reservation.Reservation {
		return r.Original
	}))
}

func newReservationWithSpotRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		// web_spot
		"id", "name", "created_at",
		// web_reservation
		"id", "author", "created_at", "start_at", "end_at", "spot_id", "guild_id", "author_discord_id", "notification_sent",
	})
}

func TestSelectUpcomingReservationsWithSpotForSpot_FiltersBySpotAndGuild(t *testing.T) {
	assert := assert.New(t)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Expect the query with guildID and spot name (lower)
	mock.ExpectQuery("select web_spot.id, web_spot.name").
		WithArgs("guild-1", "Flimsy").
		WillReturnRows(newReservationWithSpotRows().
			AddRow(
				int64(10), "Flimsy", time.Now(),
				int64(101), "Mariysz", time.Now(), time.Now(), time.Now().Add(time.Hour), int64(10), "guild-1", "mariysz#1", false,
			).
			AddRow(
				int64(10), "Flimsy", time.Now(),
				int64(102), "Asar", time.Now(), time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), int64(10), "guild-1", "asar#1", false,
			))

	repo := NewReservationRepository(mock)

	res, err := repo.SelectUpcomingReservationsWithSpotForSpot(context.Background(), "guild-1", "Flimsy")
	assert.NoError(err)
	assert.Len(res, 2)
	for _, r := range res {
		assert.Equal("Flimsy", r.Spot.Name)
		assert.Equal("guild-1", r.GuildID)
	}
	assert.NoError(mock.ExpectationsWereMet())
}

func TestSelectUpcomingReservationsWithSpotForSpot_EmptyWhenNoMatches(t *testing.T) {
	assert := assert.New(t)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("select web_spot.id, web_spot.name").
		WithArgs("guild-1", "Unknown").
		WillReturnRows(newReservationWithSpotRows())

	repo := NewReservationRepository(mock)
	res, err := repo.SelectUpcomingReservationsWithSpotForSpot(context.Background(), "guild-1", "Unknown")
	assert.NoError(err)
	assert.Len(res, 0)
	assert.NoError(mock.ExpectationsWereMet())
}

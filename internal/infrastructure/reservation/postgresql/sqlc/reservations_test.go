package sqlc

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

func TestCreateAndDeleteConflictingWithNoConflicting(t *testing.T) {
	// given
	assert := assert.New(t)
	testMember := &discord.Member{
		ID:       "test-member-id",
		Username: "test-member-username",
		Nick:     "test-member-nick",
	}
	testGuild := &discord.Guild{
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
	mock.ExpectExec("INSERT INTO web_reservation").WithArgs(testMember.Nick, testMember.ID, mocks.PgTimestamptzTime{startAt}, mocks.PgTimestamptzTime{endAt}, spotId, testGuild.ID).WillReturnResult(pgxmock.NewResult("INSERT", 1))
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
	testMember := &discord.Member{
		ID:       "test-member-id",
		Username: "test-member-username",
		Nick:     "test-member-nick",
	}
	testMember2 := &discord.Member{
		ID:       "test-member-id-2",
		Username: "test-member-username-2",
		Nick:     "test-member-nick-2",
	}
	testGuild := &discord.Guild{
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
	mock.ExpectExec("INSERT INTO web_reservation").WithArgs(
		conflictingReservations[0].Author, conflictingReservations[0].AuthorDiscordID,
		mocks.PgTimestamptzTime{reservationInput.EndAt.Add(1 * time.Minute)}, mocks.PgTimestamptzTime{conflictingReservations[0].EndAt},
		conflictingReservations[0].SpotID, conflictingReservations[0].GuildID,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO web_reservation").WithArgs(
		reservationInput.Author, reservationInput.AuthorDiscordID,
		mocks.PgTimestamptzTime{reservationInput.StartAt}, mocks.PgTimestamptzTime{reservationInput.EndAt},
		reservationInput.SpotID, reservationInput.GuildID,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	repository := NewReservationRepository(mock)

	// when
	removed, err := repository.CreateAndDeleteConflicting(context.Background(), testMember, testGuild, conflictingReservations, spotId, reservationInput.StartAt, reservationInput.EndAt)

	// assert
	assert.Nil(err)
	assert.NotEmpty(removed)
	assert.Nil(mock.ExpectationsWereMet())
	assert.Equal(conflictingReservations, removed)
}
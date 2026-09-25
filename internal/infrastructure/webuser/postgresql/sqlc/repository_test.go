package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

func newMock(t *testing.T) pgxmock.PgxPoolIface {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock
}

func newUserRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"discord_user_id", "username", "global_name", "avatar", "created_at", "updated_at"})
}

func TestWebUserRepository_Upsert(t *testing.T) {
	// given
	mock := newMock(t)
	now := time.Now()
	mock.ExpectQuery("INSERT INTO web_users").
		WithArgs("u1", "nyx", "Quiet Nyx", "avatar").
		WillReturnRows(newUserRows().AddRow("u1", "nyx", "Quiet Nyx", "avatar", now, now))
	mock.ExpectQuery("INSERT INTO web_users").
		WithArgs("u1", "nyx", "", "").
		WillReturnError(errors.New("boom"))
	repo := NewWebUserRepository(mock)

	// when
	user, err := repo.Upsert(context.Background(), webuser.User{DiscordUserID: "u1", Username: "nyx", GlobalName: "Quiet Nyx", Avatar: "avatar"})
	failed, failedErr := repo.Upsert(context.Background(), webuser.User{DiscordUserID: "u1", Username: "nyx"})

	// then
	require.NoError(t, err)
	assert.Equal(t, "Quiet Nyx", user.DisplayName())
	assert.True(t, now.Equal(user.CreatedAt))
	assert.Nil(t, failed)
	assert.Error(t, failedErr)
}

func TestWebUserRepository_Get(t *testing.T) {
	// given
	mock := newMock(t)
	now := time.Now()
	mock.ExpectQuery("FROM web_users").WithArgs("u1").
		WillReturnRows(newUserRows().AddRow("u1", "nyx", "", "", now, now))
	mock.ExpectQuery("FROM web_users").WithArgs("u2").WillReturnError(pgx.ErrNoRows)
	repo := NewWebUserRepository(mock)

	// when
	user, err := repo.Get(context.Background(), "u1")
	missing, missingErr := repo.Get(context.Background(), "u2")

	// then
	require.NoError(t, err)
	assert.Equal(t, "nyx", user.DisplayName())
	assert.Nil(t, missing)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
}

func TestWebUserRepository_SaveToken(t *testing.T) {
	// given
	mock := newMock(t)
	expiry := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE web_users").
		WithArgs("access", "refresh", mocks.NewPgTimestamptzTime(expiry), "u1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("UPDATE web_users").
		WithArgs("access", "", pgtype.Timestamptz{}, "u2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	repo := NewWebUserRepository(mock)

	// when
	err := repo.SaveToken(context.Background(), "u1", webuser.Token{AccessToken: "access", RefreshToken: "refresh", Expiry: &expiry})
	missingErr := repo.SaveToken(context.Background(), "u2", webuser.Token{AccessToken: "access"})

	// then
	assert.NoError(t, err)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
}

func TestWebUserRepository_AccessToken(t *testing.T) {
	// given
	mock := newMock(t)
	expiry := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rows := func() *pgxmock.Rows {
		return pgxmock.NewRows([]string{"access_token", "refresh_token", "token_expiry"})
	}
	mock.ExpectQuery("SELECT access_token").WithArgs("u1").WillReturnRows(rows().AddRow("access", "refresh", expiry))
	mock.ExpectQuery("SELECT access_token").WithArgs("u2").WillReturnRows(rows().AddRow("access", "", nil))
	mock.ExpectQuery("SELECT access_token").WithArgs("u3").WillReturnError(pgx.ErrNoRows)
	repo := NewWebUserRepository(mock)

	// when
	withExpiry, err := repo.AccessToken(context.Background(), "u1")
	withoutExpiry, err2 := repo.AccessToken(context.Background(), "u2")
	missing, missingErr := repo.AccessToken(context.Background(), "u3")

	// then
	require.NoError(t, err)
	assert.Equal(t, "access", withExpiry.AccessToken)
	assert.Equal(t, "refresh", withExpiry.RefreshToken)
	assert.True(t, expiry.Equal(*withExpiry.Expiry))
	require.NoError(t, err2)
	assert.Nil(t, withoutExpiry.Expiry)
	assert.Nil(t, missing)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
}

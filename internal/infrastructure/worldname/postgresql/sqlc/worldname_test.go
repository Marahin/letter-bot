package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"spot-assistant/internal/core/dto/guildsworld"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
)

func TestUpsertGuildWorld(t *testing.T) {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	repo := NewWorldNameRepository(mock)

	guildID := "guild123"
	worldName := "Celesta"

	mock.ExpectExec("INSERT INTO guilds_world").
		WithArgs(guildID, worldName).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.UpsertGuildWorld(context.Background(), guildID, worldName)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectGuildWorld(t *testing.T) {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	repo := NewWorldNameRepository(mock)

	guildID := "guild123"
	expected := &guildsworld.GuildsWorld{
		ID:        1,
		GuildID:   guildID,
		WorldName: "Celesta",
		CreatedAt: time.Now(),
	}

	rows := pgxmock.NewRows([]string{"id", "guild_id", "world_name", "created_at"}).
		AddRow(expected.ID, expected.GuildID, expected.WorldName, expected.CreatedAt)

	mock.ExpectQuery("SELECT id, guild_id, world_name, created_at FROM guilds_world").
		WithArgs(guildID).
		WillReturnRows(rows)

	got, err := repo.SelectGuildWorld(context.Background(), guildID)
	assert.NoError(t, err)
	assert.Equal(t, expected.ID, got.ID)
	assert.Equal(t, expected.GuildID, got.GuildID)
	assert.Equal(t, expected.WorldName, got.WorldName)
	assert.WithinDuration(t, expected.CreatedAt, got.CreatedAt, time.Second)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectGuildWorld_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	repo := NewWorldNameRepository(mock)

	guildID := "guild123"
	mock.ExpectQuery("SELECT id, guild_id, world_name, created_at FROM guilds_world").
		WithArgs(guildID).
		WillReturnError(errors.New("no rows in result set"))

	got, err := repo.SelectGuildWorld(context.Background(), guildID)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

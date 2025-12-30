package sqlc

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
)

func newSpotRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "name", "created_at"})
}

func TestSelectSpotsByNameCaseInsensitiveLike_EmptyFilter(t *testing.T) {
	// given
	assert := assert.New(t)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id, name, created_at FROM web_spot").
		WithArgs("").
		WillReturnRows(newSpotRows())

	repository := NewSpotRepository(mock)

	// when
	spots, err := repository.SelectSpotsByNameCaseInsensitiveLike(context.Background(), "")

	// then
	assert.NoError(err)
	assert.Empty(spots)
	assert.NoError(mock.ExpectationsWereMet())
}

func TestSelectSpotsByNameCaseInsensitiveLike_NormalInput(t *testing.T) {
	// given
	assert := assert.New(t)
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id, name, created_at FROM web_spot").
		WithArgs("dragon").
		WillReturnRows(newSpotRows().AddRow(int64(1), "Dragon Lords", nil))

	repository := NewSpotRepository(mock)

	// when
	spots, err := repository.SelectSpotsByNameCaseInsensitiveLike(context.Background(), "dragon")

	// then
	assert.NoError(err)
	assert.Len(spots, 1)
	assert.Equal("Dragon Lords", spots[0].Name)
	assert.NoError(mock.ExpectationsWereMet())
}

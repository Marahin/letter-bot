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

func TestSelectSpotsByNameCaseInsensitiveLike_SQLInjectionAttempts(t *testing.T) {
	injectionPayloads := []struct {
		name    string
		payload string
	}{
		{
			name:    "basic SQL injection with quote",
			payload: "'; DROP TABLE web_spot; --",
		},
		{
			name:    "SQL injection with OR true",
			payload: "' OR '1'='1",
		},
		{
			name:    "SQL injection with UNION",
			payload: "' UNION SELECT * FROM users --",
		},
		{
			name:    "SQL injection with comment",
			payload: "test'--",
		},
		{
			name:    "SQL injection with semicolon",
			payload: "test'; DELETE FROM web_spot WHERE '1'='1",
		},
		{
			name:    "Double quote injection",
			payload: `"; DROP TABLE web_spot; --`,
		},
		{
			name:    "Null byte injection",
			payload: "test\x00'; DROP TABLE --",
		},
		{
			name:    "LIKE wildcard abuse - percent",
			payload: "%",
		},
		{
			name:    "LIKE wildcard abuse - underscore",
			payload: "_",
		},
		{
			name:    "Combined wildcards",
			payload: "%%_%_",
		},
	}

	for _, tc := range injectionPayloads {
		t.Run(tc.name, func(t *testing.T) {
			// given
			assert := assert.New(t)
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()

			// The key assertion: the malicious payload is passed as a parameter value,
			// NOT interpolated into the query. pgxmock expects the exact parameter value.
			mock.ExpectQuery("SELECT id, name, created_at FROM web_spot").
				WithArgs(tc.payload).
				WillReturnRows(newSpotRows())

			repository := NewSpotRepository(mock)

			// when
			spots, err := repository.SelectSpotsByNameCaseInsensitiveLike(context.Background(), tc.payload)

			// then
			assert.NoError(err)
			assert.Empty(spots)
			assert.NoError(mock.ExpectationsWereMet(),
				"Query should use parameterized query with payload as literal value")
		})
	}
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

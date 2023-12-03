package mocks

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// PgTimestamptzTime allows for easy time matching in pgxmock
// https://github.com/pashagolub/pgxmock#matching-arguments-like-timetime
// Usage: `PgTimestamptzTime{startAt}`
type PgTimestamptzTime struct {
	T time.Time
}

func (a PgTimestamptzTime) Match(v interface{}) bool {
	actualPgTime, ok := v.(pgtype.Timestamptz)
	if !ok {
		return false
	}

	return actualPgTime.Time.Equal(a.T)
}

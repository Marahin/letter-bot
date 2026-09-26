package spot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpot_IsArchived(t *testing.T) {
	// given
	now := time.Now()
	active := Spot{Name: "Hero Cave"}
	archived := Spot{Name: "empty", ArchivedAt: &now}

	// when / then
	assert.False(t, active.IsArchived())
	assert.True(t, archived.IsArchived())
}

func TestList_Total(t *testing.T) {
	// given
	l := List{ActiveCount: 3, ArchivedCount: 2}

	// when / then
	assert.Equal(t, 5, l.Total())
}

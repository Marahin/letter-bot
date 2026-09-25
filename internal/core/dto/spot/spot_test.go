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

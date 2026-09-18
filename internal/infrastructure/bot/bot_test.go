package bot

import (
	"errors"
	"testing"
	"time"

	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
)

func TestConnectManager_RetriesThenSucceeds(t *testing.T) {
	// given
	calls := 0
	want := &shards.Manager{}
	create := func(token string) (*shards.Manager, error) {
		calls++
		if calls <= 2 {
			return nil, errors.New("connection reset by peer")
		}
		return want, nil
	}

	// when
	mgr, err := connectManager(create, "Bot x", 5, func(int) time.Duration { return 0 })

	// then
	assert.NoError(t, err)
	assert.Same(t, want, mgr)
	assert.Equal(t, 3, calls)
}

func TestConnectManager_ExhaustsAttempts(t *testing.T) {
	// given
	calls := 0
	lastErr := errors.New("still failing")
	create := func(token string) (*shards.Manager, error) {
		calls++
		return nil, lastErr
	}

	// when
	mgr, err := connectManager(create, "Bot x", 4, func(int) time.Duration { return 0 })

	// then
	assert.ErrorIs(t, err, lastErr)
	assert.Nil(t, mgr)
	assert.Equal(t, 4, calls)
}

func TestShardBackoff_GrowsAndCaps(t *testing.T) {
	// given / when / then
	assert.Equal(t, shardBackoffBase, shardBackoff(0))
	assert.Equal(t, 2*shardBackoffBase, shardBackoff(1))
	assert.Equal(t, shardBackoffCap, shardBackoff(10))
}

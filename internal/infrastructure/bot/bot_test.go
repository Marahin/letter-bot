package bot

import (
	"errors"
	"testing"
	"time"

	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLoadConfig_FallsBackToUnprefixed(t *testing.T) {
	// given
	t.Setenv("BOT_TOKEN", "secret")
	t.Setenv("WEB_BASE_URL", "https://letter.example.test")
	t.Setenv("METRICS_ADDR", ":9100")

	// when
	cfg, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Equal(t, Config{Token: "secret", WebBaseURL: "https://letter.example.test", MetricsAddr: ":9100"}, cfg)
}

func TestLoadConfig_PrefersThePrefixedName(t *testing.T) {
	// given
	t.Setenv("BOT_TOKEN", "secret")
	t.Setenv("METRICS_ADDR", ":9100")
	t.Setenv("BOT_METRICS_ADDR", ":9200")

	// when
	cfg, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Equal(t, ":9200", cfg.MetricsAddr)
}

func TestLoadConfig_RequiresTheToken(t *testing.T) {
	// given
	t.Setenv("BOT_TOKEN", "")

	// when
	_, err := LoadConfig()

	// then
	assert.ErrorContains(t, err, "BOT_TOKEN")
}

func TestConnectShards_WrapsTheLastError(t *testing.T) {
	// given
	boom := errors.New("gateway down")
	tokens := []string{}
	original := newShardManager
	newShardManager = func(token string) (*shards.Manager, error) {
		tokens = append(tokens, token)
		return nil, boom
	}
	t.Cleanup(func() { newShardManager = original })

	// when
	_, err := connectShardsWith("x", func(int) time.Duration { return 0 })

	// then
	assert.ErrorIs(t, err, boom)
	assert.ErrorContains(t, err, "could not create shards manager")
	assert.Len(t, tokens, shardConnectAttempts)
	assert.Equal(t, "Bot x", tokens[0])
}

func TestConnectShards_ReturnsTheManager(t *testing.T) {
	// given
	want := &shards.Manager{}
	original := newShardManager
	newShardManager = func(string) (*shards.Manager, error) { return want, nil }
	t.Cleanup(func() { newShardManager = original })

	// when
	mgr, err := ConnectShards("x")

	// then
	require.NoError(t, err)
	assert.Same(t, want, mgr)
}

func TestShutdown_IsIdempotent(t *testing.T) {
	// given
	b := NewManager(&shards.Manager{}, "", nil, nil, nil)

	// when
	first := b.Shutdown()
	second := b.Shutdown()

	// then
	assert.NoError(t, first)
	assert.NoError(t, second)
	assert.False(t, b.IsRunning())
}

package bot

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
)

func TestGatewayAckStale(t *testing.T) {
	now := time.Now()
	threshold := heartbeatStaleAfter

	tests := []struct {
		name string
		ack  time.Time
		want bool
	}{
		{"zero ack is healthy", time.Time{}, false},
		{"fresh ack", now.Add(-time.Second), false},
		{"exactly threshold", now.Add(-threshold), false},
		{"past threshold", now.Add(-threshold - time.Second), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := gatewayAckStale(tt.ack, now, threshold)
			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func newSessionWithAck(t *testing.T, ack time.Time) *discordgo.Session {
	t.Helper()
	s, err := discordgo.New("Bot x")
	assert.NoError(t, err)
	s.LastHeartbeatAck = ack
	return s
}

func botWithSessions(sessions ...*discordgo.Session) *Bot {
	mgr := &shards.Manager{}
	for _, s := range sessions {
		mgr.Shards = append(mgr.Shards, &shards.Shard{Session: s})
	}
	return &Bot{mgr: mgr}
}

func TestGatewayHealthy_NilManager(t *testing.T) {
	// given
	b := &Bot{}
	// when / then
	assert.NoError(t, b.GatewayHealthy())
}

func TestGatewayHealthy_EmptyShards(t *testing.T) {
	// given
	b := botWithSessions()
	// when / then
	assert.NoError(t, b.GatewayHealthy())
}

func TestGatewayHealthy_HealthyShards(t *testing.T) {
	// given
	now := time.Now()
	b := botWithSessions(
		newSessionWithAck(t, now),
		newSessionWithAck(t, time.Time{}),
	)
	// when / then
	assert.NoError(t, b.GatewayHealthy())
}

func TestGatewayHealthy_StaleShard(t *testing.T) {
	// given
	stale := time.Now().Add(-heartbeatStaleAfter - time.Minute)
	b := botWithSessions(
		newSessionWithAck(t, time.Now()),
		newSessionWithAck(t, stale),
	)
	// when
	err := b.GatewayHealthy()
	// then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stale")
}

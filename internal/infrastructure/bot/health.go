package bot

import (
	"fmt"
	"time"
)

// heartbeatStaleAfter is three missed gateway heartbeat intervals (~41s each).
const heartbeatStaleAfter = 3 * 41 * time.Second

// gatewayAckStale reports whether a heartbeat ack is older than threshold.
// A zero ack means the session has not heartbeated yet, so treat it as healthy.
func gatewayAckStale(ack, now time.Time, threshold time.Duration) bool {
	if ack.IsZero() {
		return false
	}
	return now.Sub(ack) > threshold
}

// GatewayHealthy fails when any shard's gateway heartbeat ack is stale.
// A zombie websocket keeps the process and tick alive, so only the ack age
// reveals it; the failing liveness probe lets Kubernetes restart the pod.
func (b *Bot) GatewayHealthy() error {
	if b.mgr == nil {
		return nil
	}
	now := time.Now()

	b.mgr.RLock()
	defer b.mgr.RUnlock()
	for _, shard := range b.mgr.Shards {
		if shard == nil || shard.Session == nil {
			continue
		}
		// Read only LastHeartbeatAck: discordgo writes LastHeartbeatSent under a different lock.
		shard.Session.RLock()
		ack := shard.Session.LastHeartbeatAck
		shard.Session.RUnlock()

		if gatewayAckStale(ack, now, heartbeatStaleAfter) {
			return fmt.Errorf("gateway heartbeat ack is %s stale", now.Sub(ack).Round(time.Second))
		}
	}
	return nil
}

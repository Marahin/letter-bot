package onlinecheck

import (
	"strings"
	"testing"

	cmap "github.com/orcaman/concurrent-map/v2"
	"go.uber.org/zap"
)

func BenchmarkIsOnline(b *testing.B) {
	log := zap.NewNop().Sugar()
	adapter := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[map[string]struct{}](),
		log:            log,
	}
	adapter.guildIdToWorld.Set("guild1", "Celesta")

	// Populate with 1000 players
	players := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		players["PlayerNum"+strings.Repeat("b", i%10)] = struct{}{}
	}
	players["Mariysz"] = struct{}{}
	adapter.players.Set("Celesta", players)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		adapter.IsOnline("guild1", "Mariysz")
		adapter.IsOnline("guild1", "Mariysz / Another")
		adapter.IsOnline("guild1", "OfflinePlayer")
	}
}

func BenchmarkIsOnline_MultiName(b *testing.B) {
	log := zap.NewNop().Sugar()
	adapter := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[map[string]struct{}](),
		log:            log,
	}
	adapter.guildIdToWorld.Set("guild1", "Celesta")
	players := map[string]struct{}{
		"Mariysz": {},
	}
	adapter.players.Set("Celesta", players)

	b.ResetTimer()
	b.ReportAllocs()

	name := "One / Two / Three / Four / Five / Six"
	for i := 0; i < b.N; i++ {
		adapter.IsOnline("guild1", name)
	}
}

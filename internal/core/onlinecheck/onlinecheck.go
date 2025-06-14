package onlinecheck

import (
	"fmt"
	"spot-assistant/internal/core/dto/summary"
	"strings"
)

func (a *Adapter) RefreshOnlinePlayers(guildID string) error {
	a.mutex.RLock()
	world, ok := a.worlds[guildID]
	a.mutex.RUnlock()
	fmt.Println("Refreshing online players for guild:", guildID, "world:", world)
	if !ok || world == "" {
		return nil
	}
	players, err := a.api.GetOnlinePlayerNames(world)
	if err != nil {
		return err
	}
	a.log.Infof("API call for '%s' (guild %s)", world, guildID)
	a.mutex.Lock()
	a.players[world] = players
	a.mutex.Unlock()
	return nil
}

func (a *Adapter) IsOnline(guildID, characterName string) bool {
	a.mutex.RLock()
	world := a.worlds[guildID]
	players := a.players[world]
	a.mutex.RUnlock()

	names := strings.Split(characterName, "/")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	for _, candidate := range names {
		candidate := strings.ToLower(candidate)
		for _, name := range players {
			if strings.ToLower(name) == candidate {
				return true
			}
		}
	}
	return false
}

func (a *Adapter) PlayerStatus(guildID, characterName string) summary.OnlineStatus {
	if a.IsOnline(guildID, characterName) {
		return summary.Online
	}
	return summary.Offline
}

func (a *Adapter) TryRefresh(guildID string) {
	if err := a.RefreshOnlinePlayers(guildID); err != nil {
		a.log.Errorf("TryRefresh failed for guild %s: %v", guildID, err)
	}
}

func (a *Adapter) ConfigureWorldName(guildID, world string) {
	a.mutex.Lock()
	a.worlds[guildID] = world
	a.mutex.Unlock()
}

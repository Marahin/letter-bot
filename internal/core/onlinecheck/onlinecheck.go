package onlinecheck

import (
	"strings"
)

func (a *Adapter) RefreshOnlinePlayers() error {
	players, err := a.api.GetOnlinePlayerNames(a.world)
	if err != nil {
		return err
	}
	a.log.Infof("API call for '%s'", a.world)
	a.mutex.Lock()
	a.players = players
	a.mutex.Unlock()
	return nil
}

func (a *Adapter) IsOnline(characterName string) bool {
	a.mutex.RLock()
	players := a.players
	a.mutex.RUnlock()

	// Split by slash and trim spaces - this allows for names like "Mariysz/Asar Cham" to be checked individually
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

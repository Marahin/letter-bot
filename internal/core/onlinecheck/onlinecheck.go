package onlinecheck

import (
	"context"
	"fmt"
	"spot-assistant/internal/core/dto/summary"
	"strings"
)

func (a *Adapter) RefreshOnlinePlayers(guildID string) error {
	if !a.IsConfigured() {
		return nil
	}
	world, ok := a.guildIdToWorld.Get(guildID)
	if !ok || world == "" {
		return nil
	}
	players, err := a.api.GetOnlinePlayerNames(world)
	if err != nil {
		return err
	}
	a.log.Infof("API call for '%s' (guild %s)", world, guildID)

	playersMap := make(map[string]struct{}, len(players))
	for _, p := range players {
		playersMap[p] = struct{}{}
	}
	a.players.Set(world, playersMap)
	return nil
}

func (a *Adapter) IsOnline(guildID, characterName string) bool {
	world, worldOk := a.guildIdToWorld.Get(guildID)
	players, playersOk := a.players.Get(world)

	if !worldOk || world == "" || !playersOk {
		return false
	}

	names := strings.Split(characterName, "/")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	for _, candidate := range names {
		if _, ok := players[candidate]; ok {
			return true
		}
	}
	return false
}

func (a *Adapter) PlayerStatus(guildID, characterName string) summary.OnlineStatus {
	world, worldOk := a.guildIdToWorld.Get(guildID)
	if !a.IsConfigured() || !worldOk || world == "" {
		return summary.Unknown
	}
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
	a.guildIdToWorld.Set(guildID, world)
}

func (a *Adapter) SetGuildWorld(guildID, world string) error {
	if a.worldNameRepo == nil {
		return fmt.Errorf("worldNameRepo is not configured")
	}
	if err := a.worldNameRepo.UpsertGuildWorld(context.Background(), guildID, world); err != nil {
		return err
	}
	a.ConfigureWorldName(guildID, world)
	return nil
}

func (a *Adapter) ConfigureWorldNameForGuild(guildID string) error {
	if a.worldNameRepo == nil {
		return fmt.Errorf("worldNameRepo is not configured")
	}
	guildWorld, err := a.worldNameRepo.SelectGuildWorld(context.Background(), guildID)
	if err != nil {
		return err
	}
	if guildWorld != nil && guildWorld.WorldName != "" {
		a.ConfigureWorldName(guildID, guildWorld.WorldName)
	}
	return nil
}

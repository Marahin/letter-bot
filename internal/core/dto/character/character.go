package character

import "time"

// Character is the TibiaData profile of one Tibia character.
type Character struct {
	Name              string
	Level             int
	Vocation          string
	World             string
	Sex               string
	AccountStatus     string
	GuildName         string
	GuildRank         string
	LastLogin         *time.Time
	Residence         string
	AchievementPoints int
}

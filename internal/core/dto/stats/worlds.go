package stats

import "time"

// World is a Tibia world set by at least one public guild. Key is lower(trim(name)).
type World struct {
	Key  string
	Name string
}

// WorldFilter selects the reservations of every public guild (premium, bot present, world set)
// that start in [From, To). Empty keys mean every world and every spawn.
type WorldFilter struct {
	From     time.Time
	To       time.Time
	WorldKey string
	SpawnKey string
}

// WorldQuery orders the spawns of WorldFilter like Query; the Min fields drop rows with less
// time with data (0 = every row).
type WorldQuery struct {
	WorldFilter

	Sort           Sort
	Limit          int
	MinExpSeconds  int64
	MinLootSeconds int64
}

type WorldRow struct {
	Totals

	Key    string
	Name   string
	Guilds int
}

type WorldGuildRow struct {
	Totals

	GuildID string
	Name    string
	Icon    string
}

// SpawnRow is one respawn name across guilds. Key is lower(trim(name)); Name its most common spelling.
type SpawnRow struct {
	Totals

	Key    string
	Name   string
	Worlds int
}

// SpawnSpotRow is one guild's respawn of a spawn.
type SpawnSpotRow struct {
	Totals

	WorldKey  string
	World     string
	GuildID   string
	GuildName string
	SpotID    int64
	SpotName  string
	Archived  bool
}

// WorldCharacterRow is a character on a world; GuildID is the guild of its latest reservation.
type WorldCharacterRow struct {
	CharacterRow

	GuildID string
}

type WorldCharacterBoards struct {
	ByExp    []WorldCharacterRow
	ByProfit []WorldCharacterRow
}

type WorldDetail struct {
	Key                   string
	Name                  string
	Range                 Range
	Totals                Totals
	Guilds                []WorldGuildRow
	SpawnsByExpPerHour    []SpawnRow
	SpawnsByProfitPerHour []SpawnRow
	CharactersByExp       []WorldCharacterRow
	CharactersByProfit    []WorldCharacterRow
}

type SpawnDetail struct {
	Key    string
	Name   string
	Range  Range
	Totals Totals
	Worlds []WorldRow
	Spots  []SpawnSpotRow
}

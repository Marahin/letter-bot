package world

type Player struct {
	Name string `json:"name"`
}

type World struct {
	OnlinePlayers []Player `json:"online_players"`
}

type Response struct {
	World World `json:"world"`
}

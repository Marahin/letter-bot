package lootcalc

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vueSample is the placeholder of Calculator.vue, verbatim.
const vueSample = `Session data: From 2020-06-01, 11:18:39 to 2020-06-01, 12:31:43
Session: 01:13h
Loot Type: Market
Loot: 1,904,159
Supplies: 1,010,086
Balance: 894,073
Marahin
    Loot: 87,536
    Supplies: 67,484
    Balance: -100
    Damage: 1,750,214
    Healing: 355,923
Iscarlott
    Loot: 0
    Supplies: 414,235
    Balance: 200
    Damage: 200
    Healing: 1,589,899
Killer Potato (Leader)
    Loot: 1,816,623
    Supplies: 528,367
    Balance: 0
    Damage: 4,043,228
    Healing: 398,456
`

// designSample is the session of the Claude Design mockup (Result.dc.html).
const designSample = "Session data: From 2026-09-25, 20:14:39 to 2026-09-25, 21:27:43\r\n" +
	"Session: 01:13h\r\n" +
	"Loot Type: Leader\r\n" +
	"Loot: 1,904,159\r\n" +
	"Supplies: 1,010,086\r\n" +
	"Balance: 894,073\r\n" +
	"Marahin\r\n" +
	"\tLoot: 87,536\r\n\tSupplies: 67,484\r\n\tBalance: 20,052\r\n\tDamage: 1,750,214\r\n\tHealing: 355,923\r\n" +
	"Iscarlott\r\n" +
	"\tLoot: 0\r\n\tSupplies: 414,235\r\n\tBalance: -414,235\r\n\tDamage: 200\r\n\tHealing: 1,589,899\r\n" +
	"Killer Potato (Leader)\r\n" +
	"\tLoot: 1,816,623\r\n\tSupplies: 528,367\r\n\tBalance: 1,288,256\r\n\tDamage: 4,043,228\r\n\tHealing: 398,456\r\n"

func block(name string, balance string) string {
	return name + "\n    Loot: 0\n    Supplies: 0\n    Balance: " + balance + "\n    Damage: 0\n    Healing: 0\n"
}

func TestParse_VueSample(t *testing.T) {
	// when
	s, err := Parse(vueSample)

	// then
	require.NoError(t, err)
	assert.Equal(t, "2020-06-01, 11:18:39", s.From)
	assert.Equal(t, "2020-06-01, 12:31:43", s.To)
	assert.Equal(t, "01:13h", s.Duration)
	assert.Equal(t, []Player{
		{Name: "Marahin", Loot: 87536, Supplies: 67484, Balance: -100, Damage: 1750214, Healing: 355923},
		{Name: "Iscarlott", Loot: 0, Supplies: 414235, Balance: 200, Damage: 200, Healing: 1589899},
		{Name: "Killer Potato", Leader: true, Loot: 1816623, Supplies: 528367, Balance: 0, Damage: 4043228, Healing: 398456},
	}, s.Players)
}

func TestParse_CRLFTabsAndLeader(t *testing.T) {
	// when
	s, err := Parse(designSample)

	// then
	require.NoError(t, err)
	require.Len(t, s.Players, 3)
	assert.Equal(t, "Killer Potato", s.Players[2].Name)
	assert.True(t, s.Players[2].Leader)
	assert.Equal(t, int64(-414235), s.Players[1].Balance)
	assert.Equal(t, "2026-09-25, 20:14:39", s.From)
}

func TestParse_Tolerant(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		players []string
		from    string
		dur     string
	}{
		{name: "no header", text: block("A", "10") + block("B", "20"), players: []string{"A", "B"}},
		{name: "blank lines and trailing spaces", text: "\n\nA  \n\n    Loot: 1\n    Supplies: 1\n    Balance: 1\n    Damage: 1\n    Healing: 1\n\n", players: []string{"A"}},
		{name: "unknown key ignored", text: "A\n    Loot: 1\n    Supplies: 1\n    Balance: 1\n    Damage: 1\n    Healing: 1\n    Kills: 7\n", players: []string{"A"}},
		{name: "single space indent", text: "A\n Loot: 1\n Supplies: 1\n Balance: 1\n Damage: 1\n Healing: 1", players: []string{"A"}},
		{name: "indented lines before any name are ignored", text: "    stray: line\n" + block("A", "1"), players: []string{"A"}},
		{name: "header anywhere", text: "From 2026-01-01, 10:00:00 to 2026-01-01, 11:00:00\nSession: 01:00h\n" + block("A", "1"), players: []string{"A"}, from: "2026-01-01, 10:00:00", dur: "01:00h"},
		{name: "name with spaces and leader suffix", text: block("Dark Quiet (Leader)", "1"), players: []string{"Dark Quiet"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			s, err := Parse(tt.text)

			// then
			require.NoError(t, err)
			var names []string
			for _, p := range s.Players {
				names = append(names, p.Name)
			}
			assert.Equal(t, tt.players, names)
			assert.Equal(t, tt.from, s.From)
			assert.Equal(t, tt.dur, s.Duration)
		})
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		err     error
		player  string
		line    int
		missing []string
		msg     string
	}{
		{name: "empty", text: "  \n\t\n", err: ErrEmpty, msg: "session text is empty"},
		{name: "no players", text: "Session: 01:00h\nLoot: 5\nBalance: 5\n", err: ErrNoPlayers, msg: "no player block"},
		{name: "indentation stripped", text: strings.ReplaceAll(vueSample, "    ", ""), err: ErrNoPlayers},
		{name: "missing damage and healing", text: "Marahin\n    Loot: 1\n    Supplies: 1\n    Balance: 1\n" + block("B", "1"), err: ErrMissingFields, player: "Marahin", missing: []string{"Damage", "Healing"}, msg: `player "Marahin" has no Damage, Healing`},
		{name: "last block incomplete", text: block("A", "1") + "Iscarlott\n    Loot: 0\n    Supplies: 414,235\n    Balance: -414,235", err: ErrMissingFields, player: "Iscarlott", missing: []string{"Damage", "Healing"}},
		{name: "malformed value", text: "A\n    Loot: 1\n    Balance: 12abc\n", err: ErrMalformedValue, player: "A", line: 3, msg: `line 3 ("Balance: 12abc")`},
		{name: "no colon", text: "A\n    Loot 1\n", err: ErrMalformedValue, player: "A", line: 2},
		{name: "overflow", text: "A\n    Balance: 99,999,999,999,999,999,999\n", err: ErrMalformedValue, player: "A", line: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			_, err := Parse(tt.text)

			// then
			require.ErrorIs(t, err, tt.err)
			var pe *ParseError
			require.True(t, errors.As(err, &pe))
			assert.Equal(t, tt.player, pe.Player)
			assert.Equal(t, tt.line, pe.Line)
			assert.Equal(t, tt.missing, pe.Missing)
			assert.Contains(t, err.Error(), tt.msg)
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		players   []Player
		total     int64
		perHead   int64
		transfers []Transfer
	}{
		{
			name:    "vue sample",
			players: []Player{{Name: "Marahin", Balance: -100}, {Name: "Iscarlott", Balance: 200}, {Name: "Killer Potato", Balance: 0}},
			total:   100, perHead: 33,
			transfers: []Transfer{{"Iscarlott", "Marahin", 133}, {"Iscarlott", "Killer Potato", 33}},
		},
		{
			name:    "design sample",
			players: []Player{{Name: "Marahin", Balance: 20052}, {Name: "Iscarlott", Balance: -414235}, {Name: "Killer Potato", Balance: 1288256}},
			total:   894073, perHead: 298024,
			transfers: []Transfer{{"Killer Potato", "Marahin", 277972}, {"Killer Potato", "Iscarlott", 712259}},
		},
		{
			name:    "negative total floors toward minus infinity",
			players: []Player{{Name: "A", Balance: -100}, {Name: "B", Balance: 0}, {Name: "C", Balance: 0}},
			total:   -100, perHead: -34,
			transfers: []Transfer{{"B", "A", 34}, {"C", "A", 32}},
		},
		{
			name:    "even split needs no transfer",
			players: []Player{{Name: "A", Balance: 50}, {Name: "B", Balance: 50}},
			total:   100, perHead: 50,
		},
		{
			name:    "single player",
			players: []Player{{Name: "Solo", Balance: 12345}},
			total:   12345, perHead: 12345,
		},
		{
			name:    "biggest giver first, takers in paste order",
			players: []Player{{Name: "T1", Balance: 0}, {Name: "G1", Balance: 300}, {Name: "T2", Balance: 0}, {Name: "G2", Balance: 900}},
			total:   1200, perHead: 300,
			transfers: []Transfer{{"G2", "T1", 300}, {"G2", "T2", 300}},
		},
		{
			name:    "two givers share one taker",
			players: []Player{{Name: "G1", Balance: 200}, {Name: "G2", Balance: 250}, {Name: "T", Balance: -450}},
			total:   0, perHead: 0,
			transfers: []Transfer{{"G2", "T", 250}, {"G1", "T", 200}},
		},
		{name: "no players"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			r := Split(Session{Players: tt.players})

			// then
			assert.Equal(t, tt.total, r.Total)
			assert.Equal(t, tt.perHead, r.PerHead)
			assert.Equal(t, len(tt.players), r.Players)
			assert.Equal(t, tt.transfers, r.Transfers)
		})
	}
}

func TestFloorDiv(t *testing.T) {
	tests := []struct{ a, b, want int64 }{
		{7, 2, 3}, {-7, 2, -4}, {6, 3, 2}, {-6, 3, -2}, {7, -2, -4}, {0, 5, 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, floorDiv(tt.a, tt.b), "%d/%d", tt.a, tt.b)
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 gp"}, {999, "999 gp"}, {1000, "1.000 gp"}, {894073, "894.073 gp"},
		{1234567, "1.234.567 gp"}, {-414235, "-414.235 gp"}, {-100, "-100 gp"},
		{math.MinInt64, "-9.223.372.036.854.775.808 gp"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, FormatAmount(tt.n))
	}
}

func TestBankCommand(t *testing.T) {
	assert.Equal(t, "transfer 712259 to Iscarlott", BankCommand(Transfer{From: "Killer Potato", To: "Iscarlott", Amount: 712259}))
}

func designResult(t *testing.T) Result {
	t.Helper()
	s, err := Parse(designSample)
	require.NoError(t, err)
	return Split(s)
}

func TestDiscordText(t *testing.T) {
	// when
	got := DiscordText(designResult(t))

	// then
	assert.Equal(t, "**Killer Potato** should give **277.972 gp** to **Marahin**.\n"+
		"**Killer Potato** should give **712.259 gp** to **Iscarlott**.\n"+
		"\n\n"+
		"Total profit: **894.073 gp** which is **298.024 gp** for each player.\n"+
		"\n*Powered by https://tibialoot.com*", got)
}

func TestTeamSpeakText(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want string
	}{
		{
			name: "profit is green",
			r:    designResult(t),
			want: "\n\n" +
				"[b]Killer Potato[/b] should give [b]277.972 gp[/b] to [b]Marahin[/b].\n" +
				"[b]Killer Potato[/b] should give [b]712.259 gp[/b] to [b]Iscarlott[/b].\n" +
				"\n\n" +
				"Total profit [color=green]894.073 gp[/color] which is [color=green]298.024 gp[/color] for each player.\n" +
				"\n[i] Powered by https://tibialoot.com [/i]",
		},
		{
			name: "waste is red",
			r:    Result{Total: -100, PerHead: -34, Players: 3},
			want: "\n\n\n\n\nTotal profit [color=red]-100 gp[/color] which is [color=red]-34 gp[/color] for each player.\n\n[i] Powered by https://tibialoot.com [/i]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TeamSpeakText(tt.r))
		})
	}
}

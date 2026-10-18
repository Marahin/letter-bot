package lootcalc

import (
	"fmt"
	"strconv"
	"strings"
)

const poweredBy = "https://tibialoot.com"

// GroupDigits writes n with dot thousands separators: -1234567 → "-1.234.567".
func GroupDigits(n int64) string {
	digits := strconv.FormatUint(absUint(n), 10)
	var b strings.Builder
	if n < 0 {
		b.WriteByte('-')
	}
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// FormatAmount is GroupDigits plus the gold unit, as prepareAmountString.
func FormatAmount(n int64) string {
	return GroupDigits(n) + " gp"
}

// BankCommand is what a player says to an NPC banker to make the transfer.
func BankCommand(t Transfer) string {
	return fmt.Sprintf("transfer %d to %s", t.Amount, t.To)
}

// DiscordText is the result as Discord markdown, as generateDiscordResult.
func DiscordText(r Result) string {
	rows := make([]string, 0, len(r.Transfers))
	for _, t := range r.Transfers {
		rows = append(rows, fmt.Sprintf("**%s** should give **%s** to **%s**.", t.From, FormatAmount(t.Amount), t.To))
	}
	summary := fmt.Sprintf("Total profit: **%s** which is **%s** for each player.", FormatAmount(r.Total), FormatAmount(r.PerHead))
	return strings.Join([]string{
		strings.Join(rows, "\n"),
		"\n",
		summary,
		"\n*Powered by " + poweredBy + "*",
	}, "\n")
}

// TeamSpeakText is the result as TeamSpeak BBCode, as generateTeamSpeakResult.
// The original printed "gp" twice in the summary ("1.000 gp[/color] gp"); this
// prints it once.
func TeamSpeakText(r Result) string {
	rows := make([]string, 0, len(r.Transfers))
	for _, t := range r.Transfers {
		rows = append(rows, fmt.Sprintf("[b]%s[/b] should give [b]%s[/b] to [b]%s[/b].", t.From, FormatAmount(t.Amount), t.To))
	}
	color := "red"
	if r.Total > 0 {
		color = "green"
	}
	summary := fmt.Sprintf("Total profit [color=%s]%s[/color] which is [color=%s]%s[/color] for each player.",
		color, FormatAmount(r.Total), color, FormatAmount(r.PerHead))
	return strings.Join([]string{
		"\n",
		strings.Join(rows, "\n"),
		"\n",
		summary,
		"\n[i] Powered by " + poweredBy + " [/i]",
	}, "\n")
}

func absUint(n int64) uint64 {
	if n < 0 {
		return uint64(-(n + 1)) + 1
	}
	return uint64(n)
}

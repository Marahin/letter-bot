// Package lootcalc splits the balance of a Tibia Party Hunt session evenly
// between the party. It is a port of the tibialoot.com Loot Calculator
// (tibiadata-front components/Calculator.vue).
package lootcalc

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Field names of a player block, in the order the analyser prints them.
const (
	FieldLoot     = "Loot"
	FieldSupplies = "Supplies"
	FieldBalance  = "Balance"
	FieldDamage   = "Damage"
	FieldHealing  = "Healing"
)

// Fields are the lines every player block must have.
var Fields = []string{FieldLoot, FieldSupplies, FieldBalance, FieldDamage, FieldHealing}

const leaderSuffix = " (Leader)"

var (
	ErrEmpty          = errors.New("session text is empty")
	ErrNoPlayers      = errors.New("session text has no player block")
	ErrMissingFields  = errors.New("player block is incomplete")
	ErrMalformedValue = errors.New("player line has a malformed value")
)

// ParseError tells which player and line made Parse fail. It wraps one of the
// Err* sentinels.
type ParseError struct {
	Err    error
	Player string
	// Line is 1-based; 0 when the error is not about one line.
	Line int
	// Missing lists the absent fields for ErrMissingFields.
	Missing []string
	// Text is the offending line for ErrMalformedValue.
	Text string
}

func (e *ParseError) Error() string {
	switch {
	case len(e.Missing) > 0:
		return fmt.Sprintf("%v: player %q has no %s", e.Err, e.Player, strings.Join(e.Missing, ", "))
	case e.Line > 0:
		return fmt.Sprintf("%v: line %d (%q)", e.Err, e.Line, e.Text)
	default:
		return e.Err.Error()
	}
}

func (e *ParseError) Unwrap() error { return e.Err }

// Player is one member of the party as the analyser reports them.
type Player struct {
	Name     string
	Leader   bool
	Loot     int64
	Supplies int64
	Balance  int64
	Damage   int64
	Healing  int64
}

// Session is a parsed Party Hunt analyser text. From, To and Duration are kept
// as printed ("2020-06-01, 11:18:39", "01:13h") and are empty when absent.
type Session struct {
	From     string
	To       string
	Duration string
	Players  []Player
}

var headerRe = regexp.MustCompile(`From (\S+, \S+) to (\S+, \S+)`)

// Parse reads a Party Hunt analyser text. A player block is a name line that is
// not indented, followed by indented "Key: value" lines. Values may carry comma
// thousands separators and a minus sign. Unknown keys are ignored; a block
// without all of Fields is an error.
func Parse(text string) (Session, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if strings.TrimSpace(text) == "" {
		return Session{}, &ParseError{Err: ErrEmpty}
	}

	p := parser{}
	for i, line := range strings.Split(text, "\n") {
		if err := p.line(i+1, line); err != nil {
			return Session{}, err
		}
	}
	if err := p.flush(); err != nil {
		return Session{}, err
	}
	if len(p.session.Players) == 0 {
		return Session{}, &ParseError{Err: ErrNoPlayers}
	}
	return p.session, nil
}

type parser struct {
	session Session
	// name is the last unindented line: the player name if indented lines follow.
	name    string
	current *Player
	seen    map[string]bool
}

func (p *parser) line(n int, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if raw[0] != ' ' && raw[0] != '\t' {
		if err := p.flush(); err != nil {
			return err
		}
		p.header(trimmed)
		p.name = trimmed
		return nil
	}
	if p.current == nil {
		if p.name == "" {
			return nil
		}
		p.start()
	}
	return p.field(n, trimmed)
}

func (p *parser) header(line string) {
	if m := headerRe.FindStringSubmatch(line); m != nil && p.session.From == "" {
		p.session.From, p.session.To = m[1], m[2]
	}
	if v, ok := strings.CutPrefix(line, "Session:"); ok && p.session.Duration == "" {
		p.session.Duration = strings.TrimSpace(v)
	}
}

func (p *parser) start() {
	name, leader := strings.CutSuffix(p.name, leaderSuffix)
	p.current = &Player{Name: strings.TrimSpace(name), Leader: leader}
	p.seen = map[string]bool{}
}

func (p *parser) field(n int, line string) error {
	malformed := &ParseError{Err: ErrMalformedValue, Player: p.current.Name, Line: n, Text: line}
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return malformed
	}
	var dst *int64
	switch strings.TrimSpace(key) {
	case FieldLoot:
		dst = &p.current.Loot
	case FieldSupplies:
		dst = &p.current.Supplies
	case FieldBalance:
		dst = &p.current.Balance
	case FieldDamage:
		dst = &p.current.Damage
	case FieldHealing:
		dst = &p.current.Healing
	default:
		return nil
	}
	v, err := parseAmount(value)
	if err != nil {
		return malformed
	}
	*dst = v
	p.seen[strings.TrimSpace(key)] = true
	return nil
}

func (p *parser) flush() error {
	if p.current == nil {
		return nil
	}
	var missing []string
	for _, f := range Fields {
		if !p.seen[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return &ParseError{Err: ErrMissingFields, Player: p.current.Name, Missing: missing}
	}
	p.session.Players = append(p.session.Players, *p.current)
	p.current = nil
	p.name = ""
	return nil
}

func parseAmount(s string) (int64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	return strconv.ParseInt(s, 10, 64)
}

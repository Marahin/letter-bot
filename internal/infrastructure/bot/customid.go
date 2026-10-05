package bot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The version prefix lets a later release refuse the buttons of older summaries.
const (
	customIDPrefix    = "lf1"
	maxCustomIDLength = 100
)

const (
	inputSpot  = "spot"
	inputStart = "start"
	inputEnd   = "end"
)

var errBadCustomID = errors.New("unknown custom id")

type formActionKind int

const (
	actionBookForm formActionKind = iota + 1
	actionMine
	actionList
	actionEditForm
	actionCancel
	actionCancelConfirm
	actionOverbook
	actionBookPick
	actionEditPick
	actionBookRetry
	actionEditRetry
	actionBookSubmit
	actionEditSubmit
)

type idField int

const (
	fieldReservation idField = iota + 1
	fieldSpot
	fieldStart
	fieldEnd
	fieldStartText
	fieldEndText
	// fieldSpotText must be the last field: it may hold ":".
	fieldSpotText
)

type actionLayout struct {
	name   string
	fields []idField
}

var actionLayouts = map[formActionKind]actionLayout{
	actionBookForm:      {"book", nil},
	actionMine:          {"mine", nil},
	actionList:          {"list", nil},
	actionEditForm:      {"edit", []idField{fieldReservation}},
	actionCancel:        {"cancel", []idField{fieldReservation}},
	actionCancelConfirm: {"cancelok", []idField{fieldReservation}},
	actionOverbook:      {"ob", []idField{fieldSpot, fieldStart, fieldEnd}},
	actionBookPick:      {"pick", []idField{fieldStart, fieldEnd}},
	actionEditPick:      {"epick", []idField{fieldReservation, fieldStart, fieldEnd}},
	actionBookRetry:     {"retry", []idField{fieldStartText, fieldEndText, fieldSpotText}},
	actionEditRetry:     {"eretry", []idField{fieldReservation, fieldStartText, fieldEndText, fieldSpotText}},
	actionBookSubmit:    {"mbook", nil},
	actionEditSubmit:    {"medit", []idField{fieldReservation}},
}

var actionsByName = func() map[string]formActionKind {
	byName := make(map[string]formActionKind, len(actionLayouts))
	for kind, layout := range actionLayouts {
		byName[layout.name] = kind
	}
	return byName
}()

// formAction is the state of a button, a select or a form. Discord gives it back
// in the custom id, so the bot stores nothing between two clicks.
type formAction struct {
	Kind          formActionKind
	ReservationID int64
	SpotID        int64
	StartAt       time.Time
	EndAt         time.Time
	// StartText and EndText are "HHMM" or empty, to fill a form again.
	StartText string
	EndText   string
	SpotText  string
}

// customID encodes the action. A long SpotText is cut to fit Discord's limit.
func (a formAction) customID() string {
	layout := actionLayouts[a.Kind]
	parts := make([]string, 0, 2+len(layout.fields))
	parts = append(parts, customIDPrefix, layout.name)
	for _, field := range layout.fields {
		parts = append(parts, a.field(field))
	}
	id := strings.Join(parts, ":")
	for utf8.RuneCountInString(id) > maxCustomIDLength {
		_, size := utf8.DecodeLastRuneInString(id)
		id = id[:len(id)-size]
	}
	return id
}

func (a formAction) field(field idField) string {
	switch field {
	case fieldReservation:
		return strconv.FormatInt(a.ReservationID, 10)
	case fieldSpot:
		return strconv.FormatInt(a.SpotID, 10)
	case fieldStart:
		return strconv.FormatInt(a.StartAt.Unix(), 10)
	case fieldEnd:
		return strconv.FormatInt(a.EndAt.Unix(), 10)
	case fieldStartText:
		return a.StartText
	case fieldEndText:
		return a.EndText
	case fieldSpotText:
		return a.SpotText
	}
	return ""
}

func parseFormAction(id string) (formAction, error) {
	rest, ok := strings.CutPrefix(id, customIDPrefix+":")
	if !ok {
		return formAction{}, errBadCustomID
	}
	name, args, _ := strings.Cut(rest, ":")
	kind, ok := actionsByName[name]
	if !ok {
		return formAction{}, errBadCustomID
	}
	layout := actionLayouts[kind]
	var values []string
	if len(layout.fields) > 0 {
		values = strings.SplitN(args, ":", len(layout.fields))
	} else if args != "" {
		return formAction{}, errBadCustomID
	}
	if len(values) != len(layout.fields) {
		return formAction{}, errBadCustomID
	}
	a := formAction{Kind: kind}
	for i, field := range layout.fields {
		if err := setField(&a, field, values[i]); err != nil {
			return formAction{}, fmt.Errorf("%w: %w", errBadCustomID, err)
		}
	}
	return a, nil
}

func setField(a *formAction, field idField, value string) error {
	var err error
	switch field {
	case fieldReservation:
		a.ReservationID, err = parsePositiveID(value)
	case fieldSpot:
		a.SpotID, err = parsePositiveID(value)
	case fieldStart:
		a.StartAt, err = parseUnix(value)
	case fieldEnd:
		a.EndAt, err = parseUnix(value)
	case fieldStartText:
		a.StartText, err = parseClockText(value)
	case fieldEndText:
		a.EndText, err = parseClockText(value)
	case fieldSpotText:
		a.SpotText = value
	}
	return err
}

func parsePositiveID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("bad id %q", value)
	}
	return id, nil
}

func parseUnix(value string) (time.Time, error) {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("bad time %q", value)
	}
	return time.Unix(seconds, 0), nil
}

func parseClockText(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) != 4 || strings.Trim(value, "0123456789") != "" {
		return "", fmt.Errorf("bad clock %q", value)
	}
	return value, nil
}

// clockInput turns "HHMM" back into the "HH:MM" of a form field.
func clockInput(text string) string {
	if len(text) != 4 {
		return ""
	}
	return text[:2] + ":" + text[2:]
}

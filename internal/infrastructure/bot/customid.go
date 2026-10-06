package bot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/reservation"
)

// The version prefix lets a later release refuse the buttons of older summaries.
const (
	customIDPrefix    = "lf1"
	maxCustomIDLength = 100
)

// inputQuery is the text field of the search form.
const inputQuery = "query"

const (
	startNow     = "now"
	maxPage      = 999
	maxWindow    = 9
	maxLengthMin = 24 * 60
	maxIndex     = 9
)

var errBadCustomID = errors.New("unknown custom id")

type formActionKind int

const (
	actionBook formActionKind = iota + 1
	actionMine
	actionList
	actionEdit
	actionCancel
	actionCancelConfirm
	actionOverbook
	// Step 1 of the wizard: the respawn.
	actionRespawnPage
	actionRespawnPick
	actionSearch
	actionSearchSubmit
	// Step 2: the start and the length.
	actionStartPick
	actionLengthPick
	actionWindow
	actionSubmit
	// The same steps for an edit, with the reservation id.
	actionEditRespawnPage
	actionEditRespawnPick
	actionEditSearch
	actionEditSearchSubmit
	actionEditStartPick
	actionEditLengthPick
	actionEditWindow
	actionEditSubmit
)

// editTwin is the edit variant of each wizard action.
var editTwin = map[formActionKind]formActionKind{
	actionRespawnPage:  actionEditRespawnPage,
	actionRespawnPick:  actionEditRespawnPick,
	actionSearch:       actionEditSearch,
	actionSearchSubmit: actionEditSearchSubmit,
	actionStartPick:    actionEditStartPick,
	actionLengthPick:   actionEditLengthPick,
	actionWindow:       actionEditWindow,
	actionSubmit:       actionEditSubmit,
}

var bookTwin = func() map[formActionKind]formActionKind {
	twins := make(map[formActionKind]formActionKind, len(editTwin))
	for book, edit := range editTwin {
		twins[edit] = book
	}
	return twins
}()

type idField int

const (
	fieldReservation idField = iota + 1
	fieldSpot
	fieldStart
	fieldEnd
	fieldPage
	// fieldWindow is empty for reservation.AutoWindow.
	fieldWindow
	// fieldChoice is the chosen start: empty, "now" or Unix seconds.
	fieldChoice
	fieldLength
	// fieldIndex tells apart the selects of one message.
	fieldIndex
)

type actionLayout struct {
	name   string
	fields []idField
}

var actionLayouts = map[formActionKind]actionLayout{
	actionBook:             {"book", nil},
	actionMine:             {"mine", nil},
	actionList:             {"list", nil},
	actionEdit:             {"edit", []idField{fieldReservation}},
	actionCancel:           {"cancel", []idField{fieldReservation}},
	actionCancelConfirm:    {"cancelok", []idField{fieldReservation}},
	actionOverbook:         {"ob", []idField{fieldSpot, fieldStart, fieldEnd}},
	actionRespawnPage:      {"rp", []idField{fieldPage}},
	actionRespawnPick:      {"rs", []idField{fieldIndex}},
	actionSearch:           {"rq", nil},
	actionSearchSubmit:     {"rqm", nil},
	actionStartPick:        {"ws", []idField{fieldSpot, fieldWindow, fieldLength}},
	actionLengthPick:       {"wl", []idField{fieldSpot, fieldWindow, fieldChoice}},
	actionWindow:           {"ww", []idField{fieldSpot, fieldWindow, fieldChoice, fieldLength}},
	actionSubmit:           {"wb", []idField{fieldSpot, fieldChoice, fieldLength}},
	actionEditRespawnPage:  {"erp", []idField{fieldReservation, fieldChoice, fieldLength, fieldPage}},
	actionEditRespawnPick:  {"ers", []idField{fieldReservation, fieldChoice, fieldLength, fieldIndex}},
	actionEditSearch:       {"erq", []idField{fieldReservation, fieldChoice, fieldLength}},
	actionEditSearchSubmit: {"erqm", []idField{fieldReservation, fieldChoice, fieldLength}},
	actionEditStartPick:    {"es", []idField{fieldReservation, fieldSpot, fieldWindow, fieldLength}},
	actionEditLengthPick:   {"el", []idField{fieldReservation, fieldSpot, fieldWindow, fieldChoice}},
	actionEditWindow:       {"ew", []idField{fieldReservation, fieldSpot, fieldWindow, fieldChoice, fieldLength}},
	actionEditSubmit:       {"eb", []idField{fieldReservation, fieldSpot, fieldChoice, fieldLength}},
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
	// StartAt is the chosen start in the wizard, the start of an overbook otherwise.
	StartAt time.Time
	EndAt   time.Time
	Now     bool
	Length  time.Duration
	Window  int
	Page    int
	Index   int
}

func (a formAction) editing() bool {
	return a.ReservationID != 0
}

// as returns the wizard action of the kind, in the edit variant when the state
// has a reservation.
func (a formAction) as(kind formActionKind) formAction {
	if twin, ok := bookTwin[kind]; ok {
		kind = twin
	}
	if twin, ok := editTwin[kind]; ok && a.editing() {
		kind = twin
	}
	a.Kind = kind
	return a
}

func (a formAction) choice() reservation.TimeChoice {
	return reservation.TimeChoice{
		SpotID:        a.SpotID,
		ReservationID: a.ReservationID,
		Window:        a.Window,
		Now:           a.Now,
		StartAt:       a.StartAt,
		Length:        a.Length,
	}
}

func choiceAction(c reservation.TimeChoice) formAction {
	return formAction{SpotID: c.SpotID, ReservationID: c.ReservationID, Window: c.Window, Now: c.Now, StartAt: c.StartAt, Length: c.Length}
}

func (a formAction) customID() string {
	layout := actionLayouts[a.Kind]
	parts := make([]string, 0, 2+len(layout.fields))
	parts = append(parts, customIDPrefix, layout.name)
	for _, field := range layout.fields {
		parts = append(parts, a.field(field))
	}
	return strings.Join(parts, ":")
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
	case fieldPage:
		return strconv.Itoa(a.Page)
	case fieldWindow:
		if a.Window == reservation.AutoWindow {
			return ""
		}
		return strconv.Itoa(a.Window)
	case fieldChoice:
		return choiceValue(a.Now, a.StartAt)
	case fieldLength:
		return strconv.Itoa(int(a.Length / time.Minute))
	case fieldIndex:
		return strconv.Itoa(a.Index)
	}
	return ""
}

// choiceValue is a start as a custom id field and as a select value.
func choiceValue(now bool, startAt time.Time) string {
	switch {
	case now:
		return startNow
	case startAt.IsZero():
		return ""
	}
	return strconv.FormatInt(startAt.Unix(), 10)
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
		values = strings.Split(args, ":")
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
	case fieldPage:
		a.Page, err = parseSmall(value, maxPage)
	case fieldWindow:
		a.Window = reservation.AutoWindow
		if value != "" {
			a.Window, err = parseSmall(value, maxWindow)
		}
	case fieldChoice:
		a.Now, a.StartAt, err = parseChoice(value)
	case fieldLength:
		var minutes int
		minutes, err = parseSmall(value, maxLengthMin)
		a.Length = time.Duration(minutes) * time.Minute
	case fieldIndex:
		a.Index, err = parseSmall(value, maxIndex)
	}
	return err
}

func parseChoice(value string) (bool, time.Time, error) {
	switch value {
	case "":
		return false, time.Time{}, nil
	case startNow:
		return true, time.Time{}, nil
	}
	startAt, err := parseUnix(value)
	return false, startAt, err
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
	if err != nil || strings.Trim(value, "0123456789") != "" {
		return time.Time{}, fmt.Errorf("bad time %q", value)
	}
	return time.Unix(seconds, 0), nil
}

// parseSmall reads 0..limit, digits only.
func parseSmall(value string, limit int) (int, error) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("bad number %q", value)
	}
	n, err := strconv.Atoi(value)
	if err != nil || n > limit {
		return 0, fmt.Errorf("bad number %q", value)
	}
	return n, nil
}

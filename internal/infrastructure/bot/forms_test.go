package bot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservationforms"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/infrastructure/bot/formatter"
)

type formFixture struct {
	b       *Bot
	gateway *mocks.MockInteractionGateway
	forms   *mocks.MockReservationFormService
	configs *mocks.MockGuildConfigRepository
	edits   []*discordgo.WebhookEdit
}

func newFormFixture(t *testing.T) *formFixture {
	f := &formFixture{
		gateway: mocks.NewMockInteractionGateway(t),
		forms:   mocks.NewMockReservationFormService(t),
		configs: mocks.NewMockGuildConfigRepository(t),
	}
	f.b = &Bot{
		gateway:      f.gateway,
		forms:        f.forms,
		guildConfigs: f.configs,
		formatter:    formatter.NewFormatter(),
		log:          zap.NewNop().Sugar(),
		webBaseURL:   "https://tibialoot.test",
	}
	return f
}

func (f *formFixture) premium(cfg guildconfig.Config) {
	cfg.GuildID, cfg.Premium = "g1", true
	f.configs.On("Get", mock.Anything, "g1").Return(&cfg, nil)
	f.gateway.On("StateGuild", "g1").Return(&discordgo.Guild{ID: "g1", Name: "Celesta", OwnerID: "owner"}, nil)
}

// expectRespond records the response type and data.
func (f *formFixture) expectRespond(i *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	got := &discordgo.InteractionResponse{}
	f.gateway.On("InteractionRespond", i.Interaction, mock.Anything).
		Run(func(args mock.Arguments) { *got = *args.Get(1).(*discordgo.InteractionResponse) }).
		Return(nil).Once()
	return got
}

func (f *formFixture) expectEdit(i *discordgo.InteractionCreate) {
	f.gateway.On("InteractionResponseEdit", i.Interaction, mock.Anything).
		Run(func(args mock.Arguments) { f.edits = append(f.edits, args.Get(1).(*discordgo.WebhookEdit)) }).
		Return(&discordgo.Message{}, nil).Once()
}

func (f *formFixture) editedContent(t *testing.T) string {
	t.Helper()
	require.Len(t, f.edits, 1)
	return *f.edits[0].Content
}

func (f *formFixture) editedCustomIDs(t *testing.T) []string {
	t.Helper()
	require.Len(t, f.edits, 1)
	raw, err := json.Marshal(f.edits[0].Components)
	require.NoError(t, err)
	var rows []sentRow
	require.NoError(t, json.Unmarshal(raw, &rows))
	var ids []string
	for _, r := range rows {
		for _, c := range r.Components {
			if c.CustomID != "" {
				ids = append(ids, c.CustomID)
			}
		}
	}
	return ids
}

type sentComponent struct {
	CustomID string `json:"custom_id"`
}

type sentRow struct {
	Components []sentComponent `json:"components"`
}

func knight() *discordgo.Member {
	return &discordgo.Member{User: &discordgo.User{ID: "u1", Username: "knight"}, Nick: "Knight"}
}

func component(customID string, values ...string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:      "i1",
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "g1",
		Member:  knight(),
		Data:    discordgo.MessageComponentInteractionData{CustomID: customID, Values: values},
	}}
}

// modalSubmit builds the interaction the way discordgo reads it from the gateway.
func modalSubmit(t *testing.T, customID, query string) *discordgo.InteractionCreate {
	t.Helper()
	payload := map[string]any{
		"id":       "i1",
		"type":     discordgo.InteractionModalSubmit,
		"guild_id": "g1",
		"member":   map[string]any{"nick": "Knight", "user": map[string]any{"id": "u1", "username": "knight"}},
		"data": map[string]any{
			"custom_id": customID,
			"components": []any{
				map[string]any{"type": 1, "components": []any{map[string]any{"type": 4, "custom_id": inputQuery, "value": query}}},
			},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	var i discordgo.InteractionCreate
	require.NoError(t, json.Unmarshal(raw, &i))
	return &i
}

func isActor(userID, name string, reserve bool) any {
	return mock.MatchedBy(func(a reservation.Actor) bool {
		return a.UserID == userID && a.Name == name && a.Caps.Reserve == reserve
	})
}

var formNow = time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)

func mine(id int64) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: id, SpotID: 7, StartAt: formNow.Add(time.Hour), EndAt: formNow.Add(3 * time.Hour), AuthorDiscordID: "u1"},
		Spot:        reservation.Spot{ID: 7, Name: "Library -1"},
	}
}

func respawnPicker(n int) *reservation.RespawnPicker {
	picker := reservationforms.PageRespawns(spotsNamed(n), 0)
	picker.Usual = usual(2)
	return &picker
}

func choiceIs(want reservation.TimeChoice) any {
	return mock.MatchedBy(func(got reservation.TimeChoice) bool {
		return got.SpotID == want.SpotID && got.ReservationID == want.ReservationID && got.Window == want.Window &&
			got.Now == want.Now && got.StartAt.Equal(want.StartAt) && got.Length == want.Length
	})
}

const slotUnix = 1791228600

var slotAt = time.Unix(slotUnix, 0)

func TestBookButton_OpensTheWizard(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("RespawnPicker", mock.Anything, "g1", isActor("u1", "Knight", true), 0).Return(respawnPicker(30), nil)
	i := component("lf1:book")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredChannelMessageWithSource, got.Type)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, got.Data.Flags)
	assert.Contains(t, f.editedContent(t), "**Book a respawn**")
	assert.Equal(t, []string{"lf1:rs:0", "lf1:rs:1", "lf1:rs:2", "lf1:rq"}, f.editedCustomIDs(t))
}

func TestBookButton_RefusesAMemberWithoutTheReserveRank(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
	i := component("lf1:book")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "rank that can book")
}

func TestBookButton_ListFailure(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("RespawnPicker", mock.Anything, "g1", mock.Anything, 0).Return(nil, errors.New("db down"))
	i := component("lf1:book")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "db down")
}

func TestWizardSteps_RefuseAMemberWithoutTheReserveRank(t *testing.T) {
	for _, id := range []string{"lf1:rp:1", "lf1:rs:1", "lf1:ws:7:0:60", "lf1:wl:7:0:now", "lf1:ww:7:1::0", "lf1:wb:7:now:60", "lf1:rqm"} {
		t.Run(id, func(t *testing.T) {
			// given
			f := newFormFixture(t)
			f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
			i := component(id, "7")
			f.expectRespond(i)
			f.expectEdit(i)

			// when
			f.b.InteractionCreate(nil, i)

			// then: the service mocks fail on any call
			assert.Contains(t, f.editedContent(t), "rank that can book")
		})
	}
}

func TestRespawnPage_UpdatesInPlace(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	picker := reservationforms.PageRespawns(spotsNamed(200), 1)
	f.forms.On("RespawnPicker", mock.Anything, "g1", mock.Anything, 1).Return(&picker, nil)
	i := component("lf1:rp:1")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Equal(t, []string{"lf1:rs:1", "lf1:rs:2", "lf1:rs:3", "lf1:rp:0", "lf1:rp:2", "lf1:rq"}, f.editedCustomIDs(t))
}

func TestRespawnPick_ShowsTheTimes(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("TimePicker", mock.Anything, "g1", mock.Anything, choiceIs(reservation.TimeChoice{SpotID: 7, Window: reservation.AutoWindow})).
		Return(timePicker(reservation.TimeChoice{SpotID: 7}), nil)
	i := component("lf1:rs:2", "7")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "**Library -1** — pick the start and the length.")
	assert.Equal(t, []string{"lf1:ws:7:0:0", "lf1:wl:7:0:", "lf1:ww:7:0::0", "lf1:ww:7:1::0", "lf1:wb:7::0", "lf1:rp:0"}, f.editedCustomIDs(t))
}

func TestRespawnPick_ForgedSpot(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("TimePicker", mock.Anything, "g1", mock.Anything, choiceIs(reservation.TimeChoice{SpotID: 999, Window: reservation.AutoWindow})).
		Return(nil, booking.ErrSpotNotFound)
	i := component("lf1:rs:1", "999")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "No respawn has this name")
	assert.Equal(t, []string{"lf1:rp:0"}, f.editedCustomIDs(t))
}

func TestSelects_RefuseABadValue(t *testing.T) {
	for _, tt := range []struct {
		id     string
		values []string
	}{
		{"lf1:rs:1", []string{"x"}},
		{"lf1:rs:1", nil},
		{"lf1:ws:7:0:60", []string{"soon"}},
		{"lf1:ws:7:0:60", []string{""}},
		{"lf1:wl:7:0:now", []string{"0"}},
		{"lf1:wl:7:0:now", []string{"60", "90"}},
	} {
		t.Run(tt.id, func(t *testing.T) {
			// given
			f := newFormFixture(t)
			f.premium(guildconfig.Config{})
			i := component(tt.id, tt.values...)
			f.expectRespond(i)
			f.expectEdit(i)

			// when
			f.b.InteractionCreate(nil, i)

			// then
			assert.Contains(t, f.editedContent(t), "out of date")
		})
	}
}

func TestTimeSelects_KeepTheOtherChoices(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		values []string
		want   reservation.TimeChoice
	}{
		{"start", "lf1:ws:7:0:120", []string{"1791228600"}, reservation.TimeChoice{SpotID: 7, StartAt: slotAt, Length: 2 * time.Hour}},
		{"start now", "lf1:ws:7:0:0", []string{"now"}, reservation.TimeChoice{SpotID: 7, Now: true}},
		{"length", "lf1:wl:7:0:now", []string{"90"}, reservation.TimeChoice{SpotID: 7, Now: true, Length: 90 * time.Minute}},
		{"window", "lf1:ww:7:1:1791228600:30", nil, reservation.TimeChoice{SpotID: 7, Window: 1, StartAt: slotAt, Length: 30 * time.Minute}},
		{"edit start", "lf1:es:5:7:0:60", []string{"now"}, reservation.TimeChoice{ReservationID: 5, SpotID: 7, Now: true, Length: time.Hour}},
		{"edit respawn", "lf1:ers:5:1791228600:120:1", []string{"8"}, reservation.TimeChoice{ReservationID: 5, SpotID: 8, Window: reservation.AutoWindow, StartAt: slotAt, Length: 2 * time.Hour}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newFormFixture(t)
			f.premium(guildconfig.Config{})
			shown := tt.want
			if shown.Window == reservation.AutoWindow {
				shown.Window = 0
			}
			f.forms.On("TimePicker", mock.Anything, "g1", mock.Anything, choiceIs(tt.want)).Return(timePicker(shown), nil)
			i := component(tt.id, tt.values...)
			f.expectRespond(i)
			f.expectEdit(i)

			// when
			f.b.InteractionCreate(nil, i)

			// then
			assert.Contains(t, f.editedContent(t), "**Library -1**")
		})
	}
}

func TestSubmit_Books(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	outcome := &reservation.FormOutcome{Draft: reservation.Draft{SpotID: 7, StartAt: formNow, EndAt: formNow.Add(2 * time.Hour)}, SpotName: "Library -1"}
	f.forms.On("BookChoice", mock.Anything, "g1", isActor("u1", "Knight", true), choiceIs(reservation.TimeChoice{SpotID: 7, Now: true, Length: 2 * time.Hour})).
		Return(outcome, nil)
	i := component("lf1:wb:7:now:120")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "Booked **Library -1**")
	assert.Equal(t, []string{"lf1:mine", "lf1:book"}, f.editedCustomIDs(t))
}

func TestSubmit_ConflictsOfferAnOverbookAndARetry(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	draft := reservation.Draft{SpotID: 7, StartAt: slotAt, EndAt: slotAt.Add(2 * time.Hour)}
	outcome := &reservation.FormOutcome{Draft: draft, SpotName: "Library -1", Conflicts: []*reservation.Reservation{{Author: "Druid"}}, CanOverbook: true}
	f.forms.On("BookChoice", mock.Anything, "g1", mock.Anything, mock.Anything).Return(outcome, booking.ErrInsufficientPermissions)
	i := component("lf1:wb:7:1791228600:120")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "overlaps these reservations")
	assert.Equal(t, []string{"lf1:ob:7:1791228600:1791235800", "lf1:ww:7::1791228600:120"}, f.editedCustomIDs(t))
}

func TestSubmit_WithoutAChoice(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("BookChoice", mock.Anything, "g1", mock.Anything, mock.Anything).Return(nil, reservationforms.ErrChoiceIncomplete)
	i := component("lf1:wb:7::0")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, "Choose a start and a length first.", f.editedContent(t))
	assert.Equal(t, []string{"lf1:ww:7:::0"}, f.editedCustomIDs(t))
}

func TestSearchButton_OpensTheForm(t *testing.T) {
	for id, want := range map[string]string{"lf1:rq": "lf1:rqm", "lf1:erq:5:now:60": "lf1:erqm:5:now:60"} {
		t.Run(id, func(t *testing.T) {
			// given
			f := newFormFixture(t)
			f.premium(guildconfig.Config{})
			i := component(id)
			got := f.expectRespond(i)

			// when
			f.b.InteractionCreate(nil, i)

			// then
			assert.Equal(t, discordgo.InteractionResponseModal, got.Type)
			assert.Equal(t, want, got.Data.CustomID)
		})
	}
}

func TestSearchButton_RefusesAMemberWithoutTheReserveRank(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
	i := component("lf1:rq")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseChannelMessageWithSource, got.Type)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, got.Data.Flags)
	assert.Contains(t, got.Data.Content, "rank that can book")
}

func TestSearchSubmit_ExactMatchGoesToTheTimes(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("FindRespawn", mock.Anything, "g1", "library -1").Return(&spot.Spot{ID: 7, Name: "Library -1"}, nil)
	f.forms.On("TimePicker", mock.Anything, "g1", mock.Anything, choiceIs(reservation.TimeChoice{ReservationID: 5, SpotID: 7, Window: reservation.AutoWindow, Now: true, Length: time.Hour})).
		Return(timePicker(reservation.TimeChoice{ReservationID: 5, SpotID: 7, Now: true, Length: time.Hour}), nil)
	i := modalSubmit(t, "lf1:erqm:5:now:60", "library -1")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "**Library -1**")
}

func TestSearchSubmit_Matches(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("FindRespawn", mock.Anything, "g1", "lib").Return(nil, &reservationforms.AmbiguousSpotError{Query: "lib", Candidates: spotsNamed(3)})
	i := modalSubmit(t, "lf1:rqm", "lib")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "Respawns that match **lib**")
	assert.Equal(t, []string{"lf1:rs:4", "lf1:rq", "lf1:rp:0"}, f.editedCustomIDs(t))
}

func TestSearchSubmit_NoMatchShowsTheLists(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("FindRespawn", mock.Anything, "g1", "nowhere").Return(nil, booking.ErrSpotNotFound)
	f.forms.On("RespawnPicker", mock.Anything, "g1", mock.Anything, 0).Return(respawnPicker(3), nil)
	i := modalSubmit(t, "lf1:rqm", "nowhere")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.True(t, strings.HasPrefix(f.editedContent(t), "No respawn has this name"))
	assert.Contains(t, f.editedContent(t), "**Book a respawn**")
}

func TestSearchSubmit_Failure(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("FindRespawn", mock.Anything, "g1", "lib").Return(nil, errors.New("db down"))
	i := modalSubmit(t, "lf1:rqm", "lib")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "db down")
}

func TestEditButton_ShowsTheTimesInPlace(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	picker := timePicker(reservation.TimeChoice{ReservationID: 5, SpotID: 7, StartAt: slotAt, Length: 2 * time.Hour})
	picker.Editing = mine(5)
	f.forms.On("TimePicker", mock.Anything, "g1", isActor("u1", "Knight", true), choiceIs(reservation.TimeChoice{ReservationID: 5, Window: reservation.AutoWindow})).
		Return(picker, nil)
	i := component("lf1:edit:5")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	ids := f.editedCustomIDs(t)
	assert.Contains(t, ids, "lf1:eb:5:7:1791228600:120")
	assert.Contains(t, ids, "lf1:erp:5:1791228600:120:0")
	assert.Contains(t, ids, "lf1:list")
}

func TestEditButton_WorksWithoutTheReserveRank(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
	f.forms.On("TimePicker", mock.Anything, "g1", isActor("u1", "Knight", false), mock.Anything).Return(nil, reservations.ErrForbidden)
	i := component("lf1:edit:5")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then: the core decides
	assert.Contains(t, f.editedContent(t), "only change your own")
	assert.Equal(t, []string{"lf1:list"}, f.editedCustomIDs(t))
}

func TestEditSave_ShowsTheList(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	draft := reservation.Draft{SpotID: 7, StartAt: slotAt, EndAt: slotAt.Add(2 * time.Hour)}
	f.forms.On("EditChoice", mock.Anything, "g1", mock.Anything, choiceIs(reservation.TimeChoice{ReservationID: 5, SpotID: 7, StartAt: slotAt, Length: 2 * time.Hour})).
		Return(&reservation.FormOutcome{Draft: draft, SpotName: "Library -1"}, nil)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{Items: []*reservation.ReservationWithSpot{mine(5)}, Total: 1}, nil)
	i := component("lf1:eb:5:7:1791228600:120")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "Updated **Library -1**")
}

func TestEditSave_ConflictKeepsTheChoice(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	draft := reservation.Draft{SpotID: 7, StartAt: slotAt, EndAt: slotAt.Add(2 * time.Hour)}
	outcome := &reservation.FormOutcome{Draft: draft, SpotName: "Library -1", Conflicts: []*reservation.Reservation{{Author: "Druid"}}}
	f.forms.On("EditChoice", mock.Anything, "g1", mock.Anything, mock.Anything).Return(outcome, booking.ErrConflict)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(nil, errors.New("db down"))
	i := component("lf1:eb:5:7:1791228600:120")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.True(t, strings.HasPrefix(f.editedContent(t), "Not changed. **Library -1**"))
	assert.Contains(t, f.editedCustomIDs(t), "lf1:ew:5:7::1791228600:120")
}

func TestButtons_RefuseANonPremiumServer(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1"}, nil)
	i := component("lf1:rq")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, notPremiumMessage("https://tibialoot.test"), got.Data.Content)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, got.Data.Flags)
}

func TestButtons_RefuseOutsideAServer(t *testing.T) {
	// given
	f := newFormFixture(t)
	i := component("lf1:rq")
	i.GuildID, i.Member, i.User = "", nil, &discordgo.User{ID: "u1"}
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, "Use this in a server.", got.Data.Content)
}

func TestButtons_ReportAConfigOrGuildFailure(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.configs.On("Get", mock.Anything, "g1").Return(nil, errors.New("db down")).Once()
	f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1", Premium: true}, nil).Once()
	f.gateway.On("StateGuild", "g1").Return(nil, discordgo.ErrStateNotFound).Once()
	f.gateway.On("Guild", "g1").Return(nil, errors.New("discord down")).Once()
	first, second := component("lf1:rq"), component("lf1:rq")
	gotFirst := f.expectRespond(first)
	gotSecond := f.expectRespond(second)

	// when
	f.b.InteractionCreate(nil, first)
	f.b.InteractionCreate(nil, second)

	// then
	assert.Contains(t, gotFirst.Data.Content, "server settings")
	assert.Contains(t, gotSecond.Data.Content, "could not load the server")
}

func TestButtons_AskTheAPIForAGuildMissingFromTheState(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1", Premium: true}, nil)
	f.gateway.On("StateGuild", "g1").Return(nil, discordgo.ErrStateNotFound).Once()
	f.gateway.On("Guild", "g1").Return(&discordgo.Guild{ID: "g1", OwnerID: "owner"}, nil).Once()
	i := component("lf1:rq")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseModal, got.Type)
}

func TestOutdatedButton(t *testing.T) {
	// given
	f := newFormFixture(t)
	i := component("lf0:something")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, got.Data.Content, "out of date")
}

func TestInteractionCreate_IgnoresOtherTypes(t *testing.T) {
	// given
	f := newFormFixture(t)

	// when
	f.b.InteractionCreate(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{Type: discordgo.InteractionPing}})

	// then: no call to the gateway
}

func TestMyReservationsButton_SendsAnEphemeralList(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Mine", mock.Anything, "g1", isActor("u1", "Knight", true), listLimit).
		Return(&reservation.Page{Items: []*reservation.ReservationWithSpot{mine(5)}, Total: 1}, nil)
	i := component("lf1:mine")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredChannelMessageWithSource, got.Type)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, got.Data.Flags)
	assert.Contains(t, f.editedContent(t), "1. **Library -1**")
	assert.Contains(t, f.editedCustomIDs(t), "lf1:edit:5")
}

func TestMyReservationsButton_ListFailure(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(nil, errors.New("db down"))
	i := component("lf1:list")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "db down")
}

func TestDeferredButton_RefusalReplacesTheMessage(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1"}, nil)
	i := component("lf1:cancel:5")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, notPremiumMessage("https://tibialoot.test"), f.editedContent(t))
}

func TestDeferredButton_StopsWhenTheDeferFails(t *testing.T) {
	// given
	f := newFormFixture(t)
	i := component("lf1:cancel:5")
	f.gateway.On("InteractionRespond", i.Interaction, mock.Anything).Return(errors.New("unknown interaction"))

	// when
	f.b.InteractionCreate(nil, i)

	// then: no edit
}

func TestCancelButton_AsksToConfirm(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Cancellable", mock.Anything, "g1", mock.Anything, int64(5)).Return(mine(5), nil)
	i := component("lf1:cancel:5")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "Cancel **Library -1**")
	assert.Equal(t, []string{"lf1:cancelok:5", "lf1:list"}, f.editedCustomIDs(t))
}

func TestCancelButton_Refused(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Cancellable", mock.Anything, "g1", mock.Anything, int64(5)).Return(nil, reservations.ErrForbidden)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{}, nil)
	i := component("lf1:cancel:5")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.True(t, strings.HasPrefix(f.editedContent(t), "You can only change your own"))
}

func TestConfirmCancel_CancelsAndShowsTheList(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Cancel", mock.Anything, "g1", mock.Anything, int64(5)).Return(mine(5), nil).Once()
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{}, nil)
	i := component("lf1:cancelok:5")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	content := f.editedContent(t)
	assert.Contains(t, content, "Cancelled **Library -1**")
	assert.Contains(t, content, "You have no upcoming reservations.")
}

func TestConfirmCancel_Failure(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Cancel", mock.Anything, "g1", mock.Anything, int64(5)).Return(nil, errors.New("db down"))
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{}, nil)
	i := component("lf1:cancelok:5")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "db down")
}

func TestOverbookButton_BooksWithOverbook(t *testing.T) {
	// given
	f := newFormFixture(t)
	metrics := mocks.NewMockMetricsPort(t)
	f.b.metrics = metrics
	f.premium(guildconfig.Config{})
	start, end := time.Unix(1791226800, 0), time.Unix(1791234000, 0)
	draft := reservation.Draft{SpotID: 7, StartAt: start, EndAt: end, Overbook: true}
	metrics.On("IncOverbook", "g1", "Celesta").Once()
	f.forms.On("Book", mock.Anything, "g1", mock.Anything, draft).
		Return(&reservation.FormOutcome{Draft: draft, SpotName: "Library -1"}, nil)
	i := component("lf1:ob:7:1791226800:1791234000")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "Booked **Library -1**")
}

func TestOverbookButton_Fails(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Book", mock.Anything, "g1", mock.Anything, mock.Anything).Return(nil, booking.ErrQuotaExceeded)
	i := component("lf1:ob:7:1791226800:1791234000")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "at most 3 hours within 24 hours")
	assert.Equal(t, []string{"lf1:ww:7::1791226800:120"}, f.editedCustomIDs(t))
}

func TestOverbookButton_WithoutTheReserveRank(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
	i := component("lf1:ob:7:1791226800:1791234000")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "rank that can book")
}

func TestMyReservationsCommand(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	withDeadline := mock.MatchedBy(func(ctx context.Context) bool {
		_, ok := ctx.Deadline()
		return ok
	})
	f.forms.On("Mine", withDeadline, "g1", mock.Anything, listLimit).Return(&reservation.Page{}, nil)
	i := component("")
	f.expectEdit(i)

	// when
	err := f.b.MyReservations(i)

	// then
	require.NoError(t, err)
	assert.Equal(t, "You have no upcoming reservations.", f.editedContent(t))
}

func TestModalValues_SkipsOtherComponents(t *testing.T) {
	// given
	data := discordgo.ModalSubmitInteractionData{Components: []discordgo.MessageComponent{
		&discordgo.Button{CustomID: "x"},
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.Button{CustomID: "y"}, &discordgo.TextInput{CustomID: inputQuery, Value: "Library"}}},
	}}

	// when
	values := modalValues(data)

	// then
	assert.Equal(t, map[string]string{inputQuery: "Library"}, values)
}

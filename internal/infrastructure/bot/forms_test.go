package bot

import (
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
	f.gateway.On("Guild", "g1").Return(&discordgo.Guild{ID: "g1", Name: "Celesta", OwnerID: "owner"}, nil)
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
func modalSubmit(t *testing.T, customID, spotName, start, end string) *discordgo.InteractionCreate {
	t.Helper()
	payload := map[string]any{
		"id":       "i1",
		"type":     discordgo.InteractionModalSubmit,
		"guild_id": "g1",
		"member":   map[string]any{"nick": "Knight", "user": map[string]any{"id": "u1", "username": "knight"}},
		"data": map[string]any{
			"custom_id": customID,
			"components": []any{
				map[string]any{"type": 1, "components": []any{map[string]any{"type": 4, "custom_id": inputSpot, "value": spotName}}},
				map[string]any{"type": 1, "components": []any{map[string]any{"type": 4, "custom_id": inputStart, "value": start}}},
				map[string]any{"type": 1, "components": []any{map[string]any{"type": 4, "custom_id": inputEnd, "value": end}}},
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

func TestBookButton_OpensTheForm(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	i := component("lf1:book")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseModal, got.Type)
	assert.Equal(t, "lf1:mbook", got.Data.CustomID)
	assert.Len(t, got.Data.Components, 3)
}

func TestRetryButton_FillsTheFormAgain(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	i := component("lf1:retry:1830::Library")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	values := modalValues(discordgo.ModalSubmitInteractionData{Components: pointerRows(t, got.Data.Components)})
	assert.Equal(t, map[string]string{inputSpot: "Library", inputStart: "18:30", inputEnd: ""}, values)
}

// pointerRows turns the rows of a sent modal into the pointer rows of a received one.
func pointerRows(t *testing.T, components []discordgo.MessageComponent) []discordgo.MessageComponent {
	t.Helper()
	out := make([]discordgo.MessageComponent, 0, len(components))
	for _, c := range components {
		r, ok := c.(discordgo.ActionsRow)
		require.True(t, ok)
		inner := make([]discordgo.MessageComponent, 0, len(r.Components))
		for _, in := range r.Components {
			input, ok := in.(discordgo.TextInput)
			require.True(t, ok)
			inner = append(inner, &input)
		}
		out = append(out, &discordgo.ActionsRow{Components: inner})
	}
	return out
}

func TestBookButton_RefusesAMemberWithoutTheReserveRank(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
	i := component("lf1:book")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseChannelMessageWithSource, got.Type)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, got.Data.Flags)
	assert.Contains(t, got.Data.Content, "rank that can book")
}

func TestButtons_RefuseANonPremiumServer(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1"}, nil)
	i := component("lf1:book")
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
	i := component("lf1:book")
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
	f.gateway.On("Guild", "g1").Return(nil, errors.New("discord down")).Once()
	first, second := component("lf1:book"), component("lf1:book")
	gotFirst := f.expectRespond(first)
	gotSecond := f.expectRespond(second)

	// when
	f.b.InteractionCreate(nil, first)
	f.b.InteractionCreate(nil, second)

	// then
	assert.Contains(t, gotFirst.Data.Content, "server settings")
	assert.Contains(t, gotSecond.Data.Content, "could not load the server")
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

func TestEditButton_OpensTheFilledForm(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Editable", mock.Anything, "g1", isActor("u1", "Knight", true), int64(5)).Return(mine(5), nil)
	i := component("lf1:edit:5")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseModal, got.Type)
	assert.Equal(t, "lf1:medit:5", got.Data.CustomID)
	values := modalValues(discordgo.ModalSubmitInteractionData{Components: pointerRows(t, got.Data.Components)})
	assert.Equal(t, map[string]string{inputSpot: "Library -1", inputStart: "19:00", inputEnd: "21:00"}, values)
}

func TestEditRetryButton_FillsTheAttempt(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Editable", mock.Anything, "g1", mock.Anything, int64(5)).Return(mine(5), nil)
	i := component("lf1:eretry:5:2000:2200:Hero Cave")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	values := modalValues(discordgo.ModalSubmitInteractionData{Components: pointerRows(t, got.Data.Components)})
	assert.Equal(t, map[string]string{inputSpot: "Hero Cave", inputStart: "20:00", inputEnd: "22:00"}, values)
}

func TestEditButton_RefusesSomeoneElsesReservation(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Editable", mock.Anything, "g1", mock.Anything, int64(5)).Return(nil, reservations.ErrForbidden)
	i := component("lf1:edit:5")
	got := f.expectRespond(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseChannelMessageWithSource, got.Type)
	assert.Contains(t, got.Data.Content, "only change your own")
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
	i := component("lf1:del:5")
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
	i := component("lf1:del:5")
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
	i := component("lf1:del:5")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "Cancel **Library -1**")
	assert.Equal(t, []string{"lf1:delok:5", "lf1:list"}, f.editedCustomIDs(t))
}

func TestCancelButton_Refused(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Cancellable", mock.Anything, "g1", mock.Anything, int64(5)).Return(nil, reservations.ErrForbidden)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{}, nil)
	i := component("lf1:del:5")
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
	i := component("lf1:delok:5")
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
	i := component("lf1:delok:5")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "db down")
}

func TestBookSubmit_BooksAndRepliesEphemerally(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	form := reservation.Form{Spot: "Library", StartAt: "19:00", EndAt: "21:00"}
	outcome := &reservation.FormOutcome{Draft: reservation.Draft{SpotID: 7, StartAt: formNow.Add(time.Hour), EndAt: formNow.Add(3 * time.Hour)}, SpotName: "Library -1"}
	f.forms.On("BookForm", mock.Anything, "g1", isActor("u1", "Knight", true), form).Return(outcome, nil)
	i := modalSubmit(t, "lf1:mbook", "Library", "19:00", "21:00")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredChannelMessageWithSource, got.Type)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, got.Data.Flags)
	assert.Contains(t, f.editedContent(t), "Booked **Library -1**")
}

func TestBookSubmit_WithoutTheReserveRank(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{ReserveRoleIDs: []string{"r-reserve"}})
	i := modalSubmit(t, "lf1:mbook", "Library", "19:00", "21:00")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "rank that can book")
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

func TestPickSelect_BooksThePickedRespawn(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	draft := reservation.Draft{SpotID: 8, StartAt: time.Unix(1791226800, 0), EndAt: time.Unix(1791234000, 0)}
	conflict := &reservation.FormOutcome{Draft: draft, SpotName: "Library -2", Conflicts: []*reservation.Reservation{{Author: "Druid"}}}
	f.forms.On("Book", mock.Anything, "g1", mock.Anything, draft).Return(conflict, booking.ErrInsufficientPermissions)
	i := component("lf1:pick:1791226800:1791234000", "8")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "overlaps these reservations")
}

func TestPickSelect_WithoutAValue(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	i := component("lf1:pick:1791226800:1791234000")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "out of date")
}

func TestEditPickSelect_EditsAndShowsTheList(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	draft := reservation.Draft{SpotID: 8, StartAt: time.Unix(1791226800, 0), EndAt: time.Unix(1791234000, 0)}
	f.forms.On("Edit", mock.Anything, "g1", mock.Anything, int64(5), draft).Return(&reservation.FormOutcome{Draft: draft, SpotName: "Library -2"}, nil)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{Items: []*reservation.ReservationWithSpot{mine(5)}, Total: 1}, nil)
	i := component("lf1:epick:5:1791226800:1791234000", "8")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "Updated **Library -2**")
}

func TestEditPickSelect_WithoutAValue(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	i := component("lf1:epick:5:1791226800:1791234000", "x")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "out of date")
}

func TestEditSubmit_UpdatesTheListInPlace(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	form := reservation.Form{Spot: "Library -1", StartAt: "19:00", EndAt: "23:00"}
	f.forms.On("EditForm", mock.Anything, "g1", mock.Anything, int64(5), form).Return(nil, booking.ErrReservationTooLong)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{Items: []*reservation.ReservationWithSpot{mine(5)}, Total: 1}, nil)
	i := modalSubmit(t, "lf1:medit:5", "Library -1", "19:00", "23:00")
	got := f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, got.Type)
	assert.Contains(t, f.editedContent(t), "Not changed. A reservation can be at most 3 hours long.")
	assert.Contains(t, f.editedCustomIDs(t), "lf1:eretry:5:1900:2300:Library -1")
}

func TestEditSubmit_ListFailure(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("EditForm", mock.Anything, "g1", mock.Anything, int64(5), mock.Anything).Return(nil, nil)
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(nil, errors.New("db down"))
	i := modalSubmit(t, "lf1:medit:5", "Library -1", "19:00", "21:00")
	f.expectRespond(i)
	f.expectEdit(i)

	// when
	f.b.InteractionCreate(nil, i)

	// then
	assert.Contains(t, f.editedContent(t), "db down")
}

func TestMyReservationsCommand(t *testing.T) {
	// given
	f := newFormFixture(t)
	f.premium(guildconfig.Config{})
	f.forms.On("Mine", mock.Anything, "g1", mock.Anything, listLimit).Return(&reservation.Page{}, nil)
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
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.Button{CustomID: "y"}, &discordgo.TextInput{CustomID: inputSpot, Value: "Library"}}},
	}}

	// when
	values := modalValues(data)

	// then
	assert.Equal(t, map[string]string{inputSpot: "Library"}, values)
}

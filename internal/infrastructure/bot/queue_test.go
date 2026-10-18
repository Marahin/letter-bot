package bot

import (
	"context"
	"errors"
	"slices"
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
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/permission"
	corequeue "spot-assistant/internal/core/queue"
	"spot-assistant/internal/core/reservationforms"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/ports"
)

var offeredAt = time.Unix(1790000000, 0)

type queueFixture struct {
	*formFixture

	queue  *mocks.MockSpotQueueService
	writer *mocks.MockSummaryChannelWriter
	events *mocks.MockAPIPort
}

func newQueueFixture(t *testing.T) *queueFixture {
	f := &queueFixture{
		formFixture: newFormFixture(t),
		queue:       mocks.NewMockSpotQueueService(t),
		writer:      mocks.NewMockSummaryChannelWriter(t),
		events:      mocks.NewMockAPIPort(t),
	}
	f.b.writer = f.writer
	f.b.eventHandler = f.events
	f.b.pacer = &pacer{now: time.Now, sleep: func(context.Context, time.Duration) error { return nil }}
	f.b.WithSpotQueue(f.queue)
	return f
}

func (f *queueFixture) config(cfg guildconfig.Config) {
	cfg.GuildID, cfg.Premium = "g1", true
	f.configs.On("Get", mock.Anything, "g1").Return(&cfg, nil)
}

func waiting(offered *time.Time) queue.Entry {
	return queue.Entry{ID: 9, GuildID: "g1", SpotID: 7, SpotName: "Library -1", UserID: "u1", Length: 2 * time.Hour,
		ExpiresAt: offeredAt.Add(24 * time.Hour), OfferedAt: offered}
}

func ptrTime(t time.Time) *time.Time { return &t }

func (f *queueFixture) recordSend(channelID string) *discordgo.MessageSend {
	sent := &discordgo.MessageSend{}
	f.writer.On("ChannelMessageSendComplex", channelID, mock.Anything).Run(func(args mock.Arguments) {
		*sent = *args.Get(1).(*discordgo.MessageSend)
	}).Return(&discordgo.Message{}, nil).Once()
	return sent
}

func messageCustomIDs(t *testing.T, components []discordgo.MessageComponent) []string {
	t.Helper()
	return customIDs(t, formView{components: components})
}

func TestWithSpotQueue_TurnsOnTheQueue(t *testing.T) {
	// given
	b := &Bot{}
	q := mocks.NewMockSpotQueueService(t)

	// when
	b.WithSpotQueue(q)

	// then
	assert.Same(t, q, b.queue)
	assert.Same(t, q, b.fulfiller)
	assert.Len(t, b.passHooks, 1)
}

func TestSendQueueOffers_SendsADM(t *testing.T) {
	// given
	f := newQueueFixture(t)
	f.config(guildconfig.Config{Name: "Celesta"})
	f.queue.EXPECT().DueOffers(mock.Anything, "g1").Return([]queue.Entry{waiting(ptrTime(offeredAt))}, nil)
	f.writer.On("UserChannelCreate", "u1").Return(&discordgo.Channel{ID: "dm1"}, nil)
	sent := f.recordSend("dm1")

	// when
	err := f.b.sendQueueOffers(context.Background(), "g1")

	// then
	require.NoError(t, err)
	assert.Contains(t, sent.Content, "**Library -1** is free now on **Celesta**")
	assert.NotContains(t, sent.Content, "<@u1>")
	assert.Empty(t, sent.AllowedMentions.Users)
	assert.Equal(t, []string{"lf1:qb:9:1790000000", "lf1:qc:9:1790000000"}, messageCustomIDs(t, sent.Components))
}

func TestSendQueueOffers_ClosedDMsGetAMentionInTheCommandChannel(t *testing.T) {
	// given
	f := newQueueFixture(t)
	f.config(guildconfig.Config{Name: "Celesta", CommandChannelID: "c2"})
	f.gateway.On("StateChannels", "g1").Return([]*discordgo.Channel{{ID: "c2", Name: "bookings"}}, nil)
	f.queue.EXPECT().DueOffers(mock.Anything, "g1").Return([]queue.Entry{waiting(ptrTime(offeredAt))}, nil)
	f.writer.On("UserChannelCreate", "u1").Return(&discordgo.Channel{ID: "dm1"}, nil)
	f.writer.On("ChannelMessageSendComplex", "dm1", mock.Anything).Return(nil, discordErr(discordgo.ErrCodeCannotSendMessagesToThisUser)).Once()
	sent := f.recordSend("c2")

	// when
	err := f.b.sendQueueOffers(context.Background(), "g1")

	// then
	require.NoError(t, err)
	assert.Contains(t, sent.Content, "<@u1> **Library -1** is free now.")
	assert.Equal(t, []string{"u1"}, sent.AllowedMentions.Users)
	assert.Len(t, messageCustomIDs(t, sent.Components), 2)
}

func TestSendQueueOffers_AFailedOfferIsUnclaimed(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *queueFixture)
	}{
		{name: "DM fails", setup: func(f *queueFixture) {
			f.writer.On("UserChannelCreate", "u1").Return(&discordgo.Channel{ID: "dm1"}, nil)
			f.writer.On("ChannelMessageSendComplex", "dm1", mock.Anything).Return(nil, errors.New("discord down"))
		}},
		{name: "DM channel fails", setup: func(f *queueFixture) {
			f.writer.On("UserChannelCreate", "u1").Return(nil, errors.New("discord down"))
		}},
		{name: "DMs closed and no command channel", setup: func(f *queueFixture) {
			f.gateway.On("StateChannels", "g1").Return([]*discordgo.Channel{}, nil)
			f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{}, nil)
			f.writer.On("UserChannelCreate", "u1").Return(&discordgo.Channel{ID: "dm1"}, nil)
			f.writer.On("ChannelMessageSendComplex", "dm1", mock.Anything).Return(nil, discordErr(discordgo.ErrCodeCannotSendMessagesToThisUser))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			f.config(guildconfig.Config{})
			f.queue.EXPECT().DueOffers(mock.Anything, "g1").Return([]queue.Entry{waiting(ptrTime(offeredAt))}, nil)
			tt.setup(f)
			f.queue.EXPECT().Unclaim(mock.Anything, int64(9)).Return(nil)

			// when
			err := f.b.sendQueueOffers(context.Background(), "g1")

			// then
			assert.ErrorContains(t, err, "send offer 9")
		})
	}
}

func TestSendQueueOffers_WithoutTheConfigEveryOfferIsUnclaimed(t *testing.T) {
	// given
	f := newQueueFixture(t)
	f.configs.On("Get", mock.Anything, "g1").Return(nil, errors.New("db down"))
	second := waiting(ptrTime(offeredAt))
	second.ID = 10
	f.queue.EXPECT().DueOffers(mock.Anything, "g1").Return([]queue.Entry{waiting(ptrTime(offeredAt)), second}, nil)
	f.queue.EXPECT().Unclaim(mock.Anything, int64(9)).Return(nil)
	f.queue.EXPECT().Unclaim(mock.Anything, int64(10)).Return(errors.New("db down"))

	// when
	err := f.b.sendQueueOffers(context.Background(), "g1")

	// then
	assert.ErrorContains(t, err, "unclaim offer 10")
}

func TestSendQueueOffers_NothingDue(t *testing.T) {
	// given
	f := newQueueFixture(t)
	f.queue.EXPECT().DueOffers(mock.Anything, "g1").Return(nil, nil).Once()
	f.queue.EXPECT().DueOffers(mock.Anything, "g2").Return(nil, errors.New("db down")).Once()

	// when
	quiet := f.b.sendQueueOffers(context.Background(), "g1")
	failed := f.b.sendQueueOffers(context.Background(), "g2")

	// then
	assert.NoError(t, quiet)
	assert.Error(t, failed)
}

// dmClick is a click in a DM: no guild and no member.
func dmClick(customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:   "i1",
		Type: discordgo.InteractionMessageComponent,
		User: &discordgo.User{ID: "u1", Username: "knight"},
		Data: discordgo.MessageComponentInteractionData{CustomID: customID},
	}}
}

func TestQueueOfferClick_RefusesBeforeDeferring(t *testing.T) {
	for _, tt := range []struct {
		name  string
		entry *queue.Entry
		err   error
		want  string
	}{
		{name: "entry gone", err: ports.ErrNotFound, want: "not in this queue"},
		{name: "lookup fails", err: errors.New("db down"), want: "could not load the queue"},
		{name: "another member", entry: func() *queue.Entry { e := waiting(nil); e.UserID = "u2"; return &e }(), want: "for another member"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			i := dmClick("lf1:qb:9:1790000000")
			f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(tt.entry, tt.err)
			resp := f.expectRespond(i)

			// when
			f.b.handleComponent(i)

			// then
			assert.Equal(t, discordgo.InteractionResponseChannelMessageWithSource, resp.Type)
			assert.Equal(t, discordgo.MessageFlagsEphemeral, resp.Data.Flags)
			assert.Contains(t, resp.Data.Content, tt.want)
		})
	}
}

func TestQueueOfferClick_BookFromADM(t *testing.T) {
	// given
	f := newQueueFixture(t)
	i := dmClick("lf1:qb:9:1790000000")
	e := waiting(ptrTime(offeredAt))
	f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(&e, nil)
	resp := f.expectRespond(i)
	f.queue.EXPECT().CheckOffer(&e, "u1", offeredAt).Return(nil)
	f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1", Premium: true, ManageRoleIDs: []string{"boss"}}, nil)
	f.gateway.On("StateGuild", "g1").Return(&discordgo.Guild{ID: "g1", Roles: []*discordgo.Role{
		{ID: "g1", Permissions: 0}, {ID: "admin", Permissions: discordgo.PermissionAdministrator},
	}}, nil)
	f.gateway.On("GuildMember", "g1", "u1").Return(&discordgo.Member{User: &discordgo.User{ID: "u1", Username: "knight"}, Roles: []string{"admin"}}, nil)
	var actor reservation.Actor
	draft := reservation.Draft{SpotID: 7, StartAt: offeredAt, EndAt: offeredAt.Add(2 * time.Hour)}
	f.queue.EXPECT().Book(mock.Anything, mock.Anything, int64(9), offeredAt).
		Run(func(_ context.Context, a reservation.Actor, _ int64, _ time.Time) { actor = a }).
		Return(&reservation.FormOutcome{Draft: draft, SpotName: "Library -1"}, nil)
	f.expectEdit(i)

	// when
	f.b.handleComponent(i)

	// then
	assert.Equal(t, discordgo.InteractionResponseDeferredMessageUpdate, resp.Type)
	assert.Equal(t, "u1", actor.UserID)
	assert.Equal(t, "knight", actor.Name)
	assert.True(t, actor.Caps.Admin, "the admin role counts without the interaction's permissions")
	assert.Contains(t, f.editedContent(t), "Booked **Library -1**")
	assert.Empty(t, f.editedCustomIDs(t))
}

func TestQueueOfferClick_BookInTheCommandChannelUsesTheClickingMember(t *testing.T) {
	// given
	f := newQueueFixture(t)
	i := component("lf1:qb:9:1790000000")
	e := waiting(ptrTime(offeredAt))
	f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(&e, nil)
	f.expectRespond(i)
	f.queue.EXPECT().CheckOffer(&e, "u1", offeredAt).Return(nil)
	f.premium(guildconfig.Config{})
	f.queue.EXPECT().Book(mock.Anything, mock.MatchedBy(func(a reservation.Actor) bool { return a.Name == "Knight" && a.Caps.Reserve }), int64(9), offeredAt).
		Return(&reservation.FormOutcome{SpotName: "Library -1"}, nil)
	f.expectEdit(i)

	// when
	f.b.handleComponent(i)

	// then
	assert.Contains(t, f.editedContent(t), "Booked **Library -1**")
}

func TestQueueOfferClick_BookRefused(t *testing.T) {
	conflict := &reservation.Reservation{Author: "Druid", StartAt: offeredAt, EndAt: offeredAt.Add(time.Hour)}
	for _, tt := range []struct {
		name       string
		outcome    *reservation.FormOutcome
		err        error
		want       string
		wantCancel bool
	}{
		{name: "taken first", outcome: &reservation.FormOutcome{SpotName: "Library -1", Conflicts: []*reservation.Reservation{conflict}, CanOverbook: true},
			err: booking.ErrInsufficientPermissions, want: "overlaps these reservations", wantCancel: true},
		{name: "quota", err: booking.ErrQuotaExceeded, want: "at most 3 hours", wantCancel: true},
		{name: "expired meanwhile", err: corequeue.ErrOfferExpired, want: "has expired"},
		{name: "left meanwhile", err: ports.ErrNotFound, want: "not in this queue"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			i := component("lf1:qb:9:1790000000")
			e := waiting(ptrTime(offeredAt))
			f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(&e, nil)
			f.expectRespond(i)
			f.queue.EXPECT().CheckOffer(&e, "u1", offeredAt).Return(nil)
			f.premium(guildconfig.Config{})
			f.queue.EXPECT().Book(mock.Anything, mock.Anything, int64(9), offeredAt).Return(tt.outcome, tt.err)
			f.expectEdit(i)

			// when
			f.b.handleComponent(i)

			// then
			content := f.editedContent(t)
			assert.Contains(t, content, tt.want)
			assert.NotContains(t, content, "You can overbook")
			if tt.wantCancel {
				assert.Equal(t, []string{"lf1:qc:9:1790000000"}, f.editedCustomIDs(t))
			} else {
				assert.Empty(t, f.editedCustomIDs(t))
			}
		})
	}
}

func TestQueueOfferClick_ExpiredOfferIsNotBooked(t *testing.T) {
	// given
	f := newQueueFixture(t)
	i := dmClick("lf1:qb:9:1790000000")
	e := waiting(ptrTime(offeredAt))
	f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(&e, nil)
	f.expectRespond(i)
	f.queue.EXPECT().CheckOffer(&e, "u1", offeredAt).Return(corequeue.ErrOfferExpired)
	f.expectEdit(i)

	// when
	f.b.handleComponent(i)

	// then
	assert.Contains(t, f.editedContent(t), "This offer for **Library -1** has expired. You stay in the queue until")
	assert.Empty(t, f.editedCustomIDs(t))
}

func TestQueueOfferClick_ActorRefusals(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *queueFixture)
		want  string
	}{
		{name: "config fails", setup: func(f *queueFixture) {
			f.configs.On("Get", mock.Anything, "g1").Return(nil, errors.New("db down"))
		}, want: "server settings"},
		{name: "not premium", setup: func(f *queueFixture) {
			f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1"}, nil)
		}, want: "does not have premium"},
		{name: "guild unknown", setup: func(f *queueFixture) {
			f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1", Premium: true}, nil)
			f.gateway.On("StateGuild", "g1").Return(nil, discordgo.ErrStateNotFound)
			f.gateway.On("Guild", "g1").Return(nil, errors.New("gone"))
		}, want: "could not load the server"},
		{name: "member left", setup: func(f *queueFixture) {
			f.configs.On("Get", mock.Anything, "g1").Return(&guildconfig.Config{GuildID: "g1", Premium: true}, nil)
			f.gateway.On("StateGuild", "g1").Return(&discordgo.Guild{ID: "g1"}, nil)
			f.gateway.On("GuildMember", "g1", "u1").Return(nil, discordErr(discordgo.ErrCodeUnknownMember))
		}, want: "could not find you on the server"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			i := dmClick("lf1:qb:9:1790000000")
			e := waiting(ptrTime(offeredAt))
			f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(&e, nil)
			f.expectRespond(i)
			f.queue.EXPECT().CheckOffer(&e, "u1", offeredAt).Return(nil)
			tt.setup(f)
			f.expectEdit(i)

			// when
			f.b.handleComponent(i)

			// then
			assert.Contains(t, f.editedContent(t), tt.want)
			assert.Equal(t, []string{"lf1:qc:9:1790000000"}, f.editedCustomIDs(t))
		})
	}
}

func TestQueueOfferClick_Cancel(t *testing.T) {
	for _, tt := range []struct {
		name       string
		err        error
		want       string
		wantCancel bool
	}{
		{name: "left", want: "You left the queue for **Library -1**."},
		{name: "already gone", err: ports.ErrNotFound, want: "You left the queue for **Library -1**."},
		{name: "fails", err: errors.New("db down"), want: "could not leave the queue", wantCancel: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			i := dmClick("lf1:qc:9:1790000000")
			e := waiting(ptrTime(offeredAt))
			f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(&e, nil)
			f.expectRespond(i)
			f.queue.EXPECT().Leave(mock.Anything, "u1", int64(9)).Return(tt.err)
			f.expectEdit(i)

			// when
			f.b.handleComponent(i)

			// then
			assert.Contains(t, f.editedContent(t), tt.want)
			assert.Equal(t, tt.wantCancel, len(f.editedCustomIDs(t)) == 1)
		})
	}
}

func TestQueueJoin_FromTheWizard(t *testing.T) {
	for _, tt := range []struct {
		name    string
		outcome *queue.JoinOutcome
		err     error
		want    string
		wantIDs []string
	}{
		{name: "waits", outcome: &queue.JoinOutcome{Entry: waiting(nil)}, want: "You are in the queue for **Library -1** (2 h)", wantIDs: []string{"lf1:ql:9"}},
		{name: "free now", outcome: &queue.JoinOutcome{Entry: waiting(ptrTime(offeredAt)), FreeNow: true}, want: "is free now. Book it", wantIDs: []string{"lf1:qb:9:1790000000", "lf1:qc:9:1790000000"}},
		{name: "full", err: corequeue.ErrQueueFull, want: "at most 3 respawns"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			f.premium(guildconfig.Config{})
			i := component("lf1:qj:7:60")
			f.expectRespond(i)
			f.queue.EXPECT().Join(mock.Anything, "g1", mock.Anything, int64(7), time.Hour).Return(tt.outcome, tt.err)
			f.expectEdit(i)

			// when
			f.b.handleComponent(i)

			// then
			assert.Contains(t, f.editedContent(t), tt.want)
			assert.Equal(t, tt.wantIDs, f.editedCustomIDs(t))
		})
	}
}

func TestQueueLeave_FromTheJoinReply(t *testing.T) {
	other := waiting(nil)
	other.UserID = "u2"
	mine := waiting(nil)
	for _, tt := range []struct {
		name     string
		entry    *queue.Entry
		entryErr error
		leaveErr error
		leaves   bool
		want     string
	}{
		{name: "left", entry: &mine, leaves: true, want: "You left the queue for **Library -1**."},
		{name: "already gone", entryErr: ports.ErrNotFound, want: "not in this queue"},
		{name: "not theirs", entry: &other, want: "not in this queue"},
		{name: "fails", entry: &mine, leaves: true, leaveErr: errors.New("db down"), want: "db down"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			f.premium(guildconfig.Config{})
			i := component("lf1:ql:9")
			f.expectRespond(i)
			f.queue.EXPECT().Entry(mock.Anything, int64(9)).Return(tt.entry, tt.entryErr)
			if tt.leaves {
				f.queue.EXPECT().Leave(mock.Anything, "u1", int64(9)).Return(tt.leaveErr)
			}
			f.expectEdit(i)

			// when
			f.b.handleComponent(i)

			// then
			assert.Contains(t, f.editedContent(t), tt.want)
		})
	}
}

func slashCommand(name string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:      "i1",
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  knight(),
		Data:    discordgo.ApplicationCommandInteractionData{Name: name, Options: options},
	}}
}

func stringOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value}
}

func TestQueueCommand(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *queueFixture)
		want  string
	}{
		{name: "joins", setup: func(f *queueFixture) {
			f.forms.On("FindRespawn", mock.Anything, "g1", "lib").Return(&spot.Spot{ID: 7, Name: "Library -1"}, nil)
			f.queue.EXPECT().Join(mock.Anything, "g1", mock.Anything, int64(7), 90*time.Minute).Return(&queue.JoinOutcome{Entry: waiting(nil)}, nil)
		}, want: "You are in the queue"},
		{name: "ambiguous", setup: func(f *queueFixture) {
			f.forms.On("FindRespawn", mock.Anything, "g1", "lib").Return(nil, &reservationforms.AmbiguousSpotError{Query: "lib"})
		}, want: `More respawns match "lib"`},
		{name: "unknown", setup: func(f *queueFixture) {
			f.forms.On("FindRespawn", mock.Anything, "g1", "lib").Return(nil, booking.ErrSpotNotFound)
		}, want: "No respawn has this name"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			f.premium(guildconfig.Config{})
			length := &discordgo.ApplicationCommandInteractionDataOption{Name: "length", Type: discordgo.ApplicationCommandOptionInteger, Value: float64(90)}
			i := slashCommand(commandQueue, stringOption("respawn", "lib"), length)
			resp := f.expectRespond(i)
			tt.setup(f)
			f.expectEdit(i)

			// when
			f.b.handleCommand(i)

			// then
			assert.Equal(t, discordgo.MessageFlagsEphemeral, resp.Data.Flags, "the queue is private")
			assert.Contains(t, f.editedContent(t), tt.want)
		})
	}
}

func TestQueueLeaveCommand(t *testing.T) {
	cave := waiting(nil)
	cave.ID, cave.SpotName = 10, "Hero Cave"
	for _, tt := range []struct {
		name   string
		value  string
		mine   []queue.Entry
		leaves int64
		want   string
	}{
		{name: "by id", value: "9", mine: []queue.Entry{waiting(nil), cave}, leaves: 9, want: "You left the queue for **Library -1**."},
		{name: "by name", value: " hero cave ", mine: []queue.Entry{waiting(nil), cave}, leaves: 10, want: "You left the queue for **Hero Cave**."},
		{name: "not queued", value: "Dragons", mine: []queue.Entry{cave}, want: "You wait for: **Hero Cave**."},
		{name: "no queue", value: "Dragons", want: "You are not in any queue."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			f := newQueueFixture(t)
			f.config(guildconfig.Config{})
			i := slashCommand(commandQueueLeave, stringOption("respawn", tt.value))
			resp := f.expectRespond(i)
			f.queue.EXPECT().Mine(mock.Anything, "g1", "u1").Return(tt.mine, nil)
			if tt.leaves != 0 {
				f.queue.EXPECT().Leave(mock.Anything, "u1", tt.leaves).Return(nil)
			}
			f.expectEdit(i)

			// when
			f.b.handleCommand(i)

			// then
			assert.Equal(t, discordgo.MessageFlagsEphemeral, resp.Data.Flags)
			assert.Contains(t, f.editedContent(t), tt.want)
		})
	}
}

func TestQueueLeaveCommand_Failures(t *testing.T) {
	// given
	b := newQueueFixture(t)
	b.queue.EXPECT().Mine(mock.Anything, "g1", "u1").Return(nil, errors.New("db down")).Once()
	b.queue.EXPECT().Mine(mock.Anything, "g2", "u1").Return([]queue.Entry{waiting(nil)}, nil).Once()
	b.queue.EXPECT().Leave(mock.Anything, "u1", int64(9)).Return(errors.New("db gone")).Once()

	// when
	listFailed := b.b.queueLeaveCommand(context.Background(), "g1", "u1", "9")
	leaveFailed := b.b.queueLeaveCommand(context.Background(), "g2", "u1", "9")

	// then
	assert.Contains(t, listFailed, "db down")
	assert.Contains(t, leaveFailed, "db gone")
}

func autocomplete(name string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	i := slashCommand(name, options...)
	i.Type = discordgo.InteractionApplicationCommandAutocomplete
	return i
}

func TestQueueAutocomplete(t *testing.T) {
	// given
	f := newQueueFixture(t)
	f.config(guildconfig.Config{})
	focused := stringOption("respawn", "lib")
	focused.Focused = true
	i := autocomplete(commandQueue, focused)
	f.events.On("OnBookAutocomplete", book.BookAutocompleteRequest{GuildID: "g1", Field: book.BookAutocompleteSpot, Value: "lib"}).
		Return(book.BookAutocompleteResponse{"Library -1"}, nil)
	resp := f.expectRespond(i)

	// when
	f.b.handleCommand(i)

	// then
	require.Len(t, resp.Data.Choices, 1)
	assert.Equal(t, "Library -1", resp.Data.Choices[0].Value)
}

func TestQueueLeaveAutocomplete(t *testing.T) {
	// given
	f := newQueueFixture(t)
	f.config(guildconfig.Config{})
	focused := stringOption("respawn", "HERO")
	focused.Focused = true
	i := autocomplete(commandQueueLeave, focused)
	cave := waiting(nil)
	cave.ID, cave.SpotName = 10, "Hero Cave"
	f.queue.EXPECT().Mine(mock.Anything, "g1", "u1").Return([]queue.Entry{waiting(nil), cave}, nil)
	resp := f.expectRespond(i)

	// when
	f.b.handleCommand(i)

	// then
	assert.Equal(t, []*discordgo.ApplicationCommandOptionChoice{{Name: "Hero Cave (2 h)", Value: "10"}}, resp.Data.Choices)
}

func TestGetCommands_QueueOnlyWithTheQueue(t *testing.T) {
	// given
	without := &Bot{formatter: formatter.NewFormatter(), log: zap.NewNop().Sugar()}
	with := &Bot{formatter: formatter.NewFormatter(), log: zap.NewNop().Sugar()}
	with.WithSpotQueue(mocks.NewMockSpotQueueService(t))

	// when
	names := func(b *Bot) []string {
		commands := b.getCommands()
		out := make([]string, 0, len(commands))
		for _, c := range commands {
			out = append(out, c.Name)
		}
		return out
	}

	// then
	assert.NotContains(t, names(without), commandQueue)
	assert.Contains(t, names(with), commandQueue)
	assert.Contains(t, names(with), commandQueueLeave)
	queueCmd := with.queueCommands()[0]
	lengths := queueCmd.Options[1].Choices
	require.Len(t, lengths, 6)
	assert.Equal(t, "30 min", lengths[0].Name)
	assert.Equal(t, 180, lengths[5].Value)
}

func TestTimeStepView_QueueButton(t *testing.T) {
	taken := timePicker(reservation.TimeChoice{Length: time.Hour})
	taken.TakenNow = true
	editing := timePicker(reservation.TimeChoice{ReservationID: 9, Length: time.Hour})
	editing.Editing, editing.TakenNow = listed(9, "Library -1", viewNow.Add(time.Hour)), true
	withQueue := viewBot()
	withQueue.queue = mocks.NewMockSpotQueueService(t)
	for _, tt := range []struct {
		name   string
		b      *Bot
		picker *reservation.TimePicker
		want   bool
	}{
		{name: "taken now", b: withQueue, picker: taken, want: true},
		{name: "free now", b: withQueue, picker: timePicker(reservation.TimeChoice{Length: time.Hour})},
		{name: "an edit", b: withQueue, picker: editing},
		{name: "no queue", b: viewBot(), picker: taken},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// when
			view := tt.b.timeStepView(tt.picker, "", viewNow)

			// then
			assertDiscordLimits(t, view)
			assert.Equal(t, tt.want, slices.Contains(customIDs(t, view), "lf1:qj:7:60"))
			assert.Equal(t, tt.want, strings.Contains(view.content, "Press Queue"))
		})
	}
}

func TestRolePermissions(t *testing.T) {
	// given
	g := &guild.Guild{ID: "g1", Roles: []*role.Role{
		{ID: "g1", Permissions: discordgo.PermissionViewChannel},
		{ID: "mod", Permissions: discordgo.PermissionManageMessages},
		{ID: "other", Permissions: discordgo.PermissionAdministrator},
		nil,
	}}

	// when
	perms := rolePermissions(g, []string{"mod"})

	// then
	assert.Equal(t, int64(discordgo.PermissionViewChannel|discordgo.PermissionManageMessages), perms)
}

func TestMemberActor(t *testing.T) {
	// given
	cfg := &guildconfig.Config{ReserveRoleIDs: []string{"r"}}

	// when
	actor := memberActor(cfg, &guild.Guild{ID: "g1"}, MapMember(&discordgo.Member{User: &discordgo.User{ID: "u1", Username: "knight"}}))

	// then
	assert.Equal(t, reservation.Actor{UserID: "u1", Name: "knight", Caps: permission.Capabilities{}}, actor)
}

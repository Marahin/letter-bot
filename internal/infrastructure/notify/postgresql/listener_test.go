package postgresql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func newTestListener() *Listener {
	return &Listener{log: zap.NewNop().Sugar(), slowDispatch: defaultSlowDispatch}
}

func TestDispatch_GuildSignals(t *testing.T) {
	// given
	l := newTestListener()
	h := mocks.NewMockNotifyHandler(t)
	ctx := context.Background()
	h.On("OnSummaryRefresh", ctx, "g1").Once()
	h.On("OnGuildResync", ctx, "g2").Once()
	h.On("OnGuildConfig", ctx, "g3").Once()
	h.On("OnOverbooked", ctx, book.BookRequest{Guild: &guild.Guild{ID: "g4"}, Member: &member.Member{}, Overbook: true},
		&reservation.ClippedOrRemovedReservation{Original: &reservation.Reservation{ID: 1}}).Once()

	// when
	l.dispatch(ctx, h, ChannelSummaryRefresh, "g1")
	l.dispatch(ctx, h, ChannelGuildResync, "g2")
	l.dispatch(ctx, h, ChannelGuildConfig, "g3")
	l.dispatch(ctx, h, ChannelOverbooked, `{"guild_id":"g4","original":{"ID":1}}`)

	// then: expectations are asserted on cleanup
}

func TestDispatch_IgnoresEmptyOrMalformedPayloadAndUnknownChannel(t *testing.T) {
	// given
	l := newTestListener()
	h := mocks.NewMockNotifyHandler(t)

	// when
	l.dispatch(context.Background(), h, ChannelSummaryRefresh, "")
	l.dispatch(context.Background(), h, "other_channel", "g1")
	l.dispatch(context.Background(), h, ChannelOverbooked, "{not json")

	// then: the handler is not called
}

func TestDispatch_WarnsOnSlowHandler(t *testing.T) {
	// given
	core, logs := observer.New(zapcore.WarnLevel)
	l := &Listener{log: zap.New(core).Sugar(), slowDispatch: time.Millisecond}
	h := mocks.NewMockNotifyHandler(t)
	h.On("OnGuildResync", context.Background(), "g1").Run(func(_ mock.Arguments) { time.Sleep(5 * time.Millisecond) }).Once()

	// when
	l.dispatch(context.Background(), h, ChannelGuildResync, "g1")

	// then
	assert.Equal(t, 1, logs.FilterMessage("slow notify dispatch").Len())
}

func TestListen_StopsWhenContextIsCancelled(t *testing.T) {
	// given
	l := NewListener("host=127.0.0.1 port=1 user=x dbname=x sslmode=disable connect_timeout=1", zap.NewNop().Sugar())
	l.retryBackoff = time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})

	// when
	go func() {
		l.Listen(ctx, mocks.NewMockNotifyHandler(t))
		close(done)
	}()

	// then
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Listen did not return after the context was cancelled")
	}
}

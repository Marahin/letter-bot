// Package webcomm is the CommunicationService of the web process. The web has no
// Discord session, so it hands the overbooked DM to the bot through NOTIFY.
package webcomm

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	notifypg "spot-assistant/internal/infrastructure/notify/postgresql"
	"spot-assistant/internal/ports"
)

// ErrNotSupported is returned by the summary methods: only the bot posts summaries.
var ErrNotSupported = errors.New("not supported in the web process")

const notifyTimeout = 5 * time.Second

// Adapter implements ports.CommunicationService.
type Adapter struct {
	notifier ports.BotNotifier
	log      *zap.SugaredLogger
}

func New(notifier ports.BotNotifier, log *zap.SugaredLogger) *Adapter {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Adapter{notifier: notifier, log: log}
}

// NotifyOverbookedMember runs in its own goroutine (booking starts it), after the
// request context may be gone, so it uses a fresh one.
func (a *Adapter) NotifyOverbookedMember(request book.BookRequest, res *reservation.ClippedOrRemovedReservation) {
	payload, err := notifypg.NewOverbookedPayload(request, res).Encode()
	if err != nil {
		a.log.Warnw("encode overbooked payload", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	if err := a.notifier.Overbooked(ctx, payload); err != nil {
		a.log.Warnw("notify overbooked member", "error", err)
	}
}

func (a *Adapter) SendGuildSummary(*guild.Guild, *summary.Summary) error {
	return ErrNotSupported
}

func (a *Adapter) SendPrivateSummary(summary.PrivateSummaryRequest, *summary.Summary) error {
	return ErrNotSupported
}

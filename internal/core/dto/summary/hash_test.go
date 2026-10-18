package summary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLedgerHash(t *testing.T) {
	start := time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)
	booking := func() *Booking {
		return &Booking{Author: "Knight", AuthorDiscordID: "u1", Status: Online, StartAt: start, EndAt: start.Add(time.Hour)}
	}
	ledger := func(change func(b *Booking)) Ledger {
		b := booking()
		change(b)
		return Ledger{
			{Spot: "Hero Cave", Bookings: []*Booking{b}},
			{Spot: "Asura Palace", Bookings: []*Booking{{Author: "Druid", StartAt: start, EndAt: start.Add(2 * time.Hour)}}},
		}
	}
	base := ledger(func(*Booking) {}).Hash()

	changes := map[string]func(b *Booking){
		"status": func(b *Booking) { b.Status = Offline },
		"start":  func(b *Booking) { b.StartAt = b.StartAt.Add(time.Minute) },
		"end":    func(b *Booking) { b.EndAt = b.EndAt.Add(time.Minute) },
		"author": func(b *Booking) { b.Author = "Paladin" },
	}
	for name, change := range changes {
		t.Run(name+" changes the hash", func(t *testing.T) {
			// when
			hash := ledger(change).Hash()

			// then
			assert.NotEqual(t, base, hash)
		})
	}

	t.Run("ignores the order and the Discord id", func(t *testing.T) {
		// given
		l := ledger(func(b *Booking) { b.AuthorDiscordID = "" })
		l[0], l[1] = l[1], l[0]

		// when
		hash := l.Hash()

		// then
		assert.Equal(t, base, hash)
	})

	t.Run("an empty ledger differs from a spot without bookings", func(t *testing.T) {
		assert.NotEqual(t, Ledger{}.Hash(), Ledger{{Spot: "Hero Cave"}}.Hash())
	})
}

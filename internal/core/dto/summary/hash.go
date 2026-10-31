package summary

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

// Hash identifies what a summary of the ledger shows. The order of the entries
// and of the bookings does not change it.
func (l Ledger) Hash() string {
	lines := make([]string, 0, len(l))
	for _, entry := range l {
		if len(entry.Bookings) == 0 {
			lines = append(lines, entry.Spot)
		}
		for _, b := range entry.Bookings {
			lines = append(lines, strings.Join([]string{
				entry.Spot,
				strconv.FormatInt(b.StartAt.Unix(), 10),
				strconv.FormatInt(b.EndAt.Unix(), 10),
				b.Author,
				strconv.Itoa(int(b.Status)),
			}, "\x1f"))
		}
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\x1e")))
	return hex.EncodeToString(sum[:])
}

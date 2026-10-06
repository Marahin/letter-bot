package lootcalc

import "sort"

// Transfer is one bank transfer that evens the party out.
type Transfer struct {
	From   string
	To     string
	Amount int64
}

// Result is the even split of a session. PerHead is floored, so up to
// Players-1 gold stays with the players who give.
type Result struct {
	Transfers []Transfer
	Total     int64
	PerHead   int64
	Players   int
}

type share struct {
	name      string
	available int64
	needed    int64
}

// Split computes the transfers exactly as the original calculate() does: every
// player above the per-head share gives, the biggest giver first, to every
// player at or below it, in paste order.
func Split(s Session) Result {
	r := Result{Players: len(s.Players)}
	if r.Players == 0 {
		return r
	}
	for _, p := range s.Players {
		r.Total += p.Balance
	}
	r.PerHead = floorDiv(r.Total, int64(r.Players))

	shares := make([]*share, 0, r.Players)
	for _, p := range s.Players {
		sh := &share{name: p.Name}
		if p.Balance > r.PerHead {
			sh.available = p.Balance - r.PerHead
		} else {
			sh.needed = r.PerHead - p.Balance
		}
		shares = append(shares, sh)
	}
	sort.SliceStable(shares, func(i, j int) bool { return shares[i].available > shares[j].available })

	var givers, takers []*share
	for _, sh := range shares {
		if sh.available > 0 {
			givers = append(givers, sh)
		}
		if sh.needed > 0 {
			takers = append(takers, sh)
		}
	}
	for _, g := range givers {
		for _, t := range takers {
			amount := min(g.available, t.needed)
			g.available -= amount
			t.needed -= amount
			if amount > 0 {
				r.Transfers = append(r.Transfers, Transfer{From: g.name, To: t.name, Amount: amount})
			}
		}
	}
	return r
}

// floorDiv rounds toward negative infinity, as Math.floor does; Go's / truncates.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

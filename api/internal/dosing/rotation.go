package dosing

import "time"

// Injection is all the rotation needs to know about a logged dose. Narrow on purpose, the way
// protocol.LoggedSlot is: this function is pure, and a dose event carries a dozen fields it
// has no business reading.
type Injection struct {
	// Absent when the wizard's zone step was skipped, and for an item that has no zone.
	Site *Site

	// When the injection happened, never when the row was written.
	At time.Time
}

// SuggestNextSite is the zone to offer next, least recently used.
//
// The prototype freezes it as a constant; here it is a function of what was logged — an unused
// zone wins, and among used ones the one whose *latest* injection is oldest. Recency is the
// timestamp and never the position in `recent`.
func SuggestNextSite(recent []Injection) Site {
	lastUsed := make(map[Site]time.Time, len(recent))
	for _, injection := range recent {
		if injection.Site == nil {
			continue
		}
		if at, used := lastUsed[*injection.Site]; !used || at.Before(injection.At) {
			lastUsed[*injection.Site] = injection.At
		}
	}

	// Presence in the map and not a zero time as «never used»: the zero Time is a real
	// instant, so the two would be one answer.
	best := Sites()[0]
	bestAt, bestUsed := lastUsed[best]

	for _, site := range Sites()[1:] {
		at, used := lastUsed[site]
		switch {
		case !bestUsed:
			// Nothing later can beat an unused zone; ties keep the set's order.
			return best
		case !used:
			best, bestAt, bestUsed = site, at, false
		case at.Before(bestAt):
			best, bestAt, bestUsed = site, at, true
		}
	}

	return best
}

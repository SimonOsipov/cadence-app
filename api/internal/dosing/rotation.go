package dosing

import "time"

// Injection is all the rotation needs to know about a logged dose. Narrow on purpose, the way
// protocol.LoggedSlot is: this function is pure, and a dose event carries a dozen fields it
// has no business reading.
type Injection struct {
	// Absent for a dose logged with the zone step skipped, and for an oral item that has
	// no zone at all. Also where an unrecognised site_code lands, refused at the boundary
	// rather than crashing a screen.
	Site *Site

	// When the injection happened, which is not when the row was written: a dose from the
	// retry queue lands hours later, and a back-fill answers an occurrence months old.
	At time.Time
}

// SuggestNextSite is the zone to offer next, least recently used.
//
// The prototype does not compute this: INITIAL_LOG_STATE carries `suggested` and `lastUsed`
// as two frozen constants, so its suggestion never moves. Here it is a function of what was
// logged — an unused zone wins, and among used ones the zone whose *latest* injection is
// oldest wins, because the tissue does not care that it was also used in April.
//
// Recency is the timestamp and never the position in `recent`: a repository list carries
// whatever order the query left.
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

	// Presence in the map rather than a zero time as «never used»: the zero Time is a
	// real instant that a back-dated row could carry, and comparing against it would make
	// «injected in year one» and «never injected» the same answer.
	best := Sites()[0]
	bestAt, bestUsed := lastUsed[best]

	for _, site := range Sites()[1:] {
		at, used := lastUsed[site]
		switch {
		case !bestUsed:
			// The first unused zone in the set's order already wins outright, and
			// nothing later can beat it — ties keep the set's order.
			return best
		case !used:
			best, bestAt, bestUsed = site, at, false
		case at.Before(bestAt):
			best, bestAt, bestUsed = site, at, true
		}
	}

	return best
}

package protocol

import (
	"slices"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

const daysPerWeek = 7

// OccurrenceStatus is computed by comparing generated occurrences against logged events,
// never stored — §03's L10 and «nothing derived is stored».
type OccurrenceStatus string

const (
	StatusDone      OccurrenceStatus = "done"
	StatusPending   OccurrenceStatus = "pending"
	StatusMissed    OccurrenceStatus = "missed"
	StatusScheduled OccurrenceStatus = "scheduled"
)

// Occurrence is keyed by time as well as date: §03's `times[]` makes BPC-157 at 08:00 and
// 20:00 two occurrences, and logging one doesn't log the other.
type Occurrence struct {
	ItemID ProtocolItemID
	Kind   ItemKind
	Date   civil.Date
	Time   civil.Slot
	Dose   *Dose
	Status OccurrenceStatus
}

// CycleWeek counts week 1 as the seven days from the protocol's start; false means the date
// lies outside the course altogether.
func CycleWeek(p Protocol, d civil.Date) (int, bool) {
	offset := p.StartDate.DaysUntil(d)
	if offset < 0 {
		return 0, false
	}
	week := offset/daysPerWeek + 1
	if week > p.Weeks {
		return 0, false
	}
	return week, true
}

// OccurrencesFor is the whole schedule: §03 has no materialized schedule table, so Today and
// Schedule render the same call. `today` is a parameter rather than a clock reading, so this
// can be tested against a calendar.
func OccurrencesFor(plan Plan, logged []LoggedSlot, d, today civil.Date) []Occurrence {
	// CANCELLED only, and the narrowing matters: a COMPLETED course is every patient after
	// twelve weeks, and blanking it would erase the dots from days they actually injected.
	// The three read endpoints do not reach this branch today — ActivePlanFor selects the
	// active course, so a finished one is answered as no course at all. That is the step's
	// recorded divergence, not this function's rule.
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}
	if _, ok := CycleWeek(plan.Protocol, d); !ok {
		return nil
	}

	var out []Occurrence
	for _, item := range plan.Items {
		if !fallsOn(item, d) {
			continue
		}
		dose := PhaseDose(plan, item.ID, d)
		for _, at := range item.Times {
			// A copy per slot: Kotlin's Dose is immutable and could be shared, a *Dose
			// cannot — one pointer across an item's slots ties BPC-157's morning and
			// evening occurrences to each other for anyone who writes through it.
			var own *Dose
			if dose != nil {
				value := *dose
				own = &value
			}
			out = append(out, Occurrence{
				ItemID: item.ID,
				Kind:   item.Kind,
				Date:   d,
				Time:   at,
				Dose:   own,
				Status: statusOf(item, d, at, logged, today),
			})
		}
	}
	return out
}

// PhaseDose returns nil for three causes with one answer: cancelled, outside the window, or
// no phase covering that week.
//
// It reads the phases unsorted while DoseBands and TitrationSteps sort them, so the two agree
// only while phases do not overlap — faithful to the Kotlin, and held by 000013's EXCLUDE
// rather than by anything here.
func PhaseDose(plan Plan, itemID ProtocolItemID, d civil.Date) *Dose {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}
	week, ok := CycleWeek(plan.Protocol, d)
	if !ok {
		return nil
	}
	for _, phase := range plan.Phases[itemID] {
		if phase.Covers(week) {
			dose := phase.Dose
			return &dose
		}
	}
	return nil
}

// CurrentDoseFor is the dose the course prescribes of a compound on a day, for callers that
// hold a drug rather than an item — the cabinet, whose vials name compounds.
//
// Nothing where two items name one compound, by the precedent OpenVialFor sets: 000013 has no
// unique index on the pair, so a course can carry an injection and a supplement of one drug,
// and answering either dose would put a number on a screen that half the prescription
// contradicts. An item that names the compound and carries no phase falls through to
// PhaseDose's own nil — Draft.Check requires phases of injections only, so a supplement
// naming a drug is a legal course.
func CurrentDoseFor(plan Plan, compound CompoundID, d civil.Date) *Dose {
	item := SoleItemFor(plan, compound)
	if item == nil {
		return nil
	}

	return PhaseDose(plan, item.ID, d)
}

// SoleItemFor is the one course position prescribing a compound, and nothing where two do.
//
// The ambiguity rule itself, so the readings that need the position rather than the dose —
// the cabinet's reorder hint divides by an item's own weekly rate — refuse on the same terms
// instead of carrying a second copy of it.
func SoleItemFor(plan Plan, compound CompoundID) *ProtocolItem {
	var sole *ProtocolItem
	for i, item := range plan.Items {
		if item.CompoundID == nil || *item.CompoundID != compound {
			continue
		}
		if sole != nil {
			return nil
		}
		sole = &plan.Items[i]
	}

	return sole
}

// DosesPerWeek is derived, not seeded: a stored copy goes stale the first time a protocol is
// edited.
func DosesPerWeek(item ProtocolItem) float64 {
	return scheduledDaysPerWeek(item) * float64(len(item.Times))
}

// Shared with fallsOn, so the two readings of `cadence` cannot drift apart.
func scheduledDaysPerWeek(item ProtocolItem) float64 {
	if item.Cadence == CadenceDaily {
		return daysPerWeek
	}
	return float64(len(item.DaysOfWeek))
}

func fallsOn(item ProtocolItem, d civil.Date) bool {
	if item.Cadence == CadenceDaily {
		return true
	}
	return slices.Contains(item.DaysOfWeek, d.Weekday())
}

// The match must include item, date and slot: matching on item alone would mark every
// remaining Sunday done after the first injection; without the slot, one BPC-157 event with
// a null time closed both 08:00 and 20:00.
func statusOf(item ProtocolItem, d civil.Date, at civil.Slot, logged []LoggedSlot, today civil.Date) OccurrenceStatus {
	for _, s := range logged {
		if s.ItemID == item.ID && s.Date == d && s.Time != nil && *s.Time == at {
			return StatusDone
		}
	}

	switch {
	case d.Before(today):
		return StatusMissed
	case d == today:
		return StatusPending
	default:
		return StatusScheduled
	}
}

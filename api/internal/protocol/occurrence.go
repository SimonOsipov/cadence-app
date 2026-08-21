package protocol

import "slices"

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
	Date   Date
	Time   Slot
	Dose   *Dose
	Status OccurrenceStatus
}

// CycleWeek counts week 1 as the seven days from the protocol's start; false means the date
// lies outside the course altogether.
func CycleWeek(p Protocol, d Date) (int, bool) {
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
func OccurrencesFor(plan Plan, logged []LoggedSlot, d, today Date) []Occurrence {
	// CANCELLED only, and the narrowing matters: a COMPLETED course is every patient after
	// twelve weeks, and blanking it would erase the dots from days they actually injected.
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
			out = append(out, Occurrence{
				ItemID: item.ID,
				Kind:   item.Kind,
				Date:   d,
				Time:   at,
				Dose:   dose,
				Status: statusOf(item, d, at, logged, today),
			})
		}
	}
	return out
}

// PhaseDose returns nil for three causes with one answer: cancelled, outside the window, or
// no phase covering that week. Phases are read in the order they arrive — DoseBands and
// TitrationSteps are the sorted readers, and this one deliberately is not.
func PhaseDose(plan Plan, itemID ProtocolItemID, d Date) *Dose {
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

func fallsOn(item ProtocolItem, d Date) bool {
	if item.Cadence == CadenceDaily {
		return true
	}
	return slices.Contains(item.DaysOfWeek, d.Weekday())
}

// The match must include item, date and slot: matching on item alone would mark every
// remaining Sunday done after the first injection; without the slot, one BPC-157 event with
// a null time closed both 08:00 and 20:00.
func statusOf(item ProtocolItem, d Date, at Slot, logged []LoggedSlot, today Date) OccurrenceStatus {
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

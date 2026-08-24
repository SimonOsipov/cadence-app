package protocol

import "github.com/SimonOsipov/cadence-app/api/internal/platform/civil"

// DoseBand is a prescription, not a history: no logged dose reaches this file. A band
// vanishing because a week was missed would tell a patient their protocol changed when it
// hadn't. civil.Range is already clipped to the window it was asked about.
type DoseBand struct {
	Dose  Dose
	Range civil.Range
}

type ProtocolMarkKind string

const (
	MarkStart     ProtocolMarkKind = "start"
	MarkTitration ProtocolMarkKind = "titration"
)

// ProtocolMark carries a nil From on MarkStart: there is no dose to have come up from.
type ProtocolMark struct {
	Kind ProtocolMarkKind
	Date civil.Date
	From *Dose
	To   Dose
}

// DoseBands builds from the phases, not from occurrences: a band is a phase — a first week,
// a last week and a dose. Phases are clipped to the protocol's own last day as well as to r,
// because §03 leaves `to_week` and `weeks` unjoined and week 20 of a twelve-week course is
// representable.
func DoseBands(plan Plan, itemID ProtocolItemID, r civil.Range) []DoseBand {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}

	var out []DoseBand
	for _, phase := range sortedPhases(plan, itemID) {
		// Clipped at both ends of the course, not just the far one: §03 leaves from_week as
		// unjoined as to_week, and WeekStart(0) is a week before the protocol began. PhaseDose
		// answers nil there, so an unclipped band drew a dose over a day the schedule says was
		// never prescribed. The Kotlin clips only the far end and has the same disagreement.
		opens := civil.MaxDate(plan.Protocol.WeekStart(phase.FromWeek), plan.Protocol.StartDate)
		// The last day of ToWeek is the day before ToWeek + 1 opens.
		closes := civil.MinDate(plan.Protocol.WeekStart(phase.ToWeek+1).AddDays(-1), plan.Protocol.LastPrescribedDay())
		from := civil.MaxDate(opens, r.From)
		through := civil.MinDate(closes, r.Through)
		if from.After(through) {
			continue
		}
		out = append(out, DoseBand{Dose: phase.Dose, Range: civil.Range{From: from, Through: through}})
	}
	return out
}

// ProtocolMarks marks a dose that changed, not one that rose: §03 lets a course taper as
// readily as it titrates, and a mark firing only upwards would leave a reduction
// unexplained. The start sits on the first day of the first phase — the protocol's own start
// when dosed from day one, the phase-opening day when the item is introduced mid-course.
func ProtocolMarks(plan Plan, itemID ProtocolItemID, r civil.Range) []ProtocolMark {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}
	phases := sortedPhases(plan, itemID)
	if len(phases) == 0 {
		return nil
	}

	// The first band that survives the course, not the first phase's nominal opening: they
	// differ for a phase opening before day 0, which DoseBands clips rather than drops. The
	// window is the course and never r — a mark must not slide onto the asked-for edge.
	course := civil.Range{From: plan.Protocol.StartDate, Through: plan.Protocol.LastPrescribedDay()}
	bands := DoseBands(plan, itemID, course)
	if len(bands) == 0 {
		return nil
	}

	marks := []ProtocolMark{{
		Kind: MarkStart,
		Date: bands[0].Range.From,
		To:   bands[0].Dose,
	}}
	for _, step := range TitrationSteps(plan, itemID) {
		from := step.From
		marks = append(marks, ProtocolMark{Kind: MarkTitration, Date: step.Date, From: &from, To: step.To})
	}

	first, last := plan.Protocol.StartDate, plan.Protocol.LastPrescribedDay()

	var out []ProtocolMark
	for _, m := range marks {
		if !m.Date.Before(first) && !m.Date.After(last) && r.Contains(m.Date) {
			out = append(out, m)
		}
	}
	return out
}

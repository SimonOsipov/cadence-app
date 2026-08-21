package protocol

// DoseBand is a prescription, not a history: no logged dose reaches this file. A band
// vanishing because a week was missed would tell a patient their protocol changed when it
// hadn't. Range is already clipped to the window it was asked about.
type DoseBand struct {
	Dose  Dose
	Range Range
}

type ProtocolMarkKind string

const (
	MarkStart     ProtocolMarkKind = "start"
	MarkTitration ProtocolMarkKind = "titration"
)

// ProtocolMark carries a nil From on MarkStart: there is no dose to have come up from.
type ProtocolMark struct {
	Kind ProtocolMarkKind
	Date Date
	From *Dose
	To   Dose
}

// DoseBands builds from the phases, not from occurrences: a band is a phase — a first week,
// a last week and a dose. Phases are clipped to the protocol's own last day as well as to r,
// because §03 leaves `to_week` and `weeks` unjoined and week 20 of a twelve-week course is
// representable.
func DoseBands(plan Plan, itemID ProtocolItemID, r Range) []DoseBand {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}

	var out []DoseBand
	for _, phase := range sortedPhases(plan, itemID) {
		// Clipped at both ends of the course, not just the far one: §03 leaves from_week as
		// unjoined as to_week, and WeekStart(0) is a week before the protocol began. PhaseDose
		// answers nil there, so an unclipped band drew a dose over a day the schedule says was
		// never prescribed. The Kotlin clips only the far end and has the same disagreement.
		opens := MaxDate(plan.Protocol.WeekStart(phase.FromWeek), plan.Protocol.StartDate)
		// The last day of ToWeek is the day before ToWeek + 1 opens.
		closes := MinDate(plan.Protocol.WeekStart(phase.ToWeek+1).AddDays(-1), plan.Protocol.LastPrescribedDay())
		from := MaxDate(opens, r.From)
		through := MinDate(closes, r.Through)
		if from.After(through) {
			continue
		}
		out = append(out, DoseBand{Dose: phase.Dose, Range: Range{From: from, Through: through}})
	}
	return out
}

// ProtocolMarks marks a dose that changed, not one that rose: §03 lets a course taper as
// readily as it titrates, and a mark firing only upwards would leave a reduction
// unexplained. The start sits on the first day of the first phase — the protocol's own start
// when dosed from day one, the phase-opening day when the item is introduced mid-course.
func ProtocolMarks(plan Plan, itemID ProtocolItemID, r Range) []ProtocolMark {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}
	phases := sortedPhases(plan, itemID)
	if len(phases) == 0 {
		return nil
	}

	marks := []ProtocolMark{{
		Kind: MarkStart,
		Date: plan.Protocol.WeekStart(phases[0].FromWeek),
		To:   phases[0].Dose,
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

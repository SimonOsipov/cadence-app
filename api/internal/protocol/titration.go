package protocol

import (
	"slices"
	"sort"
)

// TitrationStep is «Доза растёт: 0,25 мг → 0,5 мг», and the day it happens.
type TitrationStep struct {
	Week int
	From Dose
	To   Dose
	Date Date
}

// TitrationSteps derives the doses and the dates from the phases and the protocol's start
// rather than from a frozen constant. A boundary between two phases holding the same dose is
// not a step — §03 allows holding a dose. Steps past the last prescribed day are dropped
// here, so the bands, the marks and «следующий шаг» all inherit one clip.
func TitrationSteps(plan Plan, itemID ProtocolItemID) []TitrationStep {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}
	phases := sortedPhases(plan, itemID)

	var out []TitrationStep
	for i := 1; i < len(phases); i++ {
		from, to := phases[i-1], phases[i]
		if from.Dose == to.Dose {
			continue
		}
		date := plan.Protocol.WeekStart(to.FromWeek)
		if date.After(plan.Protocol.LastPrescribedDay()) {
			continue
		}
		out = append(out, TitrationStep{Week: to.FromWeek, From: from.Dose, To: to.Dose, Date: date})
	}
	return out
}

// TitrationStepAfter is bounded by the cycle, not the calendar: a date outside the
// protocol's window has no «next step», the same way it has no cycle week.
func TitrationStepAfter(plan Plan, itemID ProtocolItemID, today Date) *TitrationStep {
	week, ok := CycleWeek(plan.Protocol, today)
	if !ok {
		return nil
	}
	for _, step := range TitrationSteps(plan, itemID) {
		if step.Week > week {
			return &step
		}
	}
	return nil
}

// The caller's slice is never reordered: a plan is shared between readers, and PhaseDose
// answers from the order it was given.
func sortedPhases(plan Plan, itemID ProtocolItemID) []ProtocolPhase {
	phases := slices.Clone(plan.Phases[itemID])
	sort.SliceStable(phases, func(i, j int) bool { return phases[i].FromWeek < phases[j].FromWeek })
	return phases
}

package protocol

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// Today is the hero screen's whole answer.
//
// The fields of contexts that are not built answer null explicitly rather than zero: «0 из 4
// приёмов» over a nutrition context that does not exist is a lie a client cannot detect,
// while an absent value is a fact it can render as a dash. That is a divergence from the KMP
// type, where mealCount and the two Macros are not nullable — phase 4 adapts the client, and
// the divergence register of step 12 carries it.
type Today struct {
	Date      civil.Date
	PartOfDay PartOfDay

	// Absent outside the course and for a cancelled one: «Неделя 4» over an empty
	// calendar says less than nothing.
	CycleWeek *int

	NextDose         *Occurrence
	NextDoseCompound *Compound
	SuggestedSite    string
	WeekProtocol     []ProtocolRow
	DoseLoggedToday  bool

	VialDosesLeft *int
	Reorder       *ReorderHint
	NextTitration *TitrationStep

	// Nutrition and measurements. Absent, every one, until their contexts are built.
	MealCount      *int
	MealMacros     *Macros
	Targets        *Macros
	WeightKG       *float64
	WeightSeries   []float64
	TargetWeightKG *float64
}

// What this context needs from its neighbours, declared here because the consumer owns the
// interface — and because it must: dosing and inventory both import protocol, so protocol
// importing them back would be a cycle, and the guard in this package's own suite refuses it.
//
// Narrow on purpose. The rotation answers a zone, the cabinet answers two numbers; neither
// hands protocol a dose event or a vial, so neither can drift into protocol reading a
// neighbour's table.
type (
	// Rotation is the injection-site suggestion, computed from what the patient logged.
	Rotation interface {
		SuggestNextSite(ctx context.Context, tx pgx.Tx, patient civil.UserID) (string, error)
	}

	// Cabinet answers what the medicine cabinet knows about one prescribed item: how many
	// doses its open vial has left, and whether it is time to reorder.
	// The item and not the compound, because the reorder hint is «weeks of supply» and
	// the weeks come from the item's own rate. The dose is passed in rather than looked
	// up: the cabinet would have to import this package to find it, and the caller is
	// already holding the plan it comes from — and the caller resolves it from the drug,
	// so it is nil where no phase covers the day and nil where two positions name the
	// drug, which leaves no single dose to divide by. On a nil dose the cabinet answers
	// substance and status, and neither a count of injections nor a hint.
	Cabinet interface {
		SupplyFor(
			ctx context.Context, tx pgx.Tx, patient civil.UserID, item ProtocolItem,
			dose *Dose, today civil.Date,
		) (*int, *ReorderHint, error)
	}

	// Doses is the history the schedule reads: which occurrence was closed, and when.
	Doses interface {
		LoggedSlotsIn(
			ctx context.Context, tx pgx.Tx, patient civil.UserID, window civil.Range,
		) ([]LoggedSlot, error)
	}
)

// ReorderHint and Macros are protocol's copies of shapes their own contexts own, for the same
// reason the interfaces above are here: the aggregate has to name them and cannot import them.
// Both are inert — no behaviour, no invariant — so the copy costs a field list and not a rule.
type (
	ReorderHint struct {
		CompoundID CompoundID
		WeeksLeft  int
	}

	Macros struct {
		Kcal    int
		Protein int
		Carbs   int
		Fat     int
	}
)

func earlier(a, b civil.Slot) bool {
	if a.Hour != b.Hour {
		return a.Hour < b.Hour
	}

	return a.Minute < b.Minute
}

// doseOfTheDrug is what the course prescribes of this item's drug today, and nothing where the
// item names none — which is what SupplyFor answers about anyway.
func doseOfTheDrug(plan Plan, item ProtocolItem, at civil.Date) *Dose {
	if item.CompoundID == nil {
		return nil
	}

	return CurrentDoseFor(plan, *item.CompoundID, at)
}

// TodayFor assembles the hero screen from the plan, the day and what the neighbours answer.
//
// Every question it asks them is asked once, and the order is the reading order of the
// screen rather than of the tables: nothing here depends on anything else here, so a reader
// following the struct's fields finds them in the same sequence.
func TodayFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID,
	plan Plan, compounds []Compound, running bool, at civil.Date, clock civil.Slot,
	doses Doses, rotation Rotation, cabinet Cabinet,
) (Today, error) {
	today := Today{Date: at, PartOfDay: PartOfDayAt(clock)}

	site, err := rotation.SuggestNextSite(ctx, tx, patient)
	if err != nil {
		return Today{}, err
	}
	today.SuggestedSite = site

	if !running {
		// A patient with no course still has a screen: the day, the part of it and the
		// zone the wizard would open on. Everything else is a fact about a course.
		return today, nil
	}

	if week, inCourse := CycleWeek(plan.Protocol, at); inCourse {
		today.CycleWeek = &week
	}

	// One day of history, because that is all either reader needs: the strip is the
	// week's prescription with today's state on it, and the occurrence walk below is
	// about today. A week of slots would be read and then filtered to the same day.
	oneDay, ok := civil.NewRange(at, at)
	if !ok {
		return Today{}, fmt.Errorf("the day %v: %w", at, ErrNotADay)
	}
	logged, err := doses.LoggedSlotsIn(ctx, tx, patient, oneDay)
	if err != nil {
		return Today{}, err
	}

	today.WeekProtocol = WeekProtocolRows(plan, compounds, logged, at)

	// By the clock and not by the order the items came back in. Two injectables on one
	// day is what this product is for, and items are read ORDER BY id — so «the first
	// open one» offered whichever uuid sorted first, which could be the evening dose
	// while the morning one was still open.
	for _, occurrence := range OccurrencesFor(plan, logged, at, at) {
		if occurrence.Kind != KindInjection {
			continue
		}
		if occurrence.Status == StatusDone {
			today.DoseLoggedToday = true

			continue
		}
		if today.NextDose == nil || earlier(occurrence.Time, today.NextDose.Time) {
			next := occurrence
			today.NextDose = &next
		}
	}

	if today.NextDose != nil {
		for _, item := range plan.Items {
			if item.ID != today.NextDose.ItemID {
				continue
			}
			// The dose the cabinet divides by is resolved from the drug and not from
			// this position: with two positions of one compound the shelf answers
			// nothing about it, and a day card dividing by one of the two doses would
			// contradict the cabinet screen about the same vial. Identical wherever the
			// drug has one position, which is every course but the ambiguous one.
			left, reorder, err := cabinet.SupplyFor(ctx, tx, patient, item, doseOfTheDrug(plan, item, at), at)
			if err != nil {
				return Today{}, err
			}
			today.VialDosesLeft, today.Reorder = left, reorder
			today.NextTitration = TitrationStepAfter(plan, item.ID, at)

			// What the hero card names above the dose. Absent when the item names no
			// drug, which an injection cannot — but the read is a join and a join can
			// come back short, and a nil here says so rather than inventing a name.
			today.NextDoseCompound = compoundOf(compounds, item.CompoundID)
		}
	}

	return today, nil
}

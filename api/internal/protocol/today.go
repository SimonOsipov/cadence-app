package protocol

import (
	"context"

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

	// Cabinet answers what the medicine cabinet knows about one compound: how many doses
	// the open vial has left, and whether it is time to reorder.
	Cabinet interface {
		DosesLeftOf(
			ctx context.Context, tx pgx.Tx, patient civil.UserID, compound CompoundID,
			today civil.Date,
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
		DosesLeft  int
		WeeksLeft  float64
		ByDate     civil.Date
		CompoundID CompoundID
	}

	Macros struct {
		Kcal    int
		Protein int
		Carbs   int
		Fat     int
	}
)

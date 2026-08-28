package protocol

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

func aPlan(edit func(*Draft)) Draft {
	compound := CompoundID("9b2f3b7c-0000-4000-8000-0000000000d1")
	draft := Draft{
		PatientID: "9b2f3b7c-0000-4000-8000-0000000000a1",
		StartDate: civil.NewDate(2026, time.May, 4),
		Weeks:     12,
		Status:    StatusActive,
		Items: []DraftItem{{
			Kind:       KindInjection,
			Compound:   CompoundRef{ID: &compound},
			Cadence:    CadenceWeekly,
			DaysOfWeek: []time.Weekday{time.Sunday},
			Times:      []civil.Slot{{Hour: 8}},
			Loggable:   true,
			Phases: []ProtocolPhase{
				{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
				{FromWeek: 5, ToWeek: 12, Dose: Dose{Value: 0.5, Unit: MG}},
			},
		}},
	}
	if edit != nil {
		edit(&draft)
	}

	return draft
}

// Each rule the schema holds, asked here first so the doctor is told which row and which
// field, rather than being handed a 23514 that names a constraint. The database stays the
// authority — it is the guard against the race Go cannot see — and this is the message.
func TestEachRuleTheSchemaHoldsIsAskedHereFirst(t *testing.T) {
	for _, refused := range []struct {
		name string
		edit func(*Draft)
		want error
	}{
		{
			"a course of no weeks",
			func(d *Draft) { d.Weeks = 0 }, ErrWeeksOffRange,
		},
		{
			"a course longer than two years",
			func(d *Draft) { d.Weeks = 105 }, ErrWeeksOffRange,
		},
		{
			"a start date the calendar does not have",
			func(d *Draft) { d.StartDate = civil.Date{Year: 2026, Month: time.February, Day: 30} },
			ErrNotADay,
		},
		{
			"a status off the set",
			func(d *Draft) { d.Status = "paused" }, ErrUnknownStatus,
		},
		{
			"a course with no items",
			func(d *Draft) { d.Items = nil }, ErrNoItems,
		},
		{
			"an item of an unknown kind",
			func(d *Draft) { d.Items[0].Kind = "infusion" }, ErrUnknownKind,
		},
		{
			"an item on an unknown cadence",
			func(d *Draft) { d.Items[0].Cadence = "monthly" }, ErrUnknownCadence,
		},
		{
			// The two are read together by the generator, so they are refused
			// together: a daily item naming weekdays makes fallsOn and DosesPerWeek
			// disagree about one row.
			"a daily item that also names weekdays",
			func(d *Draft) { d.Items[0].Cadence = CadenceDaily }, ErrCadenceAgainstDays,
		},
		{
			"a weekly item that names no weekday",
			func(d *Draft) { d.Items[0].DaysOfWeek = nil }, ErrCadenceAgainstDays,
		},
		{
			"an item with no slot",
			func(d *Draft) { d.Items[0].Times = nil }, ErrNoSlot,
		},
		{
			// Unreachable over HTTP, where the pattern guards it, and guarded here
			// for the same reason the identifier's shape is: cmd/seed builds these
			// by hand, and «99:00» reaches the column as an unclassified 22007.
			"an hour the clock does not have",
			func(d *Draft) { d.Items[0].Times = []civil.Slot{{Hour: 99}} }, ErrNotASlot,
		},
		{
			"a minute the clock does not have",
			func(d *Draft) { d.Items[0].Times = []civil.Slot{{Hour: 8, Minute: 60}} }, ErrNotASlot,
		},
		{
			"an injection naming no compound",
			func(d *Draft) { d.Items[0].Compound = CompoundRef{} }, ErrInjectionWithoutCompound,
		},
		{
			"an injection naming a compound twice over",
			func(d *Draft) {
				d.Items[0].Compound.New = &NewCompound{
					NameRU: "Семаглутид", DefaultUnit: MG, Route: "sc", Icon: "syringe",
				}
			},
			ErrInjectionWithoutCompound,
		},
		{
			"a drug entered by name with no unit",
			func(d *Draft) {
				d.Items[0].Compound = CompoundRef{New: &NewCompound{
					NameRU: "Тирзепатид", Route: "sc", Icon: "syringe",
				}}
			},
			ErrDrugNotDescribed,
		},
		{
			"a drug entered by name with no route",
			func(d *Draft) {
				d.Items[0].Compound = CompoundRef{New: &NewCompound{
					NameRU: "Тирзепатид", DefaultUnit: MG, Icon: "syringe",
				}}
			},
			ErrDrugNotDescribed,
		},
		{
			"a drug entered by name with no icon",
			func(d *Draft) {
				d.Items[0].Compound = CompoundRef{New: &NewCompound{
					NameRU: "Тирзепатид", DefaultUnit: MG, Route: "sc",
				}}
			},
			ErrDrugNotDescribed,
		},
		{
			"a weigh-in that names a drug",
			func(d *Draft) { d.Items[0].Kind = KindWeighIn }, ErrCompoundOnAKindWithoutOne,
		},
		{
			"a phase that runs backwards",
			func(d *Draft) { d.Items[0].Phases[0] = ProtocolPhase{FromWeek: 4, ToWeek: 1} },
			ErrPhaseRunsBackwards,
		},
		{
			"a phase opening before the course",
			func(d *Draft) { d.Items[0].Phases[0].FromWeek = 0 }, ErrPhaseOffCourse,
		},
		{
			"a phase running past the course",
			func(d *Draft) { d.Items[0].Phases[1].ToWeek = 13 }, ErrPhaseOffCourse,
		},
		{
			"a dose of nothing",
			func(d *Draft) { d.Items[0].Phases[0].Dose.Value = 0 }, ErrDoseOffRange,
		},
		{
			// The atom differs by unit: three decimals of a milligram are whole
			// micrograms, and a microgram has no decimals to give.
			"a dose finer than the microgram it is counted in",
			func(d *Draft) { d.Items[0].Phases[0].Dose = Dose{Value: 0.0001, Unit: MG} },
			ErrDoseTooFine,
		},
		{
			"a microgram dose with a tail",
			func(d *Draft) { d.Items[0].Phases[0].Dose = Dose{Value: 250.5, Unit: MCG} },
			ErrDoseTooFine,
		},
		{
			// The other end of the range: 1e19 мг prints with no decimal point at all,
			// so the scale bound takes it, and the microgram count then saturates or
			// wraps depending on the machine that reads the row.
			"a dose past a gram",
			func(d *Draft) { d.Items[0].Phases[0].Dose = Dose{Value: 1e19, Unit: MG} },
			ErrDoseOffRange,
		},
		{
			// The boundary itself, because 1e19 clamps the constant from nowhere: with
			// only that case the ceiling could be a thousand times higher and this suite
			// would not notice, and Go would be looser than the schema — which answers
			// 23514 with no entry in this package's map, so a 500 about the form.
			"a dose one milligram past a gram",
			func(d *Draft) { d.Items[0].Phases[0].Dose = Dose{Value: 1001, Unit: MG} },
			ErrDoseOffRange,
		},
		{
			"a microgram dose past a gram",
			func(d *Draft) { d.Items[0].Phases[0].Dose = Dose{Value: 1_000_001, Unit: MCG} },
			ErrDoseOffRange,
		},
		{
			// Unreachable over HTTP, where the decoder has no literal for it, and
			// guarded for the reason the clock's hours are: cmd/seed builds drafts by
			// hand, and a NaN loses every comparison — the column would then hold a
			// value Postgres orders above every number.
			"a dose that is not a number",
			func(d *Draft) { d.Items[0].Phases[0].Dose = Dose{Value: math.NaN(), Unit: MG} },
			ErrDoseOffRange,
		},
		{
			"a dose in a unit nobody prescribes",
			func(d *Draft) { d.Items[0].Phases[0].Dose.Unit = "ме" }, ErrUnknownDoseUnit,
		},
		{
			// The database holds this one with EXCLUDE USING gist, and it stays the
			// authority — this is the message, naming which two rows.
			"phases that overlap",
			func(d *Draft) { d.Items[0].Phases[1].FromWeek = 4 }, ErrPhasesOverlap,
		},
		{
			"an item with no phase",
			func(d *Draft) { d.Items[0].Phases = nil }, ErrNoPhases,
		},
	} {
		t.Run(refused.name, func(t *testing.T) {
			err := aPlan(refused.edit).Check()
			if !errors.Is(err, refused.want) {
				t.Errorf("got %v, want %v", err, refused.want)
			}
			// The row, not merely the rule: a message that names the field without
			// saying which of twelve items carries it costs the doctor the edit.
			//
			// «item N» and nothing else. The first version also accepted the word
			// «course», which five sentinels carry in their own text, so it could
			// not fail — including against a message naming no row at all. The four
			// course-level refusals below are the ones with no row to name.
			aboutTheCourse := errors.Is(err, ErrWeeksOffRange) || errors.Is(err, ErrNotADay) ||
				errors.Is(err, ErrUnknownStatus) || errors.Is(err, ErrNoItems)
			if !aboutTheCourse && !strings.Contains(err.Error(), "item 1") {
				t.Errorf("the refusal does not say which item: %q", err)
			}
		})
	}
}

// The accept side, and the case §03 makes legal that a coverage check would refuse.
func TestACourseWithGapsBetweenItsPhasesIsLegal(t *testing.T) {
	// BROKEN_PHASES in the KMP suite: a pause between bands is a deliberate washout,
	// and only overlap is forbidden. Weeks 5..8 are prescribed nothing.
	gapped := aPlan(func(d *Draft) {
		d.Items[0].Phases = []ProtocolPhase{
			{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
			{FromWeek: 9, ToWeek: 12, Dose: Dose{Value: 0.5, Unit: MG}},
		}
	})
	if err := gapped.Check(); err != nil {
		t.Errorf("a washout between phases was refused: %v", err)
	}

	if err := aPlan(nil).Check(); err != nil {
		t.Errorf("a plain course was refused: %v", err)
	}

	// A legal Russian name long enough to pass 200 bytes and not 200 characters, which
	// is the only input that tells the two counts apart: 150 Cyrillic letters is 300
	// bytes, the schema takes it, and a byte bound here would refuse a drug the clinic
	// is allowed to enter.
	long := aPlan(func(d *Draft) {
		d.Items[0].Compound = CompoundRef{New: &NewCompound{
			NameRU: strings.Repeat("я", 150), DefaultUnit: MG, Route: "sc", Icon: "syringe",
		}}
	})
	if err := long.Check(); err != nil {
		t.Errorf("a 150-character name was refused: %v", err)
	}

	// The kinds that are not injections, which need no compound and are not logged.
	for _, kind := range []ItemKind{KindSupplement, KindWeighIn} {
		plain := aPlan(func(d *Draft) {
			d.Items[0].Kind = kind
			d.Items[0].Compound = CompoundRef{}
			d.Items[0].Loggable = false
		})
		if err := plain.Check(); err != nil {
			t.Errorf("a %s item was refused: %v", kind, err)
		}
	}
}

// The accept side of the two bounds, which the refusals above cannot supply: a rule that
// refused every dose would pass all of them. The first three are the drift cases a bound
// read off value × 1000 refuses and the schema takes; then the atom's own edges — three
// decimals of a milligram, a whole microgram — and the ceiling's, a gram in either unit.
func TestEveryDoseTheSchemaAdmitsIsAccepted(t *testing.T) {
	for _, dose := range []Dose{
		{Value: 2.01, Unit: MG},
		{Value: 1.005, Unit: MG},
		{Value: 16.1, Unit: MG},
		{Value: 0.001, Unit: MG},
		{Value: 250, Unit: MCG},
		{Value: 1000, Unit: MG},
		{Value: 1_000_000, Unit: MCG},
	} {
		t.Run(fmt.Sprintf("%v %s", dose.Value, dose.Unit), func(t *testing.T) {
			accepted := aPlan(func(d *Draft) { d.Items[0].Phases[0].Dose = dose })

			if err := accepted.Check(); err != nil {
				t.Errorf("a phase of %v %s was refused: %v", dose.Value, dose.Unit, err)
			}
		})
	}
}

// A phase is a dose band, and two of the three kinds have no dose to band.
//
// The rule «at least one phase» was this package's own — protocol_phases is a table an
// item may have no rows in — and it made two shapes of the design unprescribable. The
// supplement carries none on purpose, and inventing a dose for a weigh-in would put a
// number on a screen nobody prescribed. An injection keeps the rule.
func TestOnlyAnInjectionHasToBeDosed(t *testing.T) {
	for _, kind := range []ItemKind{KindSupplement, KindWeighIn} {
		t.Run(string(kind), func(t *testing.T) {
			draft := aPlan(func(d *Draft) {
				d.Items[0].Kind = kind
				d.Items[0].Compound = CompoundRef{}
				d.Items[0].Phases = nil
			})

			if err := draft.Check(); err != nil {
				t.Errorf("a %s with no dose is refused: %v", kind, err)
			}
		})
	}

	t.Run("an injection still is", func(t *testing.T) {
		err := aPlan(func(d *Draft) { d.Items[0].Phases = nil }).Check()
		if !errors.Is(err, ErrNoPhases) {
			t.Errorf("an undosed injection got %v, want %v", err, ErrNoPhases)
		}
	})
}

// A supplement the clinic does dose is still a supplement: relaxing the rule is
// «may have none», not «may not have any».
func TestASupplementMayCarryADoseAnyway(t *testing.T) {
	draft := aPlan(func(d *Draft) {
		d.Items[0].Kind = KindSupplement
		d.Items[0].Compound = CompoundRef{}
	})

	if err := draft.Check(); err != nil {
		t.Errorf("a dosed supplement is refused: %v", err)
	}
}

// A supplement may name a drug, and the design's own does.
//
// «Глицин + магний» is a supplement with a compound in MockSeed, and the glyph and the
// name are that compound's fields; refusing it made a row the screens draw unprescribable.
// A weigh-in still may not — a scale prescribes nothing.
func TestASupplementMayNameADrugAndAWeighInMayNot(t *testing.T) {
	compound := CompoundID("9b2f3b7c-0000-4000-8000-0000000000d1")

	t.Run("a supplement with a drug", func(t *testing.T) {
		draft := aPlan(func(d *Draft) {
			d.Items[0].Kind = KindSupplement
			d.Items[0].Phases = nil
		})

		if err := draft.Check(); err != nil {
			t.Errorf("a supplement naming a drug is refused: %v", err)
		}
	})

	t.Run("a supplement without one", func(t *testing.T) {
		draft := aPlan(func(d *Draft) {
			d.Items[0].Kind = KindSupplement
			d.Items[0].Compound = CompoundRef{}
			d.Items[0].Phases = nil
		})

		if err := draft.Check(); err != nil {
			t.Errorf("a supplement naming no drug is refused: %v", err)
		}
	})

	t.Run("a weigh-in with a drug", func(t *testing.T) {
		draft := aPlan(func(d *Draft) {
			d.Items[0].Kind = KindWeighIn
			d.Items[0].Compound = CompoundRef{ID: &compound}
			d.Items[0].Phases = nil
		})

		if err := draft.Check(); !errors.Is(err, ErrCompoundOnAKindWithoutOne) {
			t.Errorf("a weigh-in naming a drug got %v, want %v", err, ErrCompoundOnAKindWithoutOne)
		}
	})
}

package protocol

import (
	"errors"
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

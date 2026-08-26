package protocol

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// Duplicated from the schema deliberately: a CHECK refuses with a constraint name and no row,
// which reaches a doctor as «23514» over a form of twelve items, while this says which item
// and which field in time for a 422. The database stays the authority for what it carries —
// it holds the race Go cannot see — and shapeRefusals() plus its integration pair keep the two
// from drifting. Some of these the schema cannot express at all; that test is what says which.
var (
	ErrWeeksOffRange            = errors.New("a course runs between one and 104 weeks")
	ErrNotADay                  = errors.New("the calendar has no such day")
	ErrUnknownStatus            = errors.New("the status is not one of active, completed, cancelled")
	ErrNoItems                  = errors.New("a course prescribes something")
	ErrUnknownKind              = errors.New("the kind is not one of injection, supplement, weigh_in")
	ErrUnknownCadence           = errors.New("the cadence is not one of weekly, daily, n_per_week")
	ErrCadenceAgainstDays       = errors.New("a daily item names no weekday, and any other names at least one")
	ErrNoSlot                   = errors.New("an item is taken at a named time")
	ErrNotASlot                 = errors.New("the clock has no such time")
	ErrInjectionWithoutCompound = errors.New("an injection names what is injected")
	ErrNoPhases                 = errors.New("an injection is dosed by at least one phase")
	ErrPhaseRunsBackwards       = errors.New("a phase ends no earlier than it begins")
	ErrPhaseOffCourse           = errors.New("a phase lies inside the weeks of its course")
	ErrDoseOffRange             = errors.New("a dose is more than nothing")
	ErrUnknownDoseUnit          = errors.New("the unit is not one of мг, мкг")
	ErrPhasesOverlap            = errors.New("two phases of one item cover the same week")

	// The mirror of the rule above, and it is not pedantry: a weigh-in carrying a drug
	// is a form the doctor filled in for the wrong row, and accepting it would store a
	// prescription nobody can see on any screen.
	ErrCompoundOnAKindWithoutOne = errors.New("a weigh-in names no drug")

	// The three the directory needs and no default can supply — a unit guessed here is a
	// dose rendered wrong on every screen. Refused with the others, because reaching the
	// INSERT with them empty produced a 23514 that classify does not know and answer
	// turns into a 500 about the doctor's own form.
	ErrDrugNotDescribed = errors.New("a drug entered by name carries a unit, a route and an icon")

	// Two items naming one row is a prescription line silently dropped: both writes land
	// on the same row, the second wins, the reply carries the identifier twice and the
	// doctor is told the course has two items. An editor's «duplicate this row» button
	// that copies the row object with its id produces exactly this.
	ErrItemNamedTwice = errors.New("two items name the same row")

	// The directory's own lengths, mirrored because a name past its bound reaches the
	// INSERT, comes back 23514 and falls through to a 500 about the doctor's own form.
	ErrDrugNameTooLong = errors.New("a drug's name, route and icon each have a bound")
)

// shapeRefusals is every rule Check can break, and the transport maps the list rather than a
// naming convention: an error added above and not added here reaches the doctor as a 500
// about their own form.
func shapeRefusals() []error {
	return []error{
		ErrWeeksOffRange, ErrNotADay, ErrUnknownStatus, ErrNoItems, ErrUnknownKind,
		ErrUnknownCadence, ErrCadenceAgainstDays, ErrNoSlot, ErrInjectionWithoutCompound,
		ErrCompoundOnAKindWithoutOne, ErrNoPhases, ErrPhaseRunsBackwards, ErrPhaseOffCourse,
		ErrDoseOffRange, ErrUnknownDoseUnit, ErrPhasesOverlap, ErrCompoundRefUnclear,
		ErrDrugNotDescribed, ErrItemNamedTwice, ErrDrugNameTooLong, ErrNotASlot,
	}
}

// Draft is a course as the transport received it, before any of it is a row.
type Draft struct {
	PatientID civil.UserID
	StartDate civil.Date
	Weeks     int
	Status    ProtocolStatus
	Notes     *string
	Items     []DraftItem
}

// DraftItem carries its phases: they arrive nested and are written in one transaction, and a
// pair of parallel slices is where the fourth item gets the third item's doses.
type DraftItem struct {
	// The row this item already is, when the course is being edited. Absent for one the
	// doctor has just added — and absent throughout a Create, where there is nothing to
	// keep. It is what makes titration during a course possible: an item somebody has
	// injected is kept and rewritten rather than dropped and re-made.
	ID *ProtocolItemID

	Kind ItemKind
	// What is injected, named by identifier or by name. Empty for the kinds that are
	// not drugs — a weigh-in is not a prescription of anything.
	Compound   CompoundRef
	Cadence    Cadence
	DaysOfWeek []time.Weekday
	Times      []civil.Slot
	Loggable   bool
	Phases     []ProtocolPhase
}

// Check refuses everything refusable without the database, so a malformed course never takes
// a connection and never spends a transaction.
func (d Draft) Check() error {
	if d.Weeks < 1 || d.Weeks > maxCourseWeeks {
		return fmt.Errorf("the course runs %d weeks: %w", d.Weeks, ErrWeeksOffRange)
	}
	if !d.StartDate.Valid() {
		return fmt.Errorf("the course starts %v: %w", d.StartDate, ErrNotADay)
	}
	if _, ok := ParseStatus(string(d.Status)); !ok {
		return fmt.Errorf("the course is %q: %w", d.Status, ErrUnknownStatus)
	}
	if len(d.Items) == 0 {
		return fmt.Errorf("the course prescribes nothing: %w", ErrNoItems)
	}

	named := make(map[ProtocolItemID]int, len(d.Items))
	for i, item := range d.Items {
		if err := item.check(d.Weeks); err != nil {
			return fmt.Errorf("item %d: %w", i+1, err)
		}
		if item.ID == nil {
			continue
		}
		if first, twice := named[*item.ID]; twice {
			return fmt.Errorf("items %d and %d name %s: %w", first, i+1, *item.ID, ErrItemNamedTwice)
		}
		named[*item.ID] = i + 1
	}

	return nil
}

func (i DraftItem) check(weeks int) error {
	if _, ok := ParseKind(string(i.Kind)); !ok {
		return fmt.Errorf("kind %q: %w", i.Kind, ErrUnknownKind)
	}
	if _, ok := ParseCadence(string(i.Cadence)); !ok {
		return fmt.Errorf("cadence %q: %w", i.Cadence, ErrUnknownCadence)
	}
	if (i.Cadence == CadenceDaily) != (len(i.DaysOfWeek) == 0) {
		return fmt.Errorf("cadence %q with %d weekday(s): %w",
			i.Cadence, len(i.DaysOfWeek), ErrCadenceAgainstDays)
	}
	if len(i.Times) == 0 {
		return ErrNoSlot
	}
	for n, slot := range i.Times {
		if !slot.Valid() {
			return fmt.Errorf("slot %d is %v: %w", n+1, slot, ErrNotASlot)
		}
	}
	named := 0
	if i.Compound.ID != nil {
		named++
	}
	if i.Compound.New != nil {
		named++
	}
	switch {
	case i.Kind == KindInjection && named != 1:
		return fmt.Errorf("%d halves named: %w", named, ErrInjectionWithoutCompound)
	// A supplement may: «Глицин + магний» is where the strip's moon glyph and its
	// name come from. A weigh-in may not — a scale prescribes nothing.
	case i.Kind == KindWeighIn && named != 0:
		return fmt.Errorf("a %s names a drug: %w", i.Kind, ErrCompoundOnAKindWithoutOne)
	case i.Kind == KindSupplement && named > 1:
		return fmt.Errorf("%d halves named: %w", named, ErrCompoundRefUnclear)
	}
	if i.Compound.New != nil {
		if _, ok := ParseDoseUnit(string(i.Compound.New.DefaultUnit)); !ok {
			return fmt.Errorf("the drug's unit is %q: %w", i.Compound.New.DefaultUnit, ErrDrugNotDescribed)
		}
		if i.Compound.New.Route == "" || i.Compound.New.Icon == "" {
			return fmt.Errorf("the drug names route %q and icon %q: %w",
				i.Compound.New.Route, i.Compound.New.Icon, ErrDrugNotDescribed)
		}
		// Counted in runes: the names are Russian, PostgreSQL's length() counts
		// characters and Go's len() counts bytes — «Семаглутид» is ten of one and
		// twenty of the other, so a byte bound would refuse half the directory.
		if utf8.RuneCountInString(i.Compound.New.NameRU) > maxDrugName ||
			utf8.RuneCountInString(i.Compound.New.Route) > maxDrugField ||
			utf8.RuneCountInString(i.Compound.New.Icon) > maxDrugField {
			return fmt.Errorf("the drug's name is %d characters: %w",
				utf8.RuneCountInString(i.Compound.New.NameRU), ErrDrugNameTooLong)
		}
	}
	// A phase is a dose band, and the other two kinds have none to band. The schema
	// never held this rule — protocol_phases is a table an item may have no rows in.
	if i.Kind == KindInjection && len(i.Phases) == 0 {
		return ErrNoPhases
	}

	return i.checkPhases(weeks)
}

// checkPhases refuses overlap and says which two phases. Gaps are legal and deliberately
// unchecked: BROKEN_PHASES in the KMP suite is a washout between bands, and a coverage check
// here would refuse a course the product prescribes.
func (i DraftItem) checkPhases(weeks int) error {
	for n, phase := range i.Phases {
		if phase.ToWeek < phase.FromWeek {
			return fmt.Errorf("phase %d covers weeks %d..%d: %w",
				n+1, phase.FromWeek, phase.ToWeek, ErrPhaseRunsBackwards)
		}
		if phase.FromWeek < 1 || phase.ToWeek > weeks {
			return fmt.Errorf("phase %d covers weeks %d..%d of a %d-week course: %w",
				n+1, phase.FromWeek, phase.ToWeek, weeks, ErrPhaseOffCourse)
		}
		if phase.Dose.Value <= 0 || finerThanItsAtom(phase.Dose) {
			return fmt.Errorf("phase %d doses %v: %w", n+1, phase.Dose.Value, ErrDoseOffRange)
		}
		if _, ok := ParseDoseUnit(string(phase.Dose.Unit)); !ok {
			return fmt.Errorf("phase %d in %q: %w", n+1, phase.Dose.Unit, ErrUnknownDoseUnit)
		}
	}

	// Sorted by opening week and compared with the neighbour, rather than every pair
	// against every other: the message has to name two phases, and adjacent ones in
	// this order are the pair a doctor sees as adjacent on the form.
	order := make([]int, len(i.Phases))
	for n := range order {
		order[n] = n
	}
	slices.SortStableFunc(order, func(a, b int) int { return i.Phases[a].FromWeek - i.Phases[b].FromWeek })

	for n := 1; n < len(order); n++ {
		earlier, later := i.Phases[order[n-1]], i.Phases[order[n]]
		if later.FromWeek <= earlier.ToWeek {
			return fmt.Errorf("phases %d and %d both cover week %d: %w",
				order[n-1]+1, order[n]+1, later.FromWeek, ErrPhasesOverlap)
		}
	}

	return nil
}

// The schema's own bounds, each reconciled against its CHECK by test.
const (
	maxCourseWeeks = 104
	maxDrugName    = 200
	maxDrugField   = 50
)

// finerThanItsAtom is a dose the cabinet cannot divide by.
//
// Remaining substance is counted in whole micrograms, so a milligram value past three
// decimals and a microgram value with any decimals at all lose their tail on the way in.
// Measured on the unbounded column: 0,0001 мг converts to nothing, and the day card
// stops answering how many injections are left rather than answering wrongly.
func finerThanItsAtom(dose Dose) bool {
	scaled := dose.Value * 1000
	if dose.Unit == MCG {
		scaled = dose.Value
	}

	return scaled != math.Trunc(scaled)
}

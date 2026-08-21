package protocol

import "time"

// The typed ids of §03: eleven contexts hang off the same handful of foreign keys, and a
// parameter list of strings is where a patient id ends up in a vial id's place with
// everything still compiling.
type (
	UserID         string
	CompoundID     string
	ProtocolID     string
	ProtocolItemID string
)

type DoseUnit string

const (
	MG  DoseUnit = "мг"
	MCG DoseUnit = "мкг"
)

// Dose is a number and a unit; «0,25 мг» is a rendering, not data.
//
// float64 is safe only because doses are never summed. The one place a total looks likely is
// a vial's remaining amount, and §03's third correction makes that a subtraction of counts
// rather than of milligrams — see InventoryMath.kt in the KMP tree, which this step does not
// port. Two doses are compared with == in TitrationSteps, so a value that arrived through
// arithmetic rather than from the wire makes «0,3 мг → 0,3 мг» a titration step; that is
// pinned by TestADoseThatArrivedThroughArithmeticReadsAsAChangeOfDose. Adding a total means
// changing this type.
type Dose struct {
	Value float64
	Unit  DoseUnit
}

type Compound struct {
	ID          CompoundID
	Code        string
	NameRU      string
	DefaultUnit DoseUnit
	Route       string
	// Data, not presentation: which glyph a compound gets is the clinic's choice.
	Icon string
}

type ProtocolStatus string

const (
	StatusActive    ProtocolStatus = "active"
	StatusCompleted ProtocolStatus = "completed"
	StatusCancelled ProtocolStatus = "cancelled"
)

type ItemKind string

const (
	KindInjection  ItemKind = "injection"
	KindSupplement ItemKind = "supplement"
	KindWeighIn    ItemKind = "weigh_in"
)

type Cadence string

const (
	CadenceWeekly   Cadence = "weekly"
	CadenceDaily    Cadence = "daily"
	CadenceNPerWeek Cadence = "n_per_week"
)

// Protocol is §03's `protocols`. StartDate is what the cycle week derives from — the
// prototype's hardcoded «week 4» against a frozen date is what it replaces.
type Protocol struct {
	ID        ProtocolID
	PatientID UserID
	StartDate Date
	Weeks     int
	Status    ProtocolStatus
	CreatedBy *UserID
	Notes     *string
}

// WeekStart is the one place this arithmetic lives: three copies of «(N − 1) × 7» were
// three chances to disagree.
func (p Protocol) WeekStart(week int) Date {
	return p.StartDate.AddDays((week - 1) * daysPerWeek)
}

// LastPrescribedDay is day `weeks × 7 − 1`.
func (p Protocol) LastPrescribedDay() Date {
	return p.StartDate.AddDays(p.Weeks*daysPerWeek - 1)
}

// ProtocolItem is §03's `protocol_items`. Times is a list because §03 makes it one: BPC-157
// is 08:00 and 20:00, which is why an occurrence is keyed (item, date, time).
type ProtocolItem struct {
	ID         ProtocolItemID
	ProtocolID ProtocolID
	Kind       ItemKind
	CompoundID *CompoundID
	Cadence    Cadence
	DaysOfWeek []time.Weekday
	Times      []Slot
	Loggable   bool
}

// ProtocolPhase is the titration band: 0,25 (w1–4) → 0,5 (w5–8) → 1,0 (w9–12).
type ProtocolPhase struct {
	FromWeek int
	ToWeek   int
	Dose     Dose
}

func (p ProtocolPhase) Covers(week int) bool { return week >= p.FromWeek && week <= p.ToWeek }

// Plan is one type rather than three parameters that always travel together: they come from
// one fetch and are meaningless apart.
type Plan struct {
	Protocol Protocol
	Items    []ProtocolItem
	Phases   map[ProtocolItemID][]ProtocolPhase
}

// LoggedSlot is all the generator needs to know about a logged dose. Narrow on purpose: the
// import runs one way, and protocol never learns what a dose event is.
type LoggedSlot struct {
	ItemID ProtocolItemID
	Date   Date
	Time   *Slot
}

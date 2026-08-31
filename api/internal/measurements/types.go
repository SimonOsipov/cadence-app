package measurements

// Metric is one of §03's eight measured quantities. The string is what `measurements.metric`
// stores, what the read handlers carry and what KMP's `Metric.fromCode` parses back — `hip`,
// and not «hips».
type Metric string

const (
	MetricWeight  Metric = "weight"
	MetricHRV     Metric = "hrv"
	MetricRHR     Metric = "rhr"
	MetricSleep   Metric = "sleep"
	MetricBodyFat Metric = "bodyfat"
	MetricWaist   Metric = "waist"
	MetricHip     Metric = "hip"
	MetricChest   Metric = "chest"
)

// Source is where a reading came from. Only the first is writable today: there is no importer,
// and the column is defaulted and withheld from the patient so a hand-written row cannot claim
// to have come off a watch.
type Source string

const (
	SourceManual        Source = "manual"
	SourceHealthKit     Source = "healthkit"
	SourceHealthConnect Source = "health_connect"
)

// Direction is which way a metric has to move for the patient to be getting better, and it is
// also the comparator a Threshold is read with.
type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

// Threshold is the clinical bound a reading is read against. A struct rather than a bare
// number because it is optional on the wire, where absent and zero are different
// answers — a zero HRV bound would read as «any reading clears it».
type Threshold struct {
	Value float64
}

// Bound is the interval a hand-typed reading has to fall inside. It refuses a typo — 1104 kg
// for 110,4 — and not a reading the clinic would worry about, which is what Threshold is for.
type Bound struct {
	Low  float64
	High float64
}

// Contains is closed at both ends, and false for a value that is not a number: NaN fails both
// comparisons, and a NaN reaching the column would be refused by 000025 as a 500.
func (b Bound) Contains(value float64) bool { return value >= b.Low && value <= b.High }

// MetricMeta is what travels beside the points: the unit the readings are in, the way the
// metric has to move, and the bound where the clinic has one. Label, decimal places and accent
// are rendering and stay on the surface.
type MetricMeta struct {
	Unit      string
	Direction Direction
	Threshold *Threshold
}

// Window is one of the four spans the trends screen offers. Three are lengths; the fourth is
// the geometry of a course, so the same calendar day gives it different edges for different
// patients.
//
// The codes are the frozen prototype's: `mobile/src/features/trends/data.ts` keys its series
// by them and both trends screens name them in their pickers. KMP's TrendWindow is an enum
// with no wire form of its own, so this is where the string is decided.
type Window string

const (
	WindowWeek        Window = "7d"
	WindowFourWeeks   Window = "4w"
	WindowThreeMonths Window = "3m"
	WindowCycle       Window = "cycle"
)

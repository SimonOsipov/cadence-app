package protocol

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// The seam these exist for, named because it is the whole argument: `type Cadence string`
// makes a foreign value representable, and a Kotlin enum serialised as «CANCELLED» read as
// «not cancelled» to all six comparisons — a cancelled course handing out a full schedule
// with doses. Measured at step 1, deferred to here because a string becomes a value of these
// types for the first time in this step's CRUD and in step 11's seed.
func TestAValueOffTheSetIsRefusedRatherThanRepresented(t *testing.T) {
	for _, refused := range []string{"CANCELLED", "Active", "", " active", "paused"} {
		if got, ok := ParseStatus(refused); ok {
			t.Errorf("ParseStatus(%q) gave %q", refused, got)
		}
	}
	for _, refused := range []string{"WEEKLY", "Daily", "", "nPerWeek", "monthly"} {
		if got, ok := ParseCadence(refused); ok {
			t.Errorf("ParseCadence(%q) gave %q", refused, got)
		}
	}
	for _, refused := range []string{"INJECTION", "weighIn", "", "weigh-in", "infusion"} {
		if got, ok := ParseKind(refused); ok {
			t.Errorf("ParseKind(%q) gave %q", refused, got)
		}
	}
	for _, refused := range []string{"MG", "mg", "", "ме", "г"} {
		if got, ok := ParseDoseUnit(refused); ok {
			t.Errorf("ParseDoseUnit(%q) gave %q", refused, got)
		}
	}
}

// The accept side against the declared sets rather than against a repeated literal: a
// constant renamed and a parser left behind is exactly what one list would hide.
func TestEveryDeclaredValueParsesBackToItself(t *testing.T) {
	for _, status := range Statuses() {
		if got, ok := ParseStatus(string(status)); !ok || got != status {
			t.Errorf("ParseStatus(%q) gave %q, %v", status, got, ok)
		}
	}
	for _, cadence := range Cadences() {
		if got, ok := ParseCadence(string(cadence)); !ok || got != cadence {
			t.Errorf("ParseCadence(%q) gave %q, %v", cadence, got, ok)
		}
	}
	for _, kind := range Kinds() {
		if got, ok := ParseKind(string(kind)); !ok || got != kind {
			t.Errorf("ParseKind(%q) gave %q, %v", kind, got, ok)
		}
	}
	for _, unit := range DoseUnits() {
		if got, ok := ParseDoseUnit(string(unit)); !ok || got != unit {
			t.Errorf("ParseDoseUnit(%q) gave %q, %v", unit, got, ok)
		}
	}
}

// The sets are the schema's, and the schema is written elsewhere. Sizes are asserted too
// because a set that lost a member still round-trips: every value in it parses, and the
// missing one is never asked for.
func TestTheSetsAreTheOnesTheSchemaNames(t *testing.T) {
	for _, set := range []struct {
		name string
		got  []string
		want []string
	}{
		{"statuses", asStrings(Statuses()), []string{"active", "cancelled", "completed"}},
		{"cadences", asStrings(Cadences()), []string{"daily", "n_per_week", "weekly"}},
		{"kinds", asStrings(Kinds()), []string{"injection", "supplement", "weigh_in"}},
		{"units", asStrings(DoseUnits()), []string{"мг", "мкг"}},
	} {
		if len(set.got) != len(set.want) {
			t.Errorf("%s: %v, want %v", set.name, set.got, set.want)

			continue
		}
		for i := range set.want {
			if set.got[i] != set.want[i] {
				t.Errorf("%s: %v, want %v", set.name, set.got, set.want)

				break
			}
		}
	}
}

// A day the type admits and the calendar does not. §03's calendar arithmetic runs through
// time, and == runs over the fields, so an impossible date makes the two disagree: they call
// it a different day and the same day at once, and a dose logged against it closes no
// occurrence.
func TestADayTheCalendarDoesNotHaveIsRefused(t *testing.T) {
	for _, refused := range []civil.Date{
		{Year: 2026, Month: time.February, Day: 30},
		{Year: 2026, Month: time.April, Day: 31},
		{Year: 2026, Month: time.January, Day: 0},
		{Year: 2026, Month: time.January, Day: 32},
		{Year: 2026, Month: time.Month(0), Day: 1},
		{Year: 2026, Month: time.Month(13), Day: 1},
	} {
		if refused.Valid() {
			t.Errorf("%v passed as a day", refused)
		}
	}

	// The leap day, which is the case a naive month-length table gets wrong.
	for _, accepted := range []civil.Date{
		{Year: 2024, Month: time.February, Day: 29},
		{Year: 2026, Month: time.February, Day: 28},
		{Year: 2026, Month: time.December, Day: 31},
		{Year: 2026, Month: time.January, Day: 1},
	} {
		if !accepted.Valid() {
			t.Errorf("%v was refused as a day", accepted)
		}
	}
}

func asStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}

	return out
}

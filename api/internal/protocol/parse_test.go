package protocol

import (
	"testing"
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

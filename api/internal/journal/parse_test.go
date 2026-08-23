package journal

import "testing"

// The seam these exist for: a string becomes a member of a closed set here and nowhere else.
// `type Tag string` makes a foreign value representable, and an unparsed one reaching the
// insert comes back as a raw 23514 naming a constraint rather than the field the patient
// filled in.
func TestAValueOffTheSetIsRefusedRatherThanRepresented(t *testing.T) {
	for _, refused := range []string{"NAUSEA", "Nausea", "", " nausea", "dizziness"} {
		if got, ok := ParseTag(refused); ok {
			t.Errorf("ParseTag(%q) gave %q", refused, got)
		}
	}
	for _, refused := range []string{"DOSE", "Manual", "", "imported", "auto"} {
		if got, ok := ParseSource(refused); ok {
			t.Errorf("ParseSource(%q) gave %q", refused, got)
		}
	}
}

// The accept side against the declared sets rather than a repeated literal: a constant
// renamed and a parser left behind is what one list would hide.
func TestEveryDeclaredValueParsesBackToItself(t *testing.T) {
	for _, tag := range Tags() {
		if got, ok := ParseTag(string(tag)); !ok || got != tag {
			t.Errorf("ParseTag(%q) gave %q, %v", tag, got, ok)
		}
	}
	for _, source := range Sources() {
		if got, ok := ParseSource(string(source)); !ok || got != source {
			t.Errorf("ParseSource(%q) gave %q, %v", source, got, ok)
		}
	}
}

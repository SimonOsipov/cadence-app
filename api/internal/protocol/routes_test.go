package protocol

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

// Every refusal this context can produce, and the status it becomes. The list is the point:
// a default that mapped the unknown onto 422 would tell a doctor their form is wrong about a
// bug in this process, and one that mapped it onto 500 silently would hide a real refusal.
func TestEachRefusalBecomesItsOwnStatus(t *testing.T) {
	for _, mapped := range []struct {
		err  error
		want int
	}{
		{ErrNotAPrescriber, http.StatusForbidden},
		{ErrNotYourPatient, http.StatusForbidden},
		{ErrMalformedIdentifier, http.StatusUnprocessableEntity},
		{ErrNoSuchProtocol, http.StatusNotFound},
		{ErrNoSuchItem, http.StatusNotFound},
		{ErrAlreadyRunning, http.StatusConflict},
		{ErrItemHasBeenInjected, http.StatusConflict},
		{ErrNoSuchCompound, http.StatusUnprocessableEntity},
		{errors.New("something nobody named"), http.StatusInternalServerError},
	} {
		var status huma.StatusError
		if !errors.As(answer("under test", mapped.err), &status) {
			t.Errorf("%v did not become a status", mapped.err)

			continue
		}
		if status.GetStatus() != mapped.want {
			t.Errorf("%v became %d, want %d", mapped.err, status.GetStatus(), mapped.want)
		}
	}
}

// The shape refusals as a set rather than one by one: this is the list the transport reads,
// and a rule added to shape.go and left out of it reaches the doctor as a 500 about their
// own form. Asserting the set and not its size, because a suite that counted entries is how
// two invented values shipped earlier in this project.
func TestEveryShapeRefusalIsA422(t *testing.T) {
	for _, refusal := range shapeRefusals() {
		var status huma.StatusError
		if !errors.As(answer("under test", refusal), &status) || status.GetStatus() != http.StatusUnprocessableEntity {
			t.Errorf("%v is not a 422", refusal)
		}
	}

	// And the other direction: every sentinel Check can return is in the list. Declared
	// errors are found by walking what Check actually produces over a draft edited into
	// each refusal — the same drafts the unit suite uses — so a new rule with no entry
	// here fails rather than being noticed later.
	for _, produced := range refusalsCheckProduces(t) {
		if !isAShapeRefusal(produced) {
			t.Errorf("Check returns %v and the transport does not know it", produced)
		}
	}
}

// The three the wire can break before Check is reached: a date, a time and a weekday are
// strings until this point, and each has to become a 422 rather than a panic or a zero.
func TestAMalformedFieldIsRefusedBeforeItBecomesAValue(t *testing.T) {
	for _, malformed := range []struct {
		name string
		edit func(*Course)
	}{
		{"a start date that is not a date", func(c *Course) { c.StartDate = "2026-13-40" }},
		{"a start date in another format", func(c *Course) { c.StartDate = "04.05.2026" }},
		{"a status off the set", func(c *Course) { c.Status = "paused" }},
		{"a kind off the set", func(c *Course) { c.Items[0].Kind = "infusion" }},
		{"a cadence off the set", func(c *Course) { c.Items[0].Cadence = "monthly" }},
		{"a weekday outside 1..7", func(c *Course) { c.Items[0].DaysOfWeek = []int{0} }},
		{"a weekday past Sunday", func(c *Course) { c.Items[0].DaysOfWeek = []int{8} }},
		{"a time that is not a time", func(c *Course) { c.Items[0].Times = []string{"25:00"} }},
		{"a dose unit off the set", func(c *Course) { c.Items[0].Phases[0].DoseUnit = "ме" }},
	} {
		t.Run(malformed.name, func(t *testing.T) {
			course := aWireCourse()
			malformed.edit(&course)

			_, err := course.draft("5d4f3b7c-0000-4000-8000-0000000000a1")

			var status huma.StatusError
			if !errors.As(err, &status) || status.GetStatus() != http.StatusUnprocessableEntity {
				t.Errorf("got %v, want a 422", err)
			}
		})
	}

	// The accept side: the same course, unedited, becomes a draft its own Check passes.
	drafted, err := aWireCourse().draft("5d4f3b7c-0000-4000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("a plain course was refused: %v", err)
	}
	if err := drafted.Check(); err != nil {
		t.Errorf("the drafted course does not pass its own rules: %v", err)
	}
}

// What the wire carries has to arrive intact — a conversion that dropped a field would pass
// every refusal test above and prescribe the wrong thing.
func TestTheWireValuesArriveAsTheyWereSent(t *testing.T) {
	drafted, err := aWireCourse().draft("5d4f3b7c-0000-4000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("drafting: %v", err)
	}

	if drafted.StartDate.String() != "2026-05-04" {
		t.Errorf("start date arrived as %v", drafted.StartDate)
	}
	if drafted.Weeks != 12 || drafted.Status != StatusActive {
		t.Errorf("the course arrived as %d weeks, %q", drafted.Weeks, drafted.Status)
	}

	item := drafted.Items[0]
	// ISO 7 is Sunday, and Go's Weekday counts Sunday from zero — the one conversion in
	// this file that is not a parse, and the one a cast would get wrong.
	if len(item.DaysOfWeek) != 1 || item.DaysOfWeek[0] != 0 {
		t.Errorf("days arrived as %v, want Sunday", item.DaysOfWeek)
	}
	if len(item.Times) != 2 || item.Times[0].String() != "08:00" || item.Times[1].String() != "20:30" {
		t.Errorf("times arrived as %v", item.Times)
	}
	if len(item.Phases) != 2 || item.Phases[1].Dose.Value != 0.5 || item.Phases[1].Dose.Unit != MG {
		t.Errorf("phases arrived as %v", item.Phases)
	}
	if item.Compound.New == nil || item.Compound.New.NameRU != "Семаглутид" {
		t.Errorf("the drug arrived as %+v", item.Compound)
	}
	if !item.Loggable {
		t.Error("loggable arrived false")
	}
}

func aWireCourse() Course {
	return Course{
		StartDate: "2026-05-04",
		Weeks:     12,
		Status:    "active",
		Items: []Item{{
			Kind: "injection",
			Compound: &Drug{
				NameRU: "Семаглутид", DefaultUnit: "мг", Route: "sc", Icon: "syringe",
			},
			Cadence:    "weekly",
			DaysOfWeek: []int{7},
			Times:      []string{"08:00", "20:30"},
			Loggable:   true,
			Phases: []Phase{
				{FromWeek: 1, ToWeek: 4, DoseValue: 0.25, DoseUnit: "мг"},
				{FromWeek: 5, ToWeek: 12, DoseValue: 0.5, DoseUnit: "мг"},
			},
		}},
	}
}

// refusalsCheckProduces walks the drafts the unit suite refuses and collects what came back,
// so the list the transport reads is compared against behaviour rather than against itself.
var anItem = ProtocolItemID("5d4f3b7c-0000-4000-8000-0000000000e1")

func refusalsCheckProduces(t *testing.T) []error {
	t.Helper()

	var produced []error
	for _, edit := range []func(*Draft){
		func(d *Draft) { d.Weeks = 0 },
		func(d *Draft) { d.StartDate.Day = 30; d.StartDate.Month = 2 },
		func(d *Draft) { d.Status = "paused" },
		func(d *Draft) { d.Items = nil },
		func(d *Draft) { d.Items[0].Kind = "infusion" },
		func(d *Draft) { d.Items[0].Cadence = "monthly" },
		func(d *Draft) { d.Items[0].Cadence = CadenceDaily },
		func(d *Draft) { d.Items[0].Times = nil },
		func(d *Draft) { d.Items[0].Compound = CompoundRef{} },
		func(d *Draft) { d.Items[0].Kind = KindWeighIn },
		func(d *Draft) { d.Items[0].Phases = nil },
		func(d *Draft) { d.Items[0].Phases[0] = ProtocolPhase{FromWeek: 4, ToWeek: 1} },
		func(d *Draft) { d.Items[0].Phases[0].FromWeek = 0 },
		func(d *Draft) { d.Items[0].Phases[0].Dose.Value = 0 },
		func(d *Draft) { d.Items[0].Phases[0].Dose.Unit = "ме" },
		func(d *Draft) { d.Items[0].Phases[1].FromWeek = 4 },
		func(d *Draft) {
			d.Items[0].Compound = CompoundRef{New: &NewCompound{NameRU: "Ретатрутид"}}
		},
		func(d *Draft) {
			d.Items[0].Compound = CompoundRef{New: &NewCompound{
				NameRU: strings.Repeat("я", 201), DefaultUnit: MG, Route: "sc", Icon: "syringe",
			}}
		},
		func(d *Draft) {
			second := d.Items[0]
			d.Items[0].ID = &anItem
			second.ID = &anItem
			d.Items = append(d.Items, second)
		},
	} {
		if err := aPlan(edit).Check(); err != nil {
			produced = append(produced, err)
		}
	}

	return produced
}

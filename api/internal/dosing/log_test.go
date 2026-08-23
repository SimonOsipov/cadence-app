package dosing

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

var (
	theItem  = protocol.ProtocolItemID("11111111-0000-4000-8000-000000000001")
	theOther = protocol.ProtocolItemID("11111111-0000-4000-8000-000000000002")
	theDay   = civil.NewDate(2026, time.May, 10)
)

// A weekly injection on Sundays at 08:00, and a twice-daily supplement — §03's BPC-157 shape,
// which is the reason «the first open slot» is not «the first slot».
func aPlan() protocol.Plan {
	compound := protocol.CompoundID("11111111-0000-4000-8000-00000000000d")
	sunday := protocol.Dose{Value: 0.25, Unit: protocol.MG}

	return protocol.Plan{
		Protocol: protocol.Protocol{
			PatientID: "11111111-0000-4000-8000-0000000000a1",
			StartDate: civil.NewDate(2026, time.May, 4),
			Weeks:     12,
			Status:    protocol.StatusActive,
		},
		Items: []protocol.ProtocolItem{
			{
				ID: theItem, Kind: protocol.KindInjection, CompoundID: &compound,
				Cadence: protocol.CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
				Times: []civil.Slot{{Hour: 8}}, Loggable: true,
			},
			{
				ID: theOther, Kind: protocol.KindSupplement,
				Cadence: protocol.CadenceDaily,
				Times:   []civil.Slot{{Hour: 8}, {Hour: 20}}, Loggable: true,
			},
		},
		Phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
			theItem:  {{FromWeek: 1, ToWeek: 12, Dose: sunday}},
			theOther: {{FromWeek: 1, ToWeek: 12, Dose: sunday}},
		},
	}
}

func aDraft(edit func(*Draft)) Draft {
	dose := protocol.Dose{Value: 0.25, Unit: protocol.MG}
	draft := Draft{
		ItemID:          theItem,
		Dose:            &dose,
		ClientRequestID: "wizard-0001",
	}
	if edit != nil {
		edit(&draft)
	}

	return draft
}

// A closed set of outcomes and not a nullable id: «the day rolled over», «already logged»
// and «the form is not filled in» are three different things for a screen to say, and one
// nil makes them one.
func TestADraftThatCannotMakeAnEventIsIncomplete(t *testing.T) {
	for _, missing := range []struct {
		name string
		edit func(*Draft)
	}{
		{"no item", func(d *Draft) { d.ItemID = "" }},
		{"no dose", func(d *Draft) { d.Dose = nil }},
		{"a dose of nothing", func(d *Draft) { d.Dose.Value = 0 }},
		{"a dose in no unit", func(d *Draft) { d.Dose.Unit = "" }},
		{"no client key", func(d *Draft) { d.ClientRequestID = "" }},
	} {
		t.Run(missing.name, func(t *testing.T) {
			if _, outcome := Resolve(aPlan(), nil, theDay, aDraft(missing.edit)); outcome != Incomplete {
				t.Errorf("got %q, want %q", outcome, Incomplete)
			}
		})
	}
}

func TestAnItemWithNoOccurrenceTodayIsNotScheduledToday(t *testing.T) {
	// Monday: the weekly injection falls on Sundays. The day rolled over under an open
	// app, which is the shape this outcome exists for.
	monday := civil.NewDate(2026, time.May, 11)

	if _, outcome := Resolve(aPlan(), nil, monday, aDraft(nil)); outcome != NotScheduledToday {
		t.Errorf("got %q, want %q", outcome, NotScheduledToday)
	}

	// And an item that is not in the plan at all, which a stale client can name.
	stranger := aDraft(func(d *Draft) { d.ItemID = "11111111-0000-4000-8000-00000000dead" })
	if _, outcome := Resolve(aPlan(), nil, theDay, stranger); outcome != NotScheduledToday {
		t.Errorf("a stranger item: got %q, want %q", outcome, NotScheduledToday)
	}
}

// loggable is checked on the write and not only where the item was chosen: a caller building
// a draft by hand never passes through the chooser.
func TestAnItemTheClinicDoesNotLogIsNotScheduledToday(t *testing.T) {
	plan := aPlan()
	plan.Items[0].Loggable = false

	if _, outcome := Resolve(plan, nil, theDay, aDraft(nil)); outcome != NotScheduledToday {
		t.Errorf("got %q, want %q", outcome, NotScheduledToday)
	}
}

func TestTheFirstOpenSlotIsTakenAndNotTheFirstSlot(t *testing.T) {
	// The twice-daily item with its morning dose already logged: «the first occurrence»
	// would tell the patient the evening dose was recorded when it was not.
	morning := []protocol.LoggedSlot{
		{ItemID: theOther, Date: theDay, Time: &civil.Slot{Hour: 8}},
	}
	draft := aDraft(func(d *Draft) { d.ItemID = theOther })

	slot, outcome := Resolve(aPlan(), morning, theDay, draft)
	if outcome != Written {
		t.Fatalf("got %q, want %q", outcome, Written)
	}
	if slot.Time == nil || slot.Time.Hour != 20 {
		t.Errorf("the dose was recorded against %v, want the evening slot", slot.Time)
	}
	if slot.Date != theDay {
		t.Errorf("the dose was recorded on %v, want %v", slot.Date, theDay)
	}
}

func TestEveryOccurrenceLoggedIsAlreadyLogged(t *testing.T) {
	// «Every», not «the one»: both of the twice-daily slots, so re-tapping «Сохранить»
	// must not take another dose out of the vial.
	both := []protocol.LoggedSlot{
		{ItemID: theOther, Date: theDay, Time: &civil.Slot{Hour: 8}},
		{ItemID: theOther, Date: theDay, Time: &civil.Slot{Hour: 20}},
	}
	draft := aDraft(func(d *Draft) { d.ItemID = theOther })

	if _, outcome := Resolve(aPlan(), both, theDay, draft); outcome != AlreadyLogged {
		t.Errorf("got %q, want %q", outcome, AlreadyLogged)
	}

	// And the once-a-day item, where «every» and «the one» coincide — the case that
	// would pass whichever rule were written.
	sunday := []protocol.LoggedSlot{{ItemID: theItem, Date: theDay, Time: &civil.Slot{Hour: 8}}}
	if _, outcome := Resolve(aPlan(), sunday, theDay, aDraft(nil)); outcome != AlreadyLogged {
		t.Errorf("the weekly item: got %q, want %q", outcome, AlreadyLogged)
	}
}

// The dose that lands is the protocol's, not the draft's: a client that sent a number the
// course does not prescribe would otherwise write it into the clinical record.
//
// The draft's dose has to differ from the prescribed one or the two cannot be told apart —
// the first version of this test sent 0,25 мг against a course prescribing 0,25 мг, and
// `Prescribed: draft.Dose` survived it.
func TestTheSlotCarriesWhatTheCoursePrescribes(t *testing.T) {
	sentSomethingElse := aDraft(func(d *Draft) {
		d.Dose = &protocol.Dose{Value: 5, Unit: protocol.MCG}
	})

	slot, outcome := Resolve(aPlan(), nil, theDay, sentSomethingElse)
	if outcome != Written {
		t.Fatalf("got %q", outcome)
	}
	if slot.Prescribed == nil || slot.Prescribed.Value != 0.25 || slot.Prescribed.Unit != protocol.MG {
		t.Errorf("the slot prescribes %+v, want 0,25 мг", slot.Prescribed)
	}
	if slot.ItemID != theItem {
		t.Errorf("the slot names %s", slot.ItemID)
	}
}

// A logged event of another item does not close this one's occurrence, and a log of another
// day does not either — the two axes a matcher keyed on one of them would confuse.
func TestALogOfAnotherItemOrAnotherDayLeavesThisOneOpen(t *testing.T) {
	for _, elsewhere := range []struct {
		name   string
		logged protocol.LoggedSlot
	}{
		{"another item, same slot", protocol.LoggedSlot{
			ItemID: theOther, Date: theDay, Time: &civil.Slot{Hour: 8},
		}},
		{"the same item, another day", protocol.LoggedSlot{
			ItemID: theItem, Date: civil.NewDate(2026, time.May, 3), Time: &civil.Slot{Hour: 8},
		}},
	} {
		t.Run(elsewhere.name, func(t *testing.T) {
			_, outcome := Resolve(aPlan(), []protocol.LoggedSlot{elsewhere.logged}, theDay, aDraft(nil))
			if outcome != Written {
				t.Errorf("got %q, want %q", outcome, Written)
			}
		})
	}
}

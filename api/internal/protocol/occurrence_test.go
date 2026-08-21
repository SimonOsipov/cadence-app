package protocol

import (
	"testing"
	"time"
)

// The minimal protocol §03 describes: a 12-week cycle starting Sunday
// 10 May 2026, Семаглутид weekly on that weekday with three titration bands,
// BPC-157 daily at 08:00 and 20:00.

var (
	cycleStart = NewDate(2026, time.May, 10)
	patient    = UserID("p-1")
	protocolID = ProtocolID("pr-1")
	sema       = ProtocolItemID("sema")
	bpc        = ProtocolItemID("bpc")
)

var fixtureProtocol = Protocol{
	ID:        protocolID,
	PatientID: patient,
	StartDate: cycleStart,
	Weeks:     12,
	Status:    StatusActive,
}

var fixtureItems = []ProtocolItem{
	{
		ID:         sema,
		ProtocolID: protocolID,
		Kind:       KindInjection,
		CompoundID: compoundPtr("sema"),
		Cadence:    CadenceWeekly,
		DaysOfWeek: []time.Weekday{time.Sunday},
		Times:      []Slot{{Hour: 7}},
		Loggable:   true,
	},
	{
		ID:         bpc,
		ProtocolID: protocolID,
		Kind:       KindInjection,
		CompoundID: compoundPtr("bpc"),
		Cadence:    CadenceDaily,
		Times:      []Slot{{Hour: 8}, {Hour: 20}},
		Loggable:   true,
	},
}

var fixturePhases = map[ProtocolItemID][]ProtocolPhase{
	sema: {
		{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
		{FromWeek: 5, ToWeek: 8, Dose: Dose{Value: 0.5, Unit: MG}},
		{FromWeek: 9, ToWeek: 12, Dose: Dose{Value: 1.0, Unit: MG}},
	},
	bpc: {{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 250.0, Unit: MCG}}},
}

func compoundPtr(raw string) *CompoundID {
	id := CompoundID(raw)
	return &id
}

func slot(hour, minute int) *Slot { return &Slot{Hour: hour, Minute: minute} }

func fixturePlan() Plan {
	return Plan{Protocol: fixtureProtocol, Items: fixtureItems, Phases: fixturePhases}
}

func occurrencesOn(d Date, logged []LoggedSlot, today Date) []Occurrence {
	return OccurrencesFor(fixturePlan(), logged, d, today)
}

func semaDoseOn(t *testing.T, d Date) Dose {
	t.Helper()
	for _, o := range occurrencesOn(d, nil, d) {
		if o.ItemID == sema {
			if o.Dose == nil {
				t.Fatalf("no dose on %v", d)
			}
			return *o.Dose
		}
	}
	t.Fatalf("no Семаглутид occurrence on %v", d)
	return Dose{}
}

func TestTheCycleWeekCountsFromTheProtocolStartAndNotFromAConstant(t *testing.T) {
	for _, c := range []struct {
		date Date
		week int
	}{
		{NewDate(2026, time.May, 10), 1},
		{NewDate(2026, time.May, 16), 1},
		{NewDate(2026, time.May, 17), 2},
		{NewDate(2026, time.May, 31), 4},
		{NewDate(2026, time.August, 1), 12},
	} {
		week, ok := CycleWeek(fixtureProtocol, c.date)
		if !ok || week != c.week {
			t.Errorf("%v is week %d, got %d (in cycle: %v)", c.date, c.week, week, ok)
		}
	}
}

func TestADateOutsideTheCycleHasNoWeekAndNoOccurrences(t *testing.T) {
	before := NewDate(2026, time.May, 9)
	after := NewDate(2026, time.August, 2)

	if _, ok := CycleWeek(fixtureProtocol, before); ok {
		t.Error("the day before the protocol began")
	}
	if _, ok := CycleWeek(fixtureProtocol, after); ok {
		t.Error("the day after twelve weeks")
	}
	if got := occurrencesOn(before, nil, before); len(got) != 0 {
		t.Errorf("occurrences before the start: %v", got)
	}
	if got := occurrencesOn(after, nil, after); len(got) != 0 {
		t.Errorf("occurrences after the end: %v", got)
	}
}

func TestAWeeklyItemFallsOnItsWeekdayAndNowhereElse(t *testing.T) {
	sunday := NewDate(2026, time.May, 17)
	monday := NewDate(2026, time.May, 18)

	if got := countOf(occurrencesOn(sunday, nil, sunday), sema); got != 1 {
		t.Errorf("one Семаглутид on its weekday, got %d", got)
	}
	if got := countOf(occurrencesOn(monday, nil, monday), sema); got != 0 {
		t.Errorf("none on any other day, got %d", got)
	}
}

func TestADailyItemEmitsEveryTimeItIsScheduledFor(t *testing.T) {
	day := NewDate(2026, time.May, 18)

	var times []Slot
	for _, o := range occurrencesOn(day, nil, day) {
		if o.ItemID == bpc {
			times = append(times, o.Time)
		}
	}

	want := []Slot{{Hour: 8}, {Hour: 20}}
	if !slotsEqual(times, want) {
		t.Errorf("both slots, in order: want %v got %v", want, times)
	}
}

func TestTheDoseFollowsTheTitrationPhaseForThatWeek(t *testing.T) {
	// Boundary weeks are what an off-by-one gets wrong, so all six are asserted.
	for _, c := range []struct {
		date Date
		dose Dose
	}{
		{NewDate(2026, time.May, 10), Dose{Value: 0.25, Unit: MG}},
		{NewDate(2026, time.May, 31), Dose{Value: 0.25, Unit: MG}},
		{NewDate(2026, time.June, 7), Dose{Value: 0.5, Unit: MG}},
		{NewDate(2026, time.June, 28), Dose{Value: 0.5, Unit: MG}},
		{NewDate(2026, time.July, 5), Dose{Value: 1.0, Unit: MG}},
		// 26 July, not 1 August: the latter is a Saturday, and Семаглутид falls on the
		// cycle's start weekday.
		{NewDate(2026, time.July, 26), Dose{Value: 1.0, Unit: MG}},
	} {
		if got := semaDoseOn(t, c.date); got != c.dose {
			t.Errorf("%v: want %v got %v", c.date, c.dose, got)
		}
	}
}

func TestALoggedEventMarksItsOccurrenceDoneAndLeavesTheOthersAlone(t *testing.T) {
	date := NewDate(2026, time.May, 17)

	occurrences := occurrencesOn(date, []LoggedSlot{{ItemID: sema, Date: date, Time: slot(7, 0)}}, date)

	if got := statusOfItem(occurrences, sema); got != StatusDone {
		t.Errorf("the logged one is done, got %v", got)
	}
	for _, o := range occurrences {
		if o.ItemID == bpc && o.Status == StatusDone {
			t.Error("BPC-157 was not logged")
		}
	}
}

func TestAnEventOnAnotherDayDoesNotSatisfyTodaysOccurrence(t *testing.T) {
	// Matching on item alone would mark every Sunday done for the rest of the cycle.
	logged := LoggedSlot{ItemID: sema, Date: NewDate(2026, time.May, 17), Time: slot(7, 0)}
	later := NewDate(2026, time.May, 24)

	if got := statusOfItem(occurrencesOn(later, []LoggedSlot{logged}, later), sema); got == StatusDone {
		t.Error("last Sunday's injection closed this Sunday's slot")
	}
}

func TestAPastOccurrenceWithNoEventIsMissedAFutureOneScheduledAndTodaysPending(t *testing.T) {
	today := NewDate(2026, time.May, 31)

	assertEveryStatus(t, occurrencesOn(NewDate(2026, time.May, 17), nil, today), StatusMissed)
	assertEveryStatus(t, occurrencesOn(NewDate(2026, time.June, 7), nil, today), StatusScheduled)
	assertEveryStatus(t, occurrencesOn(today, nil, today), StatusPending)
}

func TestAnEventWithoutASlotSatisfiesNoOccurrence(t *testing.T) {
	// Measured: one BPC event with a null scheduledForTime once marked both 08:00 and
	// 20:00 DONE, telling a patient the evening injection was already done.
	date := NewDate(2026, time.May, 18)

	for _, o := range occurrencesOn(date, []LoggedSlot{{ItemID: bpc, Date: date}}, date) {
		if o.ItemID == bpc && o.Status == StatusDone {
			t.Error("a slotless event closed a slot")
		}
	}
}

func TestAnEventMatchesOnlyTheSlotItWasLoggedAgainst(t *testing.T) {
	date := NewDate(2026, time.May, 18)

	occurrences := occurrencesOn(date, []LoggedSlot{{ItemID: bpc, Date: date, Time: slot(8, 0)}}, date)

	if got := statusAt(occurrences, bpc, Slot{Hour: 8}); got != StatusDone {
		t.Errorf("the morning slot is done, got %v", got)
	}
	if got := statusAt(occurrences, bpc, Slot{Hour: 20}); got == StatusDone {
		t.Error("the evening slot is still due")
	}
}

func TestAnItemScheduledSeveralDaysAWeekFallsOnEachOfThem(t *testing.T) {
	// N_PER_WEEK shares a branch with WEEKLY correctly, but nothing exercised it, so
	// deleting the case would have made «трижды в неделю» vanish with the gate green.
	thrice := ProtocolItem{
		ID:         ProtocolItemID("tb"),
		ProtocolID: protocolID,
		Kind:       KindInjection,
		CompoundID: compoundPtr("tb"),
		Cadence:    CadenceNPerWeek,
		DaysOfWeek: []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		Times:      []Slot{{Hour: 9}},
		Loggable:   true,
	}
	plan := Plan{
		Protocol: fixtureProtocol,
		Items:    []ProtocolItem{thrice},
		Phases:   map[ProtocolItemID][]ProtocolPhase{thrice.ID: fixturePhases[bpc]},
	}

	var hit []Date
	for day := 18; day <= 24; day++ {
		d := NewDate(2026, time.May, day)
		if len(OccurrencesFor(plan, nil, d, d)) > 0 {
			hit = append(hit, d)
		}
	}

	want := []Date{NewDate(2026, time.May, 18), NewDate(2026, time.May, 20), NewDate(2026, time.May, 22)}
	if !datesEqual(hit, want) {
		t.Errorf("want %v got %v", want, hit)
	}
}

func TestACancelledProtocolGeneratesNothing(t *testing.T) {
	// Measured: a stopped course once produced a full schedule and kept telling the
	// patient to inject.
	date := NewDate(2026, time.May, 18)
	cancelled := fixturePlan()
	cancelled.Protocol.Status = StatusCancelled

	if got := OccurrencesFor(cancelled, nil, date, date); len(got) != 0 {
		t.Errorf("a cancelled course still prescribed: %v", got)
	}
}

func TestACompletedProtocolKeepsItsHistory(t *testing.T) {
	// An earlier guard blanked COMPLETED too, and every patient reaches it after twelve
	// weeks — their calendar would empty retroactively.
	date := NewDate(2026, time.May, 18)
	done := fixturePlan()
	done.Protocol.Status = StatusCompleted

	active := OccurrencesFor(fixturePlan(), nil, date, date)
	if !occurrencesEqual(OccurrencesFor(done, nil, date, date), active) {
		t.Error("a completed course lost its history")
	}
}

func TestTheWeeklyRateFollowsTheCadenceAndTheTimes(t *testing.T) {
	// `dosesPerWeek` had no test: replacing its body with a constant left the whole
	// suite green and the reorder hint would have quietly vanished.
	weekly := fixtureItems[0]
	daily := fixtureItems[1]
	thrice := weekly
	thrice.Cadence = CadenceNPerWeek
	thrice.DaysOfWeek = []time.Weekday{time.Monday, time.Wednesday, time.Friday}

	if got := DosesPerWeek(weekly); got != 1.0 {
		t.Errorf("one day, one time: got %v", got)
	}
	if got := DosesPerWeek(daily); got != 14.0 {
		t.Errorf("seven days, two times: got %v", got)
	}
	if got := DosesPerWeek(thrice); got != 3.0 {
		t.Errorf("three days, one time: got %v", got)
	}
}

func TestADoseLoggedForOneItemLeavesAnotherItemInTheSameSlotAlone(t *testing.T) {
	// Measured: deleting `event.protocolItemId == item.id` from `statusOf` left the
	// suite green, because no fixture put two items in the same (date, slot) before this
	// one. Without the check, logging one of two same-slot injections greys both.
	sunday := NewDate(2026, time.May, 17)
	at := Slot{Hour: 8}
	collide := make([]ProtocolItem, len(fixtureItems))
	copy(collide, fixtureItems)
	collide[0].Times = []Slot{at}
	plan := Plan{Protocol: fixtureProtocol, Items: collide, Phases: fixturePhases}

	var eight []Occurrence
	for _, o := range OccurrencesFor(plan, []LoggedSlot{{ItemID: sema, Date: sunday, Time: &at}}, sunday, sunday) {
		if o.Time == at {
			eight = append(eight, o)
		}
	}
	if len(eight) != 2 {
		t.Fatalf("the fixture has to put both items in one slot, got %d", len(eight))
	}

	if got := statusOfItem(eight, sema); got != StatusDone {
		t.Errorf("Семаглутид was logged: got %v", got)
	}
	if got := statusOfItem(eight, bpc); got != StatusPending {
		t.Errorf("BPC-157 was not logged; only Семаглутид was: got %v", got)
	}
}

func countOf(occurrences []Occurrence, item ProtocolItemID) int {
	n := 0
	for _, o := range occurrences {
		if o.ItemID == item {
			n++
		}
	}
	return n
}

func statusOfItem(occurrences []Occurrence, item ProtocolItemID) OccurrenceStatus {
	for _, o := range occurrences {
		if o.ItemID == item {
			return o.Status
		}
	}
	return OccurrenceStatus("")
}

func statusAt(occurrences []Occurrence, item ProtocolItemID, at Slot) OccurrenceStatus {
	for _, o := range occurrences {
		if o.ItemID == item && o.Time == at {
			return o.Status
		}
	}
	return OccurrenceStatus("")
}

func assertEveryStatus(t *testing.T, occurrences []Occurrence, want OccurrenceStatus) {
	t.Helper()
	if len(occurrences) == 0 {
		t.Fatalf("no occurrences to check for %v", want)
	}
	for _, o := range occurrences {
		if o.Status != want {
			t.Errorf("%v at %v: want %v got %v", o.ItemID, o.Time, want, o.Status)
		}
	}
}

func slotsEqual(a, b []Slot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func datesEqual(a, b []Date) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func occurrencesEqual(a, b []Occurrence) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ItemID != b[i].ItemID || a[i].Kind != b[i].Kind || a[i].Date != b[i].Date ||
			a[i].Time != b[i].Time || a[i].Status != b[i].Status {
			return false
		}
		switch {
		case a[i].Dose == nil && b[i].Dose == nil:
		case a[i].Dose == nil || b[i].Dose == nil:
			return false
		case *a[i].Dose != *b[i].Dose:
			return false
		}
	}
	return true
}

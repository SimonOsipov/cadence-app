package protocol

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// The minimal protocol §03 describes: a 12-week cycle starting Sunday
// 10 May 2026, Семаглутид weekly on that weekday with three titration bands,
// BPC-157 daily at 08:00 and 20:00.

var (
	cycleStart = civil.NewDate(2026, time.May, 10)
	patient    = civil.UserID("p-1")
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
		Times:      []civil.Slot{{Hour: 7}},
		Loggable:   true,
	},
	{
		ID:         bpc,
		ProtocolID: protocolID,
		Kind:       KindInjection,
		CompoundID: compoundPtr("bpc"),
		Cadence:    CadenceDaily,
		Times:      []civil.Slot{{Hour: 8}, {Hour: 20}},
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

func slot(hour, minute int) *civil.Slot { return &civil.Slot{Hour: hour, Minute: minute} }

func fixturePlan() Plan {
	return Plan{Protocol: fixtureProtocol, Items: fixtureItems, Phases: fixturePhases}
}

func occurrencesOn(d civil.Date, logged []LoggedSlot, today civil.Date) []Occurrence {
	return OccurrencesFor(fixturePlan(), logged, d, today)
}

func semaDoseOn(t *testing.T, d civil.Date) Dose {
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
		date civil.Date
		week int
	}{
		{civil.NewDate(2026, time.May, 10), 1},
		{civil.NewDate(2026, time.May, 16), 1},
		{civil.NewDate(2026, time.May, 17), 2},
		{civil.NewDate(2026, time.May, 31), 4},
		{civil.NewDate(2026, time.August, 1), 12},
	} {
		week, ok := CycleWeek(fixtureProtocol, c.date)
		if !ok || week != c.week {
			t.Errorf("%v is week %d, got %d (in cycle: %v)", c.date, c.week, week, ok)
		}
	}
}

func TestADateOutsideTheCycleHasNoWeekAndNoOccurrences(t *testing.T) {
	before := civil.NewDate(2026, time.May, 9)
	after := civil.NewDate(2026, time.August, 2)

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
	sunday := civil.NewDate(2026, time.May, 17)
	monday := civil.NewDate(2026, time.May, 18)

	if got := countOf(occurrencesOn(sunday, nil, sunday), sema); got != 1 {
		t.Errorf("one Семаглутид on its weekday, got %d", got)
	}
	if got := countOf(occurrencesOn(monday, nil, monday), sema); got != 0 {
		t.Errorf("none on any other day, got %d", got)
	}
}

func TestADailyItemEmitsEveryTimeItIsScheduledFor(t *testing.T) {
	day := civil.NewDate(2026, time.May, 18)

	var times []civil.Slot
	for _, o := range occurrencesOn(day, nil, day) {
		if o.ItemID == bpc {
			times = append(times, o.Time)
		}
	}

	want := []civil.Slot{{Hour: 8}, {Hour: 20}}
	if !slotsEqual(times, want) {
		t.Errorf("both slots, in order: want %v got %v", want, times)
	}
}

func TestAnOccurrenceCarriesTheKindOfTheItemItCameFrom(t *testing.T) {
	// Replacing Kind with a constant survives everything else: occurrencesEqual is its only
	// reader, and in the completed-course test it compares two equally-mutated sides. The
	// calendar draws a needle, a pill or a scale from this field.
	day := civil.NewDate(2026, time.May, 18)
	weighIn := fixtureItems[1]
	weighIn.ID = ProtocolItemID("scale")
	weighIn.Kind = KindWeighIn
	weighIn.Times = []civil.Slot{{Hour: 9}}
	plan := Plan{
		Protocol: fixtureProtocol,
		Items:    append(append([]ProtocolItem{}, fixtureItems...), weighIn),
		Phases:   fixturePhases,
	}

	kinds := map[ProtocolItemID]ItemKind{}
	for _, o := range OccurrencesFor(plan, nil, day, day) {
		kinds[o.ItemID] = o.Kind
	}

	if kinds[bpc] != KindInjection {
		t.Errorf("BPC-157 is an injection, got %v", kinds[bpc])
	}
	if kinds[weighIn.ID] != KindWeighIn {
		t.Errorf("the weigh-in is a weigh-in, got %v", kinds[weighIn.ID])
	}
}

func TestTheDoseFollowsTheTitrationPhaseForThatWeek(t *testing.T) {
	// Boundary weeks are what an off-by-one gets wrong, so all six are asserted.
	for _, c := range []struct {
		date civil.Date
		dose Dose
	}{
		{civil.NewDate(2026, time.May, 10), Dose{Value: 0.25, Unit: MG}},
		{civil.NewDate(2026, time.May, 31), Dose{Value: 0.25, Unit: MG}},
		{civil.NewDate(2026, time.June, 7), Dose{Value: 0.5, Unit: MG}},
		{civil.NewDate(2026, time.June, 28), Dose{Value: 0.5, Unit: MG}},
		{civil.NewDate(2026, time.July, 5), Dose{Value: 1.0, Unit: MG}},
		// 26 July, not 1 August: the latter is a Saturday, and Семаглутид falls on the
		// cycle's start weekday.
		{civil.NewDate(2026, time.July, 26), Dose{Value: 1.0, Unit: MG}},
	} {
		if got := semaDoseOn(t, c.date); got != c.dose {
			t.Errorf("%v: want %v got %v", c.date, c.dose, got)
		}
	}
}

func TestALoggedEventMarksItsOccurrenceDoneAndLeavesTheOthersAlone(t *testing.T) {
	date := civil.NewDate(2026, time.May, 17)

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
	logged := LoggedSlot{ItemID: sema, Date: civil.NewDate(2026, time.May, 17), Time: slot(7, 0)}
	later := civil.NewDate(2026, time.May, 24)

	if got := statusOfItem(occurrencesOn(later, []LoggedSlot{logged}, later), sema); got == StatusDone {
		t.Error("last Sunday's injection closed this Sunday's slot")
	}
}

func TestAPastOccurrenceWithNoEventIsMissedAFutureOneScheduledAndTodaysPending(t *testing.T) {
	today := civil.NewDate(2026, time.May, 31)

	assertEveryStatus(t, occurrencesOn(civil.NewDate(2026, time.May, 17), nil, today), StatusMissed)
	assertEveryStatus(t, occurrencesOn(civil.NewDate(2026, time.June, 7), nil, today), StatusScheduled)
	assertEveryStatus(t, occurrencesOn(today, nil, today), StatusPending)
}

func TestAnEventWithoutASlotSatisfiesNoOccurrence(t *testing.T) {
	// Measured: one BPC event with a null scheduledForTime once marked both 08:00 and
	// 20:00 DONE, telling a patient the evening injection was already done.
	date := civil.NewDate(2026, time.May, 18)

	for _, o := range occurrencesOn(date, []LoggedSlot{{ItemID: bpc, Date: date}}, date) {
		if o.ItemID == bpc && o.Status == StatusDone {
			t.Error("a slotless event closed a slot")
		}
	}
}

func TestAnEventMatchesOnlyTheSlotItWasLoggedAgainst(t *testing.T) {
	date := civil.NewDate(2026, time.May, 18)

	occurrences := occurrencesOn(date, []LoggedSlot{{ItemID: bpc, Date: date, Time: slot(8, 0)}}, date)

	if got := statusAt(occurrences, bpc, civil.Slot{Hour: 8}); got != StatusDone {
		t.Errorf("the morning slot is done, got %v", got)
	}
	if got := statusAt(occurrences, bpc, civil.Slot{Hour: 20}); got == StatusDone {
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
		Times:      []civil.Slot{{Hour: 9}},
		Loggable:   true,
	}
	plan := Plan{
		Protocol: fixtureProtocol,
		Items:    []ProtocolItem{thrice},
		Phases:   map[ProtocolItemID][]ProtocolPhase{thrice.ID: fixturePhases[bpc]},
	}

	var hit []civil.Date
	for day := 18; day <= 24; day++ {
		d := civil.NewDate(2026, time.May, day)
		if len(OccurrencesFor(plan, nil, d, d)) > 0 {
			hit = append(hit, d)
		}
	}

	want := []civil.Date{civil.NewDate(2026, time.May, 18), civil.NewDate(2026, time.May, 20), civil.NewDate(2026, time.May, 22)}
	if !datesEqual(hit, want) {
		t.Errorf("want %v got %v", want, hit)
	}
}

func TestACancelledProtocolGeneratesNothing(t *testing.T) {
	// Measured: a stopped course once produced a full schedule and kept telling the
	// patient to inject.
	date := civil.NewDate(2026, time.May, 18)
	cancelled := fixturePlan()
	cancelled.Protocol.Status = StatusCancelled

	if got := OccurrencesFor(cancelled, nil, date, date); len(got) != 0 {
		t.Errorf("a cancelled course still prescribed: %v", got)
	}
}

func TestACompletedProtocolKeepsItsHistory(t *testing.T) {
	// An earlier guard blanked COMPLETED too, and every patient reaches it after twelve
	// weeks — their calendar would empty retroactively.
	date := civil.NewDate(2026, time.May, 18)
	done := fixturePlan()
	done.Protocol.Status = StatusCompleted

	active := OccurrencesFor(fixturePlan(), nil, date, date)
	// Two empties compare equal, so without this the assertion below holds whenever the
	// generator stops producing anything at all.
	if len(active) == 0 {
		t.Fatal("the active course has to produce something to compare against")
	}
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
	sunday := civil.NewDate(2026, time.May, 17)
	at := civil.Slot{Hour: 8}
	collide := make([]ProtocolItem, len(fixtureItems))
	copy(collide, fixtureItems)
	collide[0].Times = []civil.Slot{at}
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

func statusAt(occurrences []Occurrence, item ProtocolItemID, at civil.Slot) OccurrenceStatus {
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

func slotsEqual(a, b []civil.Slot) bool {
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

func datesEqual(a, b []civil.Date) bool {
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

func TestALoggedDoseStaysDoneOnceTheDayHasPassed(t *testing.T) {
	// Measured: inserting an early `if d.Before(today) { return StatusMissed }` at the head
	// of statusOf, ahead of the scan over logged slots, left all sixty-seven tests green.
	// Every other DONE in this suite is asserted with today == date, and WeekProtocolRows
	// asks OccurrencesFor for (today, today), so the row suite cannot vary the axis at all.
	// The schedule would grey out every injection the patient logged the moment midnight
	// passed. The Kotlin suite has the identical hole.
	injected := civil.NewDate(2026, time.May, 17)
	weeksLater := civil.NewDate(2026, time.May, 31)
	logged := []LoggedSlot{{ItemID: sema, Date: injected, Time: slot(7, 0)}}

	occurrences := occurrencesOn(injected, logged, weeksLater)

	if got := statusOfItem(occurrences, sema); got != StatusDone {
		t.Errorf("a dose logged a fortnight ago is still done, got %v", got)
	}
	// The sibling has to stay missed, or the assertion above would pass on a function that
	// simply never reports a miss.
	if got := statusAt(occurrences, bpc, civil.Slot{Hour: 8}); got != StatusMissed {
		t.Errorf("the injection nobody logged that day is missed, got %v", got)
	}
}

func TestTwoSlotsInsideOneHourAreLoggedApart(t *testing.T) {
	// The axis no fixture varies: every logged slot in this suite is built at a whole hour,
	// so `*s.Time == at` can be weakened to `s.Time.Hour == at.Hour` with all sixty-seven
	// tests green. §03's times[] admits 08:00 and 08:30 — the prototype already prescribes a
	// :30 time — and one log would then close both. Same defect class as the slotless event
	// above, on the half of the slot nothing measured.
	day := civil.NewDate(2026, time.May, 18)
	twice := fixtureItems[1]
	twice.Times = []civil.Slot{{Hour: 8}, {Hour: 8, Minute: 30}}
	plan := Plan{Protocol: fixtureProtocol, Items: []ProtocolItem{twice}, Phases: fixturePhases}

	occurrences := OccurrencesFor(plan, []LoggedSlot{{ItemID: bpc, Date: day, Time: slot(8, 0)}}, day, day)

	if len(occurrences) != 2 {
		t.Fatalf("the fixture has to put two slots inside one hour, got %d", len(occurrences))
	}
	if got := statusAt(occurrences, bpc, civil.Slot{Hour: 8}); got != StatusDone {
		t.Errorf("08:00 was logged, got %v", got)
	}
	if got := statusAt(occurrences, bpc, civil.Slot{Hour: 8, Minute: 30}); got != StatusPending {
		t.Errorf("08:30 was not, got %v", got)
	}
}

func TestACourseThatIsNotTwelveWeeksLongEndsWhereItsOwnLengthSays(t *testing.T) {
	// Every fixture in the package is twelve weeks, so `week > 12` at CycleWeek and
	// `AddDays(83)` at LastPrescribedDay survive together with the suite green. This is to
	// Weeks what the leap-year band test is to StartDate.
	eight := fixtureProtocol
	eight.Weeks = 8

	if week, ok := CycleWeek(eight, civil.NewDate(2026, time.July, 4)); !ok || week != 8 {
		t.Errorf("4 July is week 8 of an eight-week course, got %d (%v)", week, ok)
	}
	if _, ok := CycleWeek(eight, civil.NewDate(2026, time.July, 5)); ok {
		t.Error("5 July is past an eight-week course")
	}
	if got := eight.LastPrescribedDay(); got != civil.NewDate(2026, time.July, 4) {
		t.Errorf("day 55 is the last an eight-week course prescribes, got %v", got)
	}
}

func TestEachOccurrenceOwnsItsDose(t *testing.T) {
	// One *Dose hoisted out of the slot loop tied BPC-157's morning and evening occurrences
	// together: a caller writing through one changed the other. Kotlin shared an immutable
	// Dose and was safe; a pointer is not.
	day := civil.NewDate(2026, time.May, 18)

	occurrences := occurrencesOn(day, nil, day)
	var bpcSlots []Occurrence
	for _, o := range occurrences {
		if o.ItemID == bpc {
			bpcSlots = append(bpcSlots, o)
		}
	}
	if len(bpcSlots) != 2 || bpcSlots[0].Dose == nil || bpcSlots[1].Dose == nil {
		t.Fatalf("two dosed slots to compare, got %v", bpcSlots)
	}

	bpcSlots[0].Dose.Value = 99

	if bpcSlots[1].Dose.Value != 250.0 {
		t.Errorf("the evening slot moved with the morning one: %v", bpcSlots[1].Dose.Value)
	}
}

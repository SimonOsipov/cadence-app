package protocol

import (
	"testing"
	"time"
)

// The prototype's three rows: a weekly injection, a daily one, and a nightly supplement.

var (
	rowStart = NewDate(2026, time.May, 10)
	rowPID   = ProtocolID("pr")
	glycine  = ProtocolItemID("glycine")
)

func rowItem(id ProtocolItemID, kind ItemKind, times []Slot, days []time.Weekday) ProtocolItem {
	cadence := CadenceDaily
	if len(days) > 0 {
		cadence = CadenceWeekly
	}
	compound := CompoundID(id)
	return ProtocolItem{
		ID:         id,
		ProtocolID: rowPID,
		Kind:       kind,
		CompoundID: &compound,
		Cadence:    cadence,
		DaysOfWeek: days,
		Times:      times,
		Loggable:   kind != KindSupplement,
	}
}

var rowItems = []ProtocolItem{
	rowItem(sema, KindInjection, []Slot{{Hour: 7}}, []time.Weekday{time.Sunday}),
	rowItem(bpc, KindInjection, []Slot{{Hour: 8}, {Hour: 20}}, nil),
	rowItem(glycine, KindSupplement, []Slot{{Hour: 21, Minute: 30}}, nil),
}

var rowPhases = map[ProtocolItemID][]ProtocolPhase{
	sema: {
		{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
		{FromWeek: 5, ToWeek: 8, Dose: Dose{Value: 0.5, Unit: MG}},
		{FromWeek: 9, ToWeek: 12, Dose: Dose{Value: 1.0, Unit: MG}},
	},
	bpc: {{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 250.0, Unit: MCG}}},
}

var rowCompounds = []Compound{
	{ID: CompoundID(sema), Code: string(sema), NameRU: "SEMA", DefaultUnit: MG, Route: "п/к", Icon: "beaker"},
	{ID: CompoundID(bpc), Code: string(bpc), NameRU: "BPC", DefaultUnit: MG, Route: "п/к", Icon: "beaker"},
	{ID: CompoundID(glycine), Code: string(glycine), NameRU: "GLYCINE", DefaultUnit: MG, Route: "п/к", Icon: "beaker"},
}

func rowPlan(status ProtocolStatus) Plan {
	return Plan{
		Protocol: Protocol{
			ID:        rowPID,
			PatientID: UserID("p"),
			StartDate: rowStart,
			Weeks:     12,
			Status:    status,
		},
		Items:  rowItems,
		Phases: rowPhases,
	}
}

func rowsOn(today Date, logged []LoggedSlot, status ProtocolStatus) []ProtocolRow {
	return WeekProtocolRows(rowPlan(status), rowCompounds, logged, today)
}

func rowFor(t *testing.T, rows []ProtocolRow, item ProtocolItemID) ProtocolRow {
	t.Helper()
	for _, row := range rows {
		if row.ItemID == item {
			return row
		}
	}
	t.Fatalf("no row for %v", item)
	return ProtocolRow{}
}

func TestEveryItemOfTheProtocolGetsARowWhateverDayItIs(t *testing.T) {
	// The strip is «Протокол этой недели», not «сегодня»: a projection built from the day's
	// occurrences would drop the weekly injection six days out of seven.
	wednesday := NewDate(2026, time.May, 20)

	rows := rowsOn(wednesday, nil, StatusActive)

	want := []ProtocolItemID{sema, bpc, glycine}
	if len(rows) != len(want) {
		t.Fatalf("want %d rows, got %d", len(want), len(rows))
	}
	for i, id := range want {
		if rows[i].ItemID != id {
			t.Errorf("row %d: want %v got %v", i, id, rows[i].ItemID)
		}
	}
}

func TestTheRowCarriesTheDoseOfTheWeekItIsIn(t *testing.T) {
	// Not the protocol's first phase: the prototype's row is hardcoded «0,25 мг» for all
	// twelve weeks.
	for _, c := range []struct {
		today Date
		dose  Dose
	}{
		{NewDate(2026, time.May, 31), Dose{Value: 0.25, Unit: MG}},
		{NewDate(2026, time.June, 10), Dose{Value: 0.5, Unit: MG}},
		{NewDate(2026, time.July, 8), Dose{Value: 1.0, Unit: MG}},
	} {
		row := rowsOn(c.today, nil, StatusActive)[0]
		if row.Dose == nil || *row.Dose != c.dose {
			t.Errorf("%v: want %v got %v", c.today, c.dose, row.Dose)
		}
	}
}

func TestAnItemWithNoPhasesHasNoDoseRatherThanAWrongOne(t *testing.T) {
	row := rowFor(t, rowsOn(NewDate(2026, time.May, 20), nil, StatusActive), glycine)

	if row.Dose != nil {
		t.Errorf("glycine has no phases and so no dose, got %v", *row.Dose)
	}
}

func TestTodaysStatusFollowsTheDayOccurrenceAndNotTheWeek(t *testing.T) {
	sunday := NewDate(2026, time.May, 31)
	logged := []LoggedSlot{{ItemID: sema, Date: sunday, Time: slot(7, 0)}}

	before := rowFor(t, rowsOn(sunday, nil, StatusActive), sema)
	after := rowFor(t, rowsOn(sunday, logged, StatusActive), sema)

	if before.TodayStatus == nil || *before.TodayStatus != StatusPending {
		t.Errorf("pending before, got %v", before.TodayStatus)
	}
	if after.TodayStatus == nil || *after.TodayStatus != StatusDone {
		t.Errorf("done after, got %v", after.TodayStatus)
	}
}

func TestAnItemNotDueTodayHasNoStatusForToday(t *testing.T) {
	wednesday := NewDate(2026, time.May, 20)
	rows := rowsOn(wednesday, nil, StatusActive)

	if row := rowFor(t, rows, sema); row.TodayStatus != nil {
		t.Errorf("Семаглутид is not due on a Wednesday, got %v", *row.TodayStatus)
	}
	if row := rowFor(t, rows, bpc); row.TodayStatus == nil {
		t.Error("BPC-157 is due every day")
	}
}

func TestARowNamesItsCompoundRatherThanCarryingAnID(t *testing.T) {
	row := rowsOn(NewDate(2026, time.May, 20), nil, StatusActive)[0]

	if row.Compound == nil || row.Compound.NameRU != "SEMA" {
		t.Errorf("the row carries the name, got %v", row.Compound)
	}
}

func TestEachRowGetsItsOwnCompoundAndNotTheFirstOne(t *testing.T) {
	// An earlier version passed an empty compound list, asserting "no compounds" rather
	// than the per-row mapping.
	rows := rowsOn(NewDate(2026, time.May, 20), nil, StatusActive)

	for i, name := range []string{"SEMA", "BPC", "GLYCINE"} {
		if rows[i].Compound == nil || rows[i].Compound.NameRU != name {
			t.Errorf("row %d: want %s got %v", i, name, rows[i].Compound)
		}
	}
}

func TestAnItemReferencingAnUnknownCompoundHasNoneRatherThanTheWrongOne(t *testing.T) {
	var onlyBPC []Compound
	for _, c := range rowCompounds {
		if c.ID == CompoundID(bpc) {
			onlyBPC = append(onlyBPC, c)
		}
	}

	rows := WeekProtocolRows(rowPlan(StatusActive), onlyBPC, nil, NewDate(2026, time.May, 20))

	if row := rowFor(t, rows, sema); row.Compound != nil {
		t.Errorf("no compound rather than the wrong one, got %v", row.Compound)
	}
	if row := rowFor(t, rows, bpc); row.Compound == nil || row.Compound.NameRU != "BPC" {
		t.Errorf("BPC still resolves, got %v", row.Compound)
	}
}

func TestTheRowCarriesTheKindTheStripDrawsItBy(t *testing.T) {
	rows := rowsOn(NewDate(2026, time.May, 20), nil, StatusActive)

	for i, kind := range []ItemKind{KindInjection, KindInjection, KindSupplement} {
		if rows[i].Kind != kind {
			t.Errorf("row %d: want %v got %v", i, kind, rows[i].Kind)
		}
	}
}

func TestACancelledProtocolHasNoStrip(t *testing.T) {
	if got := rowsOn(NewDate(2026, time.May, 20), nil, StatusCancelled); len(got) != 0 {
		t.Errorf("a cancelled course has no strip, got %v", got)
	}
}

func TestADateOutsideTheCycleHasNoStrip(t *testing.T) {
	if got := rowsOn(NewDate(2026, time.May, 9), nil, StatusActive); len(got) != 0 {
		t.Errorf("before the course, got %v", got)
	}
	if got := rowsOn(NewDate(2026, time.August, 3), nil, StatusActive); len(got) != 0 {
		t.Errorf("after the course, got %v", got)
	}
}

func TestATwiceDailyItemIsNotDoneUntilBothSlotsAre(t *testing.T) {
	// The first slot was read for the whole day once: logging the morning injection said
	// «Сегодня · записано» with the 20:00 one still due.
	day := NewDate(2026, time.May, 20)
	bpcStatus := func(logged []LoggedSlot) OccurrenceStatus {
		row := rowFor(t, rowsOn(day, logged, StatusActive), bpc)
		if row.TodayStatus == nil {
			return OccurrenceStatus("")
		}
		return *row.TodayStatus
	}

	morning := []LoggedSlot{{ItemID: bpc, Date: day, Time: slot(8, 0)}}
	both := []LoggedSlot{
		{ItemID: bpc, Date: day, Time: slot(8, 0)},
		{ItemID: bpc, Date: day, Time: slot(20, 0)},
	}

	if got := bpcStatus(morning); got != StatusPending {
		t.Errorf("the evening injection is still due, got %v", got)
	}
	if got := bpcStatus(both); got != StatusDone {
		t.Errorf("both slots logged, got %v", got)
	}
	if got := bpcStatus(nil); got != StatusPending {
		t.Errorf("nothing logged at all, got %v", got)
	}
}

func TestTheStripCarriesWhichItemsCanBeLogged(t *testing.T) {
	// Guarded in the domain, not only through a UI test: a mapping that lives here should
	// fail here.
	loggableBy := map[ItemKind]bool{}
	for _, row := range rowsOn(NewDate(2026, time.May, 20), nil, StatusActive) {
		loggableBy[row.Kind] = row.Loggable
	}

	if len(loggableBy) != 2 || !loggableBy[KindInjection] || loggableBy[KindSupplement] {
		t.Errorf("an injection is loggable and a supplement is not, got %v", loggableBy)
	}
}

package protocol

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// The neighbours as this package sees them: three answers and nothing else. Stubbing them is
// the whole point of the interfaces living here — the aggregate's rules are testable without
// a database, and without dosing or inventory existing at all.
type stubNeighbours struct {
	site    string
	logged  []LoggedSlot
	left    *int
	reorder *ReorderHint

	askedFor []ProtocolItemID
	// What dose the cabinet was told to divide by — nil where no phase covers the day.
	askedAt []*Dose
	windows []civil.Range
	// Every subject the aggregate passed, so «it asks about the token's patient» is
	// measurable rather than traced: substituting the plan's own patient id survived the
	// whole suite while the fixture had data for one patient.
	subjects []civil.UserID

	rotationFails error
	dosesFail     error
	cabinetFails  error
}

func (s *stubNeighbours) SuggestNextSite(
	_ context.Context, _ pgx.Tx, patient civil.UserID,
) (string, error) {
	s.subjects = append(s.subjects, patient)

	return s.site, s.rotationFails
}

func (s *stubNeighbours) LoggedSlotsIn(
	_ context.Context, _ pgx.Tx, patient civil.UserID, window civil.Range,
) ([]LoggedSlot, error) {
	s.subjects = append(s.subjects, patient)
	s.windows = append(s.windows, window)

	return s.logged, s.dosesFail
}

func (s *stubNeighbours) SupplyFor(
	_ context.Context, _ pgx.Tx, patient civil.UserID, item ProtocolItem, dose *Dose,
	_ civil.Date,
) (*int, *ReorderHint, error) {
	s.subjects = append(s.subjects, patient)
	s.askedFor = append(s.askedFor, item.ID)
	s.askedAt = append(s.askedAt, dose)

	return s.left, s.reorder, s.cabinetFails
}

var theCompound = Compound{
	ID: "3c1f3b7c-0000-4000-8000-00000000000d", NameRU: "Семаглутид",
	DefaultUnit: MG, Route: "sc", Icon: "syringe",
}

func todayFor(t *testing.T, plan Plan, running bool, at civil.Date, clock civil.Slot, n *stubNeighbours) Today {
	t.Helper()

	today, err := TodayFor(t.Context(), nil, "3c1f3b7c-0000-4000-8000-0000000000a1",
		plan, []Compound{theCompound}, running, at, clock, n, n, n)
	if err != nil {
		t.Fatalf("assembling: %v", err)
	}

	return today
}

// A patient with no running course still has a screen: the day, the part of it, and the zone
// the wizard would open on. Everything else is a fact about a course, and inventing zeros for
// them would draw a card about a prescription that does not exist.
func TestAPatientWithNoCourseStillHasADay(t *testing.T) {
	n := &stubNeighbours{site: "l-abdomen"}

	today := todayFor(t, Plan{}, false, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 9}, n)

	if today.Date != civil.NewDate(2026, time.May, 10) || today.PartOfDay != Morning {
		t.Errorf("the day reads %v %q", today.Date, today.PartOfDay)
	}
	if today.SuggestedSite != "l-abdomen" {
		t.Errorf("the zone is %q", today.SuggestedSite)
	}
	if today.CycleWeek != nil || today.NextDose != nil || today.WeekProtocol != nil {
		t.Errorf("a course's facts are answered without one: %+v", today)
	}
	if today.VialDosesLeft != nil || today.Reorder != nil || today.NextTitration != nil {
		t.Errorf("the cabinet answered for a patient with no course: %+v", today)
	}
	if len(n.askedFor) != 0 {
		t.Errorf("the cabinet was asked about %v", n.askedFor)
	}
}

// The next dose is the next *injection* still open, and the strip is every item. A supplement
// is on the strip and is never the hero card's dose — the two readers disagree by design, and
// a single «next occurrence» would put a vitamin in the hero card.
func TestTheNextDoseIsAnInjectionAndTheStripIsEverything(t *testing.T) {
	n := &stubNeighbours{site: "r-glute"}

	// 10 May is the Sunday the injection falls on; the supplement is daily at 08:00 and
	// 20:00, so its morning slot comes first in the day.
	today := todayFor(t, aSchedulePlan(), true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if today.NextDose == nil {
		t.Fatal("nothing is next")
	}
	if today.NextDose.Kind != KindInjection {
		t.Errorf("the hero card's dose is a %s", today.NextDose.Kind)
	}
	if len(today.WeekProtocol) != 2 {
		t.Errorf("the strip holds %d rows, want both items", len(today.WeekProtocol))
	}
	if today.NextDoseCompound == nil || today.NextDoseCompound.NameRU != "Семаглутид" {
		t.Errorf("the hero card names %v", today.NextDoseCompound)
	}
	if today.CycleWeek == nil || *today.CycleWeek != 1 {
		t.Errorf("the week is %v", today.CycleWeek)
	}
}

// Logged today, and by the injection: the supplement's slots closing does not make «доза
// записана» true, and the hero card would say so if the flag were about any occurrence.
func TestTheDoseLoggedFlagIsAboutTheInjection(t *testing.T) {
	day := civil.NewDate(2026, time.May, 10)

	for _, state := range []struct {
		name   string
		logged []LoggedSlot
		want   bool
	}{
		{"nothing logged", nil, false},
		{
			"both of the supplement's slots", []LoggedSlot{
				{ItemID: "item-supplement", Date: day, Time: &civil.Slot{Hour: 8}},
				{ItemID: "item-supplement", Date: day, Time: &civil.Slot{Hour: 20}},
			}, false,
		},
		{
			"the injection", []LoggedSlot{
				{ItemID: "item-injection", Date: day, Time: &civil.Slot{Hour: 8}},
			}, true,
		},
	} {
		t.Run(state.name, func(t *testing.T) {
			n := &stubNeighbours{site: "r-glute", logged: state.logged}
			today := todayFor(t, aSchedulePlan(), true, day, civil.Slot{Hour: 21}, n)

			if today.DoseLoggedToday != state.want {
				t.Errorf("logged=%v, want %v", today.DoseLoggedToday, state.want)
			}
			// And once it is logged there is no next dose to offer.
			if state.want && today.NextDose != nil {
				t.Errorf("a dose is offered after one was logged: %+v", today.NextDose)
			}
		})
	}
}

// The cabinet is asked about the item the hero card names, and about no other. Asking per
// item would put a supplement's supply under an injection's card.
func TestTheCabinetIsAskedAboutTheDoseBeingOffered(t *testing.T) {
	left := 3
	n := &stubNeighbours{
		site: "r-glute", left: &left,
		reorder: &ReorderHint{CompoundID: theCompound.ID, WeeksLeft: 2},
	}

	today := todayFor(t, aSchedulePlan(), true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if len(n.askedFor) != 1 || n.askedFor[0] != "item-injection" {
		t.Errorf("the cabinet was asked about %v", n.askedFor)
	}
	// Which dose reached the cabinet, and not merely that one did: it was written into
	// the stub and never read, so passing the course's start date instead of the day
	// passed the whole suite. Written down rather than compared to the fixture's own
	// variable. That it is the *day's* phase is measured where the plan titrates — this
	// one has a single band, so every candidate is the same number.
	if len(n.askedAt) != 1 || n.askedAt[0] == nil ||
		n.askedAt[0].Value != 0.25 || n.askedAt[0].Unit != MG {
		t.Errorf("the cabinet was told to divide by %v, want 0,25 мг", dosesOf(n.askedAt))
	}
	if today.VialDosesLeft == nil || *today.VialDosesLeft != 3 {
		t.Errorf("the vial has %v doses left", today.VialDosesLeft)
	}
	if today.Reorder == nil || today.Reorder.WeeksLeft != 2 {
		t.Errorf("the reorder hint reads %+v", today.Reorder)
	}
}

// The history is read for the day the summary is about, and once. A window of a week would be
// read and then filtered to the same day, and a window of the wrong day would answer about
// somebody's yesterday.
func TestTheHistoryIsReadForThatDayAndOnce(t *testing.T) {
	n := &stubNeighbours{site: "r-glute"}
	day := civil.NewDate(2026, time.May, 10)

	todayFor(t, aSchedulePlan(), true, day, civil.Slot{Hour: 7}, n)

	if len(n.windows) != 1 {
		t.Fatalf("the history was read %d times", len(n.windows))
	}
	if n.windows[0].From != day || n.windows[0].Through != day {
		t.Errorf("the window is %v..%v, want that day", n.windows[0].From, n.windows[0].Through)
	}
}

// A neighbour that fails fails the whole answer: half a hero card is a screen that says
// something untrue about a patient's own treatment.
//
// One at a time, because the rotation is asked first: a single stub failing everything
// measured only that call, and dropping either of the other two error checks left it green.
func TestANeighbourThatFailsFailsTheAnswer(t *testing.T) {
	shut := errors.New("the cabinet is shut")

	for _, broken := range []struct {
		name string
		set  func(*stubNeighbours)
	}{
		{"the rotation", func(n *stubNeighbours) { n.rotationFails = shut }},
		{"the logged slots", func(n *stubNeighbours) { n.dosesFail = shut }},
		{"the cabinet", func(n *stubNeighbours) { n.cabinetFails = shut }},
	} {
		t.Run(broken.name, func(t *testing.T) {
			n := &stubNeighbours{site: "r-glute"}
			broken.set(n)

			if _, err := TodayFor(t.Context(), nil, "3c1f3b7c-0000-4000-8000-0000000000a1",
				aSchedulePlan(), nil, true, civil.NewDate(2026, time.May, 10),
				civil.Slot{Hour: 7}, n, n, n); !errors.Is(err, shut) {
				t.Errorf("got %v, want the neighbour's own error", err)
			}
		})
	}
}

// Every field of a context that is not built is absent, and none of them is a zero. «0 из 4
// приёмов» over a nutrition context that does not exist is a lie the client cannot detect.
func TestTheFieldsOfContextsThatDoNotExistAreAbsent(t *testing.T) {
	n := &stubNeighbours{site: "r-glute"}

	today := todayFor(t, aSchedulePlan(), true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if today.MealCount != nil || today.MealMacros != nil || today.Targets != nil {
		t.Errorf("nutrition answered %+v", today)
	}
	if today.WeightKG != nil || today.TargetWeightKG != nil || today.WeightSeries != nil {
		t.Errorf("measurements answered %+v", today)
	}
}

// Every neighbour is asked about the caller's own patient, and about no other subject. The
// plan carries a patient id of its own, so «pass the plan's» is a substitution that survived
// the whole suite while the fixture had data for one patient only.
func TestTheNeighboursAreAskedAboutTheCaller(t *testing.T) {
	n := &stubNeighbours{site: "r-glute", left: new(int)}
	const caller = civil.UserID("3c1f3b7c-0000-4000-8000-00000000beef")

	plan := aSchedulePlan()
	if plan.Protocol.PatientID == caller {
		t.Fatal("the fixture's patient is the caller, so a substitution is invisible")
	}

	if _, err := TodayFor(t.Context(), nil, caller, plan, nil, true,
		civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n, n, n); err != nil {
		t.Fatalf("assembling: %v", err)
	}

	if len(n.subjects) != 3 {
		t.Fatalf("the neighbours were asked %d times, want three", len(n.subjects))
	}
	for i, asked := range n.subjects {
		if asked != caller {
			t.Errorf("call %d asked about %s, want %s", i, asked, caller)
		}
	}
}

// Two injectables on one day is what this product is for, and the card names one of them.
// The rule is the clock, not the order the items came back in — items are read ORDER BY id,
// so «the first open one» offered whichever uuid sorted first, which could be the evening
// dose while the morning one was still open. Both orders are run because a fixture whose
// item order already agrees with the clock cannot tell the two rules apart.
func TestTheNextDoseIsTheEarliestOpenOneByTheClock(t *testing.T) {
	compound := CompoundID(theCompound.ID)
	// Two drugs on the morning-and-evening pair, and not one twice: two positions naming
	// one compound is the shape the cabinet refuses to count, and a fixture built that
	// way would run those subcases inside the silent branch. The half-past copy below
	// keeps the morning drug and so runs inside that branch on purpose: nothing it
	// asserts — which occurrence is next, and which item was asked about — reads a count.
	other := CompoundID("other-compound")
	morning := ProtocolItem{
		ID: "item-morning", Kind: KindInjection, CompoundID: &compound,
		Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
		Times: []civil.Slot{{Hour: 8}}, Loggable: true,
	}
	evening := ProtocolItem{
		ID: "item-evening", Kind: KindInjection, CompoundID: &other,
		Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
		Times: []civil.Slot{{Hour: 20}}, Loggable: true,
	}
	dose := Dose{Value: 0.25, Unit: MG}

	sameHour := morning
	sameHour.ID = "item-half-past"
	sameHour.Times = []civil.Slot{{Hour: 8, Minute: 30}}

	for _, order := range []struct {
		name  string
		items []ProtocolItem
	}{
		{"the evening item first", []ProtocolItem{evening, morning}},
		{"the morning item first", []ProtocolItem{morning, evening}},
		// Same hour, different minute: without the minute the comparison answers
		// «not earlier» for both, and the item order decides again.
		{"a later slot of the same hour first", []ProtocolItem{sameHour, morning}},
	} {
		t.Run(order.name, func(t *testing.T) {
			plan := Plan{
				Protocol: Protocol{
					StartDate: civil.NewDate(2026, time.May, 4), Weeks: 12, Status: StatusActive,
				},
				Items: order.items,
				Phases: map[ProtocolItemID][]ProtocolPhase{
					"item-morning":   {{FromWeek: 1, ToWeek: 12, Dose: dose}},
					"item-evening":   {{FromWeek: 1, ToWeek: 12, Dose: dose}},
					"item-half-past": {{FromWeek: 1, ToWeek: 12, Dose: dose}},
				},
			}
			n := &stubNeighbours{site: "l-abdomen"}

			today := todayFor(t, plan, true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

			if today.NextDose == nil {
				t.Fatal("no dose is named on a day prescribing two")
			}
			if today.NextDose.ItemID != "item-morning" || today.NextDose.Time.Hour != 8 {
				t.Errorf("the card names %s at %v", today.NextDose.ItemID, today.NextDose.Time)
			}
			// The cabinet is asked about the item the card names and no other: two
			// injectables mean two possible answers, and the wrong one is the wrong
			// remaining count on the screen.
			if len(n.askedFor) != 1 || n.askedFor[0] != "item-morning" {
				t.Errorf("the cabinet was asked about %v", n.askedFor)
			}
		})
	}
}

// The morning dose logged, the evening one still open: the card moves on rather than staying
// on the earliest slot of the day, and «доза записана» is true from the first of the two.
func TestTheCardMovesToTheEveningOnceTheMorningIsLogged(t *testing.T) {
	compound := CompoundID(theCompound.ID)
	dose := Dose{Value: 0.25, Unit: MG}
	other := CompoundID("other-compound")
	plan := Plan{
		Protocol: Protocol{
			StartDate: civil.NewDate(2026, time.May, 4), Weeks: 12, Status: StatusActive,
		},
		Items: []ProtocolItem{
			{
				ID: "item-morning", Kind: KindInjection, CompoundID: &compound,
				Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
				Times: []civil.Slot{{Hour: 8}}, Loggable: true,
			},
			{
				// A second drug rather than the same one twice: one compound on two
				// positions is what the cabinet refuses to count, and this case is
				// about the card moving on, not about that silence.
				ID: "item-evening", Kind: KindInjection, CompoundID: &other,
				Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
				Times: []civil.Slot{{Hour: 20}}, Loggable: true,
			},
		},
		Phases: map[ProtocolItemID][]ProtocolPhase{
			"item-morning": {{FromWeek: 1, ToWeek: 12, Dose: dose}},
			"item-evening": {{FromWeek: 1, ToWeek: 12, Dose: dose}},
		},
	}
	n := &stubNeighbours{site: "l-abdomen", logged: []LoggedSlot{{
		ItemID: "item-morning", Date: civil.NewDate(2026, time.May, 10),
		Time: &civil.Slot{Hour: 8},
	}}}

	today := todayFor(t, plan, true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 9}, n)

	if today.NextDose == nil || today.NextDose.ItemID != "item-evening" {
		t.Errorf("the card names %+v", today.NextDose)
	}
	if !today.DoseLoggedToday {
		t.Error("the day is not marked as carrying a dose")
	}
}

// A day inside the course that no phase covers.
//
// Gaps between phases are legal and deliberately so, and on such a day nothing is
// prescribed — so the cabinet is asked with no dose, and the card answers substance and
// status without a count of injections. The interface says this in its own comment and
// nothing measured it.
func TestOnADayNoPhaseCoversTheCabinetIsAskedWithNoDose(t *testing.T) {
	plan := aSchedulePlan()
	// Prescribed in weeks 1-2 and again from week 6. The course starts on 4 May and the
	// injection falls on Sundays, so 24 May is week three's — inside the course, inside
	// the gap, and an occurrence is still generated for it with no dose on it.
	plan.Phases["item-injection"] = []ProtocolPhase{
		{FromWeek: 1, ToWeek: 2, Dose: Dose{Value: 0.25, Unit: MG}},
		{FromWeek: 6, ToWeek: 12, Dose: Dose{Value: 1, Unit: MG}},
	}

	n := &stubNeighbours{}
	todayFor(t, plan, true, civil.NewDate(2026, time.May, 24), civil.Slot{Hour: 7}, n)

	if len(n.askedAt) != 1 || n.askedAt[0] != nil {
		t.Errorf("the cabinet was told to divide by %v, want nothing", dosesOf(n.askedAt))
	}
	// And no assertion on the card's own count here: the aggregate hands through
	// whatever the cabinet answers, so with a stub it would measure the stub. That a
	// dose of nothing buys no count is inventory's, at math_test.go's nil-dose case.
}

// The drug the cabinet is asked about is the one the hero card offers, not whichever the course
// happens to list first.
//
// Resolving the divisor from the drug rather than from the position made «which drug» a
// question worth asking, and no fixture asked it: the two positions here carry different drugs
// and different doses, and the one that is next is listed second, so an implementation reading
// the plan's first position answers a milligram where the card offers a quarter of one.
func TestTheCabinetIsAskedWithTheDoseOfTheDrugBeingOffered(t *testing.T) {
	compound, other := CompoundID(theCompound.ID), CompoundID("other-compound")
	plan := Plan{
		Protocol: Protocol{
			PatientID: "3c1f3b7c-0000-4000-8000-0000000000a1",
			StartDate: civil.NewDate(2026, time.May, 4), Weeks: 12, Status: StatusActive,
		},
		Items: []ProtocolItem{
			{
				ID: "item-evening", Kind: KindInjection, CompoundID: &other,
				Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
				Times: []civil.Slot{{Hour: 20}}, Loggable: true,
			},
			{
				ID: "item-morning", Kind: KindInjection, CompoundID: &compound,
				Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
				Times: []civil.Slot{{Hour: 8}}, Loggable: true,
			},
		},
		Phases: map[ProtocolItemID][]ProtocolPhase{
			"item-evening": {{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 1, Unit: MG}}},
			"item-morning": {{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 0.25, Unit: MG}}},
		},
	}

	n := &stubNeighbours{site: "r-glute"}
	todayFor(t, plan, true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if len(n.askedFor) != 1 || n.askedFor[0] != "item-morning" {
		t.Fatalf("the cabinet was asked about %v, want the morning position", n.askedFor)
	}
	if len(n.askedAt) != 1 || n.askedAt[0] == nil || n.askedAt[0].Value != 0.25 {
		t.Errorf("the cabinet was told to divide by %v, want the morning drug's 0,25 мг", dosesOf(n.askedAt))
	}
}

// Two course positions of one drug, and the day card goes quiet about it — the same answer the
// cabinet gives, and for the same reason: the rate is a position's while the vials are the
// drug's, so a count divided by one of the two doses contradicts the other half of the
// prescription. Measured through the stub, which records what it was asked to divide by.
func TestTwoPositionsOfOneDrugLeaveTheDayCardWithNoDivisor(t *testing.T) {
	plan := aSchedulePlan()
	evening := ProtocolItemID("item-injection-evening")
	compound := CompoundID(theCompound.ID)
	plan.Items = append(plan.Items, ProtocolItem{
		ID: evening, Kind: KindInjection, CompoundID: &compound,
		Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
		Times: []civil.Slot{{Hour: 19}}, Loggable: true,
	})
	plan.Phases[evening] = []ProtocolPhase{{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 0.5, Unit: MG}}}

	left := 3
	n := &stubNeighbours{site: "r-glute", left: &left}
	today := todayFor(t, plan, true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if len(n.askedAt) != 1 || n.askedAt[0] != nil {
		t.Errorf("the cabinet was told to divide by %v, want nothing", dosesOf(n.askedAt))
	}
	// And the premise: each position on its own does prescribe today, so the silence is
	// the ambiguity rule rather than a day nothing covers.
	if one, other := PhaseDose(plan, "item-injection", today.Date), PhaseDose(plan, evening, today.Date); one == nil || other == nil {
		t.Fatalf("the fixture prescribes %v and %v; both positions must carry a dose", one, other)
	}
}

// The divisor is the band covering this day, and the fixture has to titrate before that
// is measurable: with one band the day's phase, the plan's first and the item's only are
// the same number, and all three survive.
func TestTheCabinetDividesByTheDoseOfTheDaysOwnPhase(t *testing.T) {
	// The course opens on Monday 4 May and the injection falls on Sundays: 10 May is
	// week one's and 5 July is week nine's.
	for _, day := range []struct {
		name string
		at   civil.Date
		want Dose
	}{
		{"a day in the opening band", civil.NewDate(2026, time.May, 10), Dose{Value: 0.25, Unit: MG}},
		{"a day past the titration", civil.NewDate(2026, time.July, 5), Dose{Value: 1, Unit: MG}},
	} {
		t.Run(day.name, func(t *testing.T) {
			plan := aSchedulePlan()
			plan.Phases["item-injection"] = []ProtocolPhase{
				{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
				{FromWeek: 5, ToWeek: 12, Dose: Dose{Value: 1, Unit: MG}},
			}

			n := &stubNeighbours{site: "r-glute"}
			todayFor(t, plan, true, day.at, civil.Slot{Hour: 7}, n)

			if len(n.askedAt) != 1 || n.askedAt[0] == nil || *n.askedAt[0] != day.want {
				t.Errorf("the cabinet was told to divide by %v, want %v %s",
					dosesOf(n.askedAt), day.want.Value, day.want.Unit)
			}
		})
	}
}

// A []*Dose printed with %+v is a list of addresses, which is what these failures said until
// somebody read one.
func dosesOf(asked []*Dose) []string {
	out := make([]string, 0, len(asked))
	for _, dose := range asked {
		if dose == nil {
			out = append(out, "nothing")

			continue
		}
		out = append(out, fmt.Sprintf("%v %s", dose.Value, dose.Unit))
	}

	return out
}

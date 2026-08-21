package protocol

import (
	"testing"
	"time"
)

var (
	bandsStart = NewDate(2026, time.May, 10)
	weighIn    = ProtocolItemID("weigh")
	quarter    = Dose{Value: 0.25, Unit: MG}
	half       = Dose{Value: 0.5, Unit: MG}
	whole      = Dose{Value: 1.0, Unit: MG}
)

// Out of order on purpose: §03 doesn't promise the server sorts them, and a sorted fixture
// lets a missing sort through.
var shuffledPhases = []ProtocolPhase{
	{FromWeek: 5, ToWeek: 8, Dose: half},
	{FromWeek: 9, ToWeek: 12, Dose: whole},
	{FromWeek: 1, ToWeek: 4, Dose: quarter},
}

// Four weeks on, four weeks off, four weeks on — a deliberate wash-out.
var brokenPhases = []ProtocolPhase{
	{FromWeek: 1, ToWeek: 4, Dose: quarter},
	{FromWeek: 9, ToWeek: 12, Dose: whole},
}

func bandsPlan(status ProtocolStatus, phases []ProtocolPhase) Plan {
	return Plan{
		Protocol: Protocol{
			ID:        ProtocolID("pr"),
			PatientID: UserID("p"),
			StartDate: bandsStart,
			Weeks:     12,
			Status:    status,
		},
		Items: []ProtocolItem{{
			ID:         sema,
			ProtocolID: ProtocolID("pr"),
			Kind:       KindInjection,
			CompoundID: compoundPtr("sema"),
			Cadence:    CadenceWeekly,
			DaysOfWeek: []time.Weekday{time.Sunday},
			Times:      []Slot{{Hour: 7}},
			Loggable:   true,
		}},
		Phases: map[ProtocolItemID][]ProtocolPhase{sema: phases},
	}
}

func defaultBandsPlan() Plan { return bandsPlan(StatusActive, shuffledPhases) }

// The whole course and a little either side, so nothing is clipped by accident.
var wholeCourse = Range{From: NewDate(2026, time.May, 1), Through: NewDate(2026, time.August, 31)}

type span struct {
	dose    Dose
	from    Date
	through Date
}

func spansOf(r Range, p Plan) []span {
	var out []span
	for _, b := range DoseBands(p, sema, r) {
		out = append(out, span{b.Dose, b.Range.From, b.Range.Through})
	}
	return out
}

func assertSpans(t *testing.T, want, got []span) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("want %d bands %v, got %d %v", len(want), want, len(got), got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("band %d: want %v got %v", i, want[i], got[i])
		}
	}
}

// MockSeed's protocol, reproduced rather than imported: the KMP mock lives in commonMain,
// and these are the numbers its assertions turn on.
func seedPlan() Plan {
	return Plan{
		Protocol: Protocol{
			ID:        ProtocolID("seed"),
			PatientID: UserID("seed-p"),
			StartDate: NewDate(2026, time.May, 10),
			Weeks:     12,
			Status:    StatusActive,
		},
		Items: []ProtocolItem{
			{
				ID:         seedSema,
				ProtocolID: ProtocolID("seed"),
				Kind:       KindInjection,
				CompoundID: compoundPtr("sema"),
				Cadence:    CadenceWeekly,
				DaysOfWeek: []time.Weekday{time.Sunday},
				Times:      []Slot{{Hour: 7}},
				Loggable:   true,
			},
			{
				ID:         seedBPC,
				ProtocolID: ProtocolID("seed"),
				Kind:       KindInjection,
				CompoundID: compoundPtr("bpc"),
				Cadence:    CadenceDaily,
				Times:      []Slot{{Hour: 8}, {Hour: 20}},
				Loggable:   true,
			},
		},
		Phases: map[ProtocolItemID][]ProtocolPhase{
			seedSema: {
				{FromWeek: 1, ToWeek: 4, Dose: quarter},
				{FromWeek: 5, ToWeek: 8, Dose: half},
				{FromWeek: 9, ToWeek: 12, Dose: whole},
			},
			seedBPC: {{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 250.0, Unit: MCG}}},
		},
	}
}

var (
	seedSema      = ProtocolItemID("item-sema")
	seedBPC       = ProtocolItemID("item-bpc")
	seededThrough = NewDate(2026, time.May, 30)
)

// TrendWindow.CYCLE.rangeOn, inlined: bounded at both ends by the course itself.
func cycleRange(p Protocol, today Date) (Range, bool) {
	through := MinDate(today, p.LastPrescribedDay())
	if through.Before(p.StartDate) {
		return Range{}, false
	}
	return Range{From: p.StartDate, Through: through}, true
}

func TestEachPhaseIsOneBandRunningFromItsFirstDayToItsLast(t *testing.T) {
	// Week N begins (N-1)×7 days after start, matching schedule/data.ts. The trends
	// module's DOSE_SPANS disagrees — see the seed test at the bottom of this file.
	assertSpans(t, []span{
		{quarter, NewDate(2026, time.May, 10), NewDate(2026, time.June, 6)},
		{half, NewDate(2026, time.June, 7), NewDate(2026, time.July, 4)},
		{whole, NewDate(2026, time.July, 5), NewDate(2026, time.August, 1)},
	}, spansOf(wholeCourse, defaultBandsPlan()))
}

func TestTheBandsMeetWithoutOverlappingAndCoverTheCourseExactly(t *testing.T) {
	bands := DoseBands(defaultBandsPlan(), sema, wholeCourse)

	if bands[0].Range.From != bandsStart {
		t.Errorf("the first band opens on day 0, got %v", bands[0].Range.From)
	}
	if last := bands[len(bands)-1].Range.Through; last != NewDate(2026, time.August, 1) {
		t.Errorf("day 83, the last prescribed day: got %v", last)
	}
	for i := 1; i < len(bands); i++ {
		if got := bands[i-1].Range.Through.DaysUntil(bands[i].Range.From); got != 1 {
			t.Errorf("one band ends the day before the next begins: got %d", got)
		}
	}
	total := 0
	for _, b := range bands {
		total += b.Range.Days()
	}
	if total != 84 {
		t.Errorf("twelve weeks, no day counted twice: got %d", total)
	}
}

func TestTheBandCoveringADayCarriesTheDoseThatDayWasPrescribed(t *testing.T) {
	// Asserted day by day, not at four dates a literal test happens to name. Run over the
	// gapped course too: on a contiguous course PhaseDose is never nil, so the "no band
	// over an unprescribed day" half isn't exercised by the default plan alone.
	for _, p := range []Plan{defaultBandsPlan(), bandsPlan(StatusActive, brokenPhases)} {
		bands := DoseBands(p, sema, wholeCourse)
		for offset := 0; offset <= 83; offset++ {
			day := p.Protocol.StartDate.AddDays(offset)

			var covering []Dose
			for _, b := range bands {
				if b.Range.Contains(day) {
					covering = append(covering, b.Dose)
				}
			}

			var want []Dose
			if dose := PhaseDose(p, sema, day); dose != nil {
				want = append(want, *dose)
			}
			if len(covering) != len(want) || (len(want) == 1 && covering[0] != want[0]) {
				t.Fatalf("one band, carrying the prescribed dose, on %v: want %v got %v", day, want, covering)
			}
		}
	}
}

func TestAGapBetweenTwoPhasesIsAGapBetweenTwoBands(t *testing.T) {
	// Makes phase.ToWeek observable: a band closing where the *next* phase opens, instead
	// of where its own ends, draws the same spans for a contiguous course — only a gap
	// separates the two implementations. Do not let it become contiguous.
	broken := bandsPlan(StatusActive, brokenPhases)

	assertSpans(t, []span{
		{quarter, NewDate(2026, time.May, 10), NewDate(2026, time.June, 6)},
		{whole, NewDate(2026, time.July, 5), NewDate(2026, time.August, 1)},
	}, spansOf(wholeCourse, broken))

	// Weeks 5–8 are prescribed nothing, and nothing is drawn over them.
	gap := Range{From: NewDate(2026, time.June, 7), Through: NewDate(2026, time.July, 4)}
	if got := spansOf(gap, broken); len(got) != 0 {
		t.Errorf("nothing is drawn over a wash-out, got %v", got)
	}
}

func TestABandIsWhatWasPrescribedAndNotWhatWasTaken(t *testing.T) {
	// No logged slot reaches this function: the seed's history stops at seededThrough
	// holding one dose, but the bands run two months further with three different doses.
	bands := DoseBands(seedPlan(), seedSema, wholeCourse)

	var doses []Dose
	for _, b := range bands {
		doses = append(doses, b.Dose)
	}
	if len(doses) != 3 || doses[0] != quarter || doses[1] != half || doses[2] != whole {
		t.Fatalf("three prescribed doses, got %v", doses)
	}
	if !bands[len(bands)-1].Range.Through.After(seededThrough) {
		t.Error("the prescription outruns the history it was measured against")
	}
}

func TestAWindowInsideOnePhaseSeesThatPhaseClippedToIt(t *testing.T) {
	r := Range{From: NewDate(2026, time.May, 25), Through: NewDate(2026, time.May, 31)}

	assertSpans(t, []span{{quarter, r.From, r.Through}}, spansOf(r, defaultBandsPlan()))
}

func TestAWindowStraddlingATitrationSeesBothBandsAndTheJoinBetweenThem(t *testing.T) {
	r := Range{From: NewDate(2026, time.June, 4), Through: NewDate(2026, time.June, 10)}

	assertSpans(t, []span{
		{quarter, NewDate(2026, time.June, 4), NewDate(2026, time.June, 6)},
		{half, NewDate(2026, time.June, 7), NewDate(2026, time.June, 10)},
	}, spansOf(r, defaultBandsPlan()))
}

func TestAWindowThatMissesTheCourseEntirelyHasNoBands(t *testing.T) {
	// Before it, and after it.
	before := Range{From: NewDate(2026, time.March, 1), Through: NewDate(2026, time.May, 9)}
	after := Range{From: NewDate(2026, time.August, 2), Through: NewDate(2026, time.September, 1)}

	if got := spansOf(before, defaultBandsPlan()); len(got) != 0 {
		t.Errorf("before the course: got %v", got)
	}
	if got := spansOf(after, defaultBandsPlan()); len(got) != 0 {
		t.Errorf("after the course: got %v", got)
	}
}

func TestAWindowTouchingTheCourseByOneDaySeesOneDayOfIt(t *testing.T) {
	// A one-day band is what «весь цикл» asks for on the day a course begins; an overlap
	// test written `from >= through` would leave a patient's first day with no
	// prescription under it.
	opening := Range{From: NewDate(2026, time.March, 1), Through: bandsStart}
	closing := Range{From: NewDate(2026, time.August, 1), Through: NewDate(2026, time.September, 1)}

	assertSpans(t, []span{{quarter, bandsStart, bandsStart}}, spansOf(opening, defaultBandsPlan()))
	assertSpans(t, []span{
		{whole, NewDate(2026, time.August, 1), NewDate(2026, time.August, 1)},
	}, spansOf(closing, defaultBandsPlan()))
}

func TestTheWeeksAreCountedFromWhicheverDayTheCourseBeganOn(t *testing.T) {
	// Every other fixture here starts on 10 May 2026, so a baked-in date would pass them
	// all. This one crosses a new year and a leap February — measured, not assumed.
	leapYear := defaultBandsPlan()
	leapYear.Protocol.StartDate = NewDate(2027, time.December, 20)
	leapYear.Items = nil
	window := Range{From: NewDate(2027, time.January, 1), Through: NewDate(2029, time.January, 1)}

	assertSpans(t, []span{
		// Day 0 → 27, across the new year.
		{quarter, NewDate(2027, time.December, 20), NewDate(2028, time.January, 16)},
		// Day 28 → 55.
		{half, NewDate(2028, time.January, 17), NewDate(2028, time.February, 13)},
		// Day 56 → 83, across 29 February: 2028 is a leap year, so this band moves under a
		// 365-day assumption.
		{whole, NewDate(2028, time.February, 14), NewDate(2028, time.March, 12)},
	}, spansOf(window, leapYear))
}

func TestACancelledCourseDrawsNoBandsAtAll(t *testing.T) {
	// Same answer PhaseDose gives, for the same reason.
	cancelled := bandsPlan(StatusCancelled, shuffledPhases)

	if got := spansOf(wholeCourse, cancelled); len(got) != 0 {
		t.Errorf("bands on a cancelled course: %v", got)
	}
	if got := ProtocolMarks(cancelled, sema, wholeCourse); len(got) != 0 {
		t.Errorf("marks on a cancelled course: %v", got)
	}
}

func TestAnItemWithNoPhasesHasNoBands(t *testing.T) {
	// Two different absences, different branches: unheard-of in the phase map (weigh-in)
	// vs. present with an empty list.
	if got := DoseBands(defaultBandsPlan(), weighIn, wholeCourse); len(got) != 0 {
		t.Errorf("bands for an item with no phase row: %v", got)
	}
	if got := ProtocolMarks(defaultBandsPlan(), weighIn, wholeCourse); len(got) != 0 {
		t.Errorf("marks for an item with no phase row: %v", got)
	}

	dosedNothing := bandsPlan(StatusActive, []ProtocolPhase{})
	if got := DoseBands(dosedNothing, sema, wholeCourse); len(got) != 0 {
		t.Errorf("bands for an empty phase list: %v", got)
	}
	if got := ProtocolMarks(dosedNothing, sema, wholeCourse); len(got) != 0 {
		t.Errorf("marks for an empty phase list: %v", got)
	}
}

func TestAPhaseReachingPastTheCourseIsCutAtItsLastDay(t *testing.T) {
	// Malformed, not impossible: §03 leaves `to_week`/`weeks` unjoined, so week 20 of a
	// twelve-week course is representable.
	overrun := bandsPlan(StatusActive, []ProtocolPhase{{FromWeek: 1, ToWeek: 20, Dose: quarter}})

	assertSpans(t, []span{{quarter, bandsStart, NewDate(2026, time.August, 1)}}, spansOf(wholeCourse, overrun))
}

func TestTheCourseIsMarkedWhereItStartedAndWhereEachDoseWentUp(t *testing.T) {
	marks := ProtocolMarks(defaultBandsPlan(), sema, wholeCourse)

	assertMarks(t, []ProtocolMark{
		{Kind: MarkStart, Date: bandsStart, From: nil, To: quarter},
		{Kind: MarkTitration, Date: NewDate(2026, time.June, 7), From: &quarter, To: half},
		{Kind: MarkTitration, Date: NewDate(2026, time.July, 5), From: &half, To: whole},
	}, marks)
}

func TestAMarkSitsOnTheFirstDayOfTheBandItOpens(t *testing.T) {
	bands := DoseBands(defaultBandsPlan(), sema, wholeCourse)

	var titrations []ProtocolMark
	for _, m := range ProtocolMarks(defaultBandsPlan(), sema, wholeCourse) {
		if m.Kind == MarkTitration {
			titrations = append(titrations, m)
		}
	}

	if len(titrations) != len(bands)-1 {
		t.Fatalf("one titration per band after the first: %d bands, %d marks", len(bands), len(titrations))
	}
	for i, m := range titrations {
		if m.Date != bands[i+1].Range.From {
			t.Errorf("mark %d sits at %v, band opens %v", i, m.Date, bands[i+1].Range.From)
		}
		if m.To != bands[i+1].Dose {
			t.Errorf("mark %d rises to %v, band carries %v", i, m.To, bands[i+1].Dose)
		}
	}
}

func TestMarksOutsideTheWindowAreLeftOut(t *testing.T) {
	// The seeded «весь цикл» window: three weeks lived, so the start is in and both
	// titrations are still ahead.
	seed := seedPlan()
	cycle, ok := cycleRange(seed.Protocol, NewDate(2026, time.May, 31))
	if !ok {
		t.Fatal("the seeded course has a cycle window")
	}

	marks := ProtocolMarks(seed, seedSema, cycle)

	if len(marks) != 1 || marks[0].Kind != MarkStart {
		t.Fatalf("only the start is inside the window, got %v", marks)
	}
	if marks[0].Date != seed.Protocol.StartDate {
		t.Errorf("the start sits on the cycle start, got %v", marks[0].Date)
	}
}

func TestBothEdgesOfTheWindowKeepTheirMarks(t *testing.T) {
	// A «7 дней» window can close exactly on the day a dose went up. The left edge is
	// measured by the test above, where the start sits on range.From.
	r := Range{From: NewDate(2026, time.June, 1), Through: NewDate(2026, time.June, 7)}

	marks := ProtocolMarks(defaultBandsPlan(), sema, r)

	if len(marks) != 1 || marks[0].Date != NewDate(2026, time.June, 7) {
		t.Fatalf("the closing edge keeps its mark, got %v", marks)
	}
}

func TestAMarkPastTheEndOfTheCourseIsDroppedLikeTheBandUnderIt(t *testing.T) {
	// Second phase entirely outside a twelve-week course: its band is cut away, so a
	// dashed line left behind would stand over nothing.
	overrun := bandsPlan(StatusActive, []ProtocolPhase{
		{FromWeek: 1, ToWeek: 12, Dose: quarter},
		{FromWeek: 13, ToWeek: 20, Dose: half},
	})

	assertSpans(t, []span{{quarter, bandsStart, NewDate(2026, time.August, 1)}}, spansOf(wholeCourse, overrun))

	marks := ProtocolMarks(overrun, sema, wholeCourse)
	if len(marks) != 1 || marks[0].Kind != MarkStart {
		t.Errorf("only the start survives, got %v", marks)
	}
	// The third reader agrees: TitrationStepAfter is what the schedule screen asks, and a
	// step past the course's end would promise «Доза растёт» after the chart drops the
	// line. The clip lives in TitrationSteps so all three inherit it.
	if got := TitrationSteps(overrun, sema); len(got) != 0 {
		t.Errorf("a step past the end: %v", got)
	}
	if got := TitrationStepAfter(overrun, sema, NewDate(2026, time.May, 31)); got != nil {
		t.Errorf("a next step past the end: %v", got)
	}
}

func TestTwoPhasesHoldingTheSameDoseAreNotATitration(t *testing.T) {
	// §03 lets a course split its phase table without changing the dose. Two bands, one
	// dose, no dashed line: «0,5 мг → 0,5 мг» is a change that didn't happen.
	held := bandsPlan(StatusActive, []ProtocolPhase{
		{FromWeek: 1, ToWeek: 4, Dose: half},
		{FromWeek: 5, ToWeek: 8, Dose: half},
	})

	if got := len(DoseBands(held, sema, wholeCourse)); got != 2 {
		t.Errorf("two bands, got %d", got)
	}
	marks := ProtocolMarks(held, sema, wholeCourse)
	if len(marks) != 1 || marks[0].Kind != MarkStart {
		t.Errorf("no dashed line, got %v", marks)
	}
	if got := TitrationSteps(held, sema); len(got) != 0 {
		t.Errorf("holding a dose is not a step, got %v", got)
	}
}

func TestACancelledCourseHasNoNextStepEither(t *testing.T) {
	// The third reader of the same phases: a withdrawn prescription must not still promise
	// «Доза растёт».
	cancelled := bandsPlan(StatusCancelled, shuffledPhases)

	if got := TitrationSteps(cancelled, sema); len(got) != 0 {
		t.Errorf("steps on a cancelled course: %v", got)
	}
	if got := TitrationStepAfter(cancelled, sema, NewDate(2026, time.May, 31)); got != nil {
		t.Errorf("a next step on a cancelled course: %v", got)
	}
}

func TestAWindowOpeningMidCourseCarriesNoStartMark(t *testing.T) {
	r := Range{From: NewDate(2026, time.June, 1), Through: NewDate(2026, time.July, 31)}

	marks := ProtocolMarks(defaultBandsPlan(), sema, r)

	if len(marks) != 2 ||
		marks[0].Date != NewDate(2026, time.June, 7) ||
		marks[1].Date != NewDate(2026, time.July, 5) {
		t.Fatalf("both titrations and nothing else, got %v", marks)
	}
	for _, m := range marks {
		if m.Kind == MarkStart {
			t.Error("the start is outside this window")
		}
	}
}

func TestAnItemDosedFromTheSecondWeekIsMarkedWhereItsFirstBandOpens(t *testing.T) {
	// §03 lets a phase begin after week 1 (a wash-in). The start belongs on the day the
	// item was first prescribed, not the protocol's own start date.
	washIn := bandsPlan(StatusActive, []ProtocolPhase{
		{FromWeek: 2, ToWeek: 4, Dose: quarter},
		{FromWeek: 5, ToWeek: 8, Dose: half},
	})

	assertMarks(t, []ProtocolMark{
		// Day 7, not day 0.
		{Kind: MarkStart, Date: NewDate(2026, time.May, 17), From: nil, To: quarter},
		{Kind: MarkTitration, Date: NewDate(2026, time.June, 7), From: &quarter, To: half},
	}, ProtocolMarks(washIn, sema, wholeCourse))

	// Bands under it: the first opens the same day as the mark, the last closes at its own
	// phase's end, not running on to the end of a course it doesn't cover.
	bands := DoseBands(washIn, sema, wholeCourse)
	if bands[0].Range.From != NewDate(2026, time.May, 17) {
		t.Errorf("the first band opens with the mark, got %v", bands[0].Range.From)
	}
	if last := bands[len(bands)-1].Range.Through; last != NewDate(2026, time.July, 4) {
		t.Errorf("the last band closes at its own phase's end, got %v", last)
	}
}

func TestAnItemWithOneBandStartsAndNeverTitrates(t *testing.T) {
	marks := ProtocolMarks(seedPlan(), seedBPC, wholeCourse)

	if len(marks) != 1 || marks[0].Kind != MarkStart {
		t.Fatalf("one mark, a start, got %v", marks)
	}
	if marks[0].To != (Dose{Value: 250.0, Unit: MCG}) {
		t.Errorf("250 мкг, got %v", marks[0].To)
	}
}

func TestTheSeedTitratesOnTheDaysThePrototypesScheduleDoesAndNotItsChart(t *testing.T) {
	// The prototype disagrees with itself: schedule/data.ts puts the steps at days 28 and
	// 56, trends/data.ts draws the second at day 70. §03's phases give 28 and 56, so the
	// chart is wrong — porting it would draw a titration two weeks late.
	seed := seedPlan()

	var titrations []ProtocolMark
	for _, m := range ProtocolMarks(seed, seedSema, wholeCourse) {
		if m.Kind == MarkTitration {
			titrations = append(titrations, m)
		}
	}

	if len(titrations) != 2 ||
		titrations[0].Date != NewDate(2026, time.June, 7) ||
		titrations[1].Date != NewDate(2026, time.July, 5) {
		t.Fatalf("7 June and 5 July, got %v", titrations)
	}
	for i, want := range []int{28, 56} {
		if got := seed.Protocol.StartDate.DaysUntil(titrations[i].Date); got != want {
			t.Errorf("titration %d on day %d, got %d", i, want, got)
		}
	}
}

func assertMarks(t *testing.T, want, got []ProtocolMark) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("want %d marks %v, got %d %v", len(want), want, len(got), got)
	}
	for i := range want {
		if want[i].Kind != got[i].Kind || want[i].Date != got[i].Date || want[i].To != got[i].To {
			t.Errorf("mark %d: want %v got %v", i, want[i], got[i])
			continue
		}
		switch {
		case want[i].From == nil && got[i].From == nil:
		case want[i].From == nil || got[i].From == nil:
			t.Errorf("mark %d: want from %v got %v", i, want[i].From, got[i].From)
		case *want[i].From != *got[i].From:
			t.Errorf("mark %d: want from %v got %v", i, *want[i].From, *got[i].From)
		}
	}
}

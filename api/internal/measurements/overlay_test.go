package measurements

import (
	"slices"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// Ids ascend a → e, and every rule under test disagrees with that order somewhere: the
// smallest id of the whole plan belongs to a supplement, the smallest injectable one to a
// position that does not titrate, and where two titrate the earlier first phase belongs to
// the larger id. A fixture whose ids agree with the rule leaves the rule unmeasured, and
// this project has booked that three rounds running on one step.
const (
	posSupplement = protocol.ProtocolItemID("a-supplement")
	posFlat       = protocol.ProtocolItemID("b-flat-injection")
	posTitrating  = protocol.ProtocolItemID("c-titrating")
	posEarlier    = protocol.ProtocolItemID("d-titrating-earlier")
	posFlatLater  = protocol.ProtocolItemID("e-flat-injection")
)

// A dose per position, all distinct: the bands are what says which position was chosen, and
// two positions sharing a number would make the answer unreadable.
var (
	supplementLow  = protocol.Dose{Value: 0.1, Unit: protocol.MG}
	supplementHigh = protocol.Dose{Value: 0.2, Unit: protocol.MG}
	flatDose       = protocol.Dose{Value: 0.3, Unit: protocol.MG}
	flatLaterDose  = protocol.Dose{Value: 0.9, Unit: protocol.MG}
	titrating1     = protocol.Dose{Value: 0.25, Unit: protocol.MG}
	titrating2     = protocol.Dose{Value: 0.5, Unit: protocol.MG}
	titrating3     = protocol.Dose{Value: 1.0, Unit: protocol.MG}
	earlier1       = protocol.Dose{Value: 0.4, Unit: protocol.MG}
	earlier2       = protocol.Dose{Value: 0.8, Unit: protocol.MG}
)

// Twelve weeks from a Monday: week 1 opens 2 March and the last prescribed day is 24 May.
var overlayStart = civil.NewDate(2026, time.March, 2)

// The whole course and a margin either side, so the table's spans are the geometry of the
// course rather than of the window. The window's own arrival at both halves is measured
// separately, by a range that clips.
var overlayWide = civil.Range{From: civil.NewDate(2026, time.February, 1), Through: civil.NewDate(2026, time.June, 30)}

var (
	supplementPhases = []protocol.ProtocolPhase{
		{FromWeek: 1, ToWeek: 6, Dose: supplementLow},
		{FromWeek: 7, ToWeek: 12, Dose: supplementHigh},
	}
	flatPhases      = []protocol.ProtocolPhase{{FromWeek: 1, ToWeek: 12, Dose: flatDose}}
	flatLaterPhases = []protocol.ProtocolPhase{{FromWeek: 1, ToWeek: 12, Dose: flatLaterDose}}
	titratingPhases = []protocol.ProtocolPhase{
		{FromWeek: 1, ToWeek: 4, Dose: titrating1},
		{FromWeek: 5, ToWeek: 8, Dose: titrating2},
		{FromWeek: 9, ToWeek: 12, Dose: titrating3},
	}
	// The same position starting two weeks late, so that «earliest first phase» and
	// «smallest id» point at different positions.
	titratingLatePhases = []protocol.ProtocolPhase{
		{FromWeek: 3, ToWeek: 6, Dose: titrating1},
		{FromWeek: 7, ToWeek: 12, Dose: titrating2},
	}
	// Out of order on purpose, and it is the earliest week that has to decide: with the last
	// phase listed first, a rule reading `phases[0]` hands week 7 to the position that opens
	// in week 1 and picks the other one.
	earlierPhases = []protocol.ProtocolPhase{
		{FromWeek: 7, ToWeek: 12, Dose: earlier2},
		{FromWeek: 1, ToWeek: 6, Dose: earlier1},
	}
)

func position(id protocol.ProtocolItemID, kind protocol.ItemKind) protocol.ProtocolItem {
	return protocol.ProtocolItem{
		ID:         id,
		ProtocolID: protocol.ProtocolID("pr"),
		Kind:       kind,
		Cadence:    protocol.CadenceWeekly,
		DaysOfWeek: []time.Weekday{time.Monday},
		Times:      []civil.Slot{{Hour: 8}},
		Loggable:   true,
	}
}

func overlayPlan(status protocol.ProtocolStatus, items []protocol.ProtocolItem, phases map[protocol.ProtocolItemID][]protocol.ProtocolPhase) protocol.Plan {
	return protocol.Plan{
		Protocol: protocol.Protocol{
			ID:        protocol.ProtocolID("pr"),
			PatientID: civil.UserID("p"),
			StartDate: overlayStart,
			Weeks:     12,
			Status:    status,
		},
		Items:  items,
		Phases: phases,
	}
}

type band struct {
	dose    protocol.Dose
	from    civil.Date
	through civil.Date
}

type mark struct {
	kind protocol.ProtocolMarkKind
	date civil.Date
	from *protocol.Dose
	to   protocol.Dose
}

func startMark(date civil.Date, to protocol.Dose) mark {
	return mark{kind: protocol.MarkStart, date: date, to: to}
}

func titrationMark(date civil.Date, from, to protocol.Dose) mark {
	return mark{kind: protocol.MarkTitration, date: date, from: &from, to: to}
}

func day(month time.Month, d int) civil.Date { return civil.NewDate(2026, month, d) }

func assertOverlay(t *testing.T, got Overlay, wantBands []band, wantMarks []mark) {
	t.Helper()

	// Never nil, either half: KMP's MetricDetail declares both lists non-nullable, so a nil
	// slice rendering as `null` is a decode failure on the screen that asked for a course
	// with no strip — exactly the case the empty answer is for.
	if got.Bands == nil || got.Marks == nil {
		t.Fatalf("overlay carries a nil half: bands %v, marks %v", got.Bands, got.Marks)
	}

	if len(got.Bands) != len(wantBands) {
		t.Fatalf("%d bands %v, want %d %v", len(got.Bands), got.Bands, len(wantBands), wantBands)
	}
	for i, want := range wantBands {
		g := got.Bands[i]
		if g.Dose != want.dose || g.Range.From != want.from || g.Range.Through != want.through {
			t.Errorf("band %d is %v %s..%s, want %v %s..%s",
				i, g.Dose, g.Range.From, g.Range.Through, want.dose, want.from, want.through)
		}
	}

	if len(got.Marks) != len(wantMarks) {
		t.Fatalf("%d marks %v, want %d %v", len(got.Marks), got.Marks, len(wantMarks), wantMarks)
	}
	for i, want := range wantMarks {
		g := got.Marks[i]
		if g.Kind != want.kind || g.Date != want.date || g.To != want.to {
			t.Errorf("mark %d is %s %s → %v, want %s %s → %v",
				i, g.Kind, g.Date, g.To, want.kind, want.date, want.to)
		}
		switch {
		case want.from == nil && g.From != nil:
			t.Errorf("mark %d came up from %v, want no earlier dose", i, *g.From)
		case want.from != nil && g.From == nil:
			t.Errorf("mark %d came up from nothing, want %v", i, *want.from)
		case want.from != nil && g.From != nil && *g.From != *want.from:
			t.Errorf("mark %d came up from %v, want %v", i, *g.From, *want.from)
		}
	}
}

// The three rules of the overlay, each measured against a plan where the rule and the id
// order disagree, and each run against both arrival orders of the positions: `readItems`
// calls its own order «stable, arbitrary», so a choice that reads the first row of the slice
// is right half the time and green all of it.
func TestTheOverlayFollowsTheTitratingPosition(t *testing.T) {
	cases := []struct {
		name   string
		status protocol.ProtocolStatus
		items  []protocol.ProtocolItem
		phases map[protocol.ProtocolItemID][]protocol.ProtocolPhase
		bands  []band
		marks  []mark
	}{
		{
			// No injectable positions, no strip — and the supplement titrates, so a rule
			// that forgot to ask the kind would draw its bands here.
			name:   "no injectable positions",
			status: protocol.StatusActive,
			items:  []protocol.ProtocolItem{position(posSupplement, protocol.KindSupplement)},
			phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{posSupplement: supplementPhases},
		},
		{
			// Injectables, none titrating: the first by id, and its flat band is the
			// prescribed dose. Two of them, in an order the id disagrees with, so the two
			// arrival orders between them make «the last» and «the first that arrived»
			// wrong — either alone leaves one of the pair passing.
			name:   "injectables but none titrating",
			status: protocol.StatusActive,
			items: []protocol.ProtocolItem{
				position(posSupplement, protocol.KindSupplement),
				position(posFlatLater, protocol.KindInjection),
				position(posFlat, protocol.KindInjection),
			},
			phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
				posSupplement: supplementPhases,
				posFlatLater:  flatLaterPhases,
				posFlat:       flatPhases,
			},
			bands: []band{{flatDose, overlayStart, day(time.May, 24)}},
			marks: []mark{startMark(overlayStart, flatDose)},
		},
		{
			// The titrating position over the flat one that holds the smaller id. This is
			// the mutation the step names: «the first injectable by id» draws BPC's flat
			// band under a chart the patient reads as their semaglutide.
			name:   "one titrates and it is not the smallest injectable id",
			status: protocol.StatusActive,
			items: []protocol.ProtocolItem{
				position(posSupplement, protocol.KindSupplement),
				position(posFlat, protocol.KindInjection),
				position(posTitrating, protocol.KindInjection),
			},
			phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
				posSupplement: supplementPhases,
				posFlat:       flatPhases,
				posTitrating:  titratingPhases,
			},
			bands: []band{
				{titrating1, overlayStart, day(time.March, 29)},
				{titrating2, day(time.March, 30), day(time.April, 26)},
				{titrating3, day(time.April, 27), day(time.May, 24)},
			},
			marks: []mark{
				startMark(overlayStart, titrating1),
				titrationMark(day(time.March, 30), titrating1, titrating2),
				titrationMark(day(time.April, 27), titrating2, titrating3),
			},
		},
		{
			// Two titrate and the earlier first phase wins, against the id: `d` opens in
			// week 1 where `c` opens in week 3.
			name:   "two titrate and the earlier first phase decides",
			status: protocol.StatusActive,
			items: []protocol.ProtocolItem{
				position(posFlat, protocol.KindInjection),
				position(posTitrating, protocol.KindInjection),
				position(posEarlier, protocol.KindInjection),
			},
			phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
				posFlat:      flatPhases,
				posTitrating: titratingLatePhases,
				posEarlier:   earlierPhases,
			},
			bands: []band{
				{earlier1, overlayStart, day(time.April, 12)},
				{earlier2, day(time.April, 13), day(time.May, 24)},
			},
			marks: []mark{
				startMark(overlayStart, earlier1),
				titrationMark(day(time.April, 13), earlier1, earlier2),
			},
		},
		{
			// Two titrate from the same week, and only then does the id decide.
			name:   "two titrate from the same week and the id decides",
			status: protocol.StatusActive,
			items: []protocol.ProtocolItem{
				position(posTitrating, protocol.KindInjection),
				position(posEarlier, protocol.KindInjection),
			},
			phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
				posTitrating: titratingPhases,
				posEarlier:   earlierPhases,
			},
			bands: []band{
				{titrating1, overlayStart, day(time.March, 29)},
				{titrating2, day(time.March, 30), day(time.April, 26)},
				{titrating3, day(time.April, 27), day(time.May, 24)},
			},
			marks: []mark{
				startMark(overlayStart, titrating1),
				titrationMark(day(time.March, 30), titrating1, titrating2),
				titrationMark(day(time.April, 27), titrating2, titrating3),
			},
		},
		{
			// A cancelled course keeps its axis and loses its strip: the window is
			// geometry, the bands are a prescription, and a cancelled course has none.
			name:   "a cancelled course draws no strip",
			status: protocol.StatusCancelled,
			items: []protocol.ProtocolItem{
				position(posFlat, protocol.KindInjection),
				position(posTitrating, protocol.KindInjection),
			},
			phases: map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
				posFlat:      flatPhases,
				posTitrating: titratingPhases,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orders := map[string][]protocol.ProtocolItem{"as they arrived": c.items}
			reversed := slices.Clone(c.items)
			slices.Reverse(reversed)
			orders["reversed"] = reversed

			for name, items := range orders {
				t.Run(name, func(t *testing.T) {
					plan := overlayPlan(c.status, items, c.phases)
					assertOverlay(t, OverlayOn(&plan, overlayWide), c.bands, c.marks)
				})
			}
		})
	}
}

// The asked-for window reaches both halves, and neither of them is handed the course
// instead: the near edge clips the first band, the far edge clips the last, and the start
// mark falls outside the window altogether.
func TestTheWindowReachesBothHalvesOfTheOverlay(t *testing.T) {
	plan := overlayPlan(protocol.StatusActive, []protocol.ProtocolItem{
		position(posFlat, protocol.KindInjection),
		position(posTitrating, protocol.KindInjection),
	}, map[protocol.ProtocolItemID][]protocol.ProtocolPhase{
		posFlat:      flatPhases,
		posTitrating: titratingPhases,
	})

	narrow := civil.Range{From: day(time.March, 25), Through: day(time.April, 30)}

	assertOverlay(t, OverlayOn(&plan, narrow),
		[]band{
			{titrating1, day(time.March, 25), day(time.March, 29)},
			{titrating2, day(time.March, 30), day(time.April, 26)},
			{titrating3, day(time.April, 27), day(time.April, 30)},
		},
		[]mark{
			titrationMark(day(time.March, 30), titrating1, titrating2),
			titrationMark(day(time.April, 27), titrating2, titrating3),
		})
}

// No course is not an error and not a nil: the patient between courses gets the same empty
// pair of lists as the patient whose course prescribes no injection.
func TestNoCourseDrawsAnEmptyOverlay(t *testing.T) {
	assertOverlay(t, OverlayOn(nil, overlayWide), nil, nil)
}

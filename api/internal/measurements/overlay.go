package measurements

import (
	"cmp"
	"math"
	"slices"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// Overlay is the prescription drawn under one metric's axis — the dose strip and the days it
// changed. Its two lists are MetricDetail's, where neither is nullable.
type Overlay struct {
	Bands []protocol.DoseBand
	Marks []protocol.ProtocolMark
}

// OverlayOn is the strip under the axis for the patient's last course, clipped to r. Both
// halves are asked of one position, so the bands and the marks are the same prescription; a
// nil plan is the patient between courses and draws nothing.
func OverlayOn(plan *protocol.Plan, r civil.Range) Overlay {
	var bands []protocol.DoseBand
	var marks []protocol.ProtocolMark

	if plan != nil {
		if item, found := overlayPosition(*plan); found {
			bands = protocol.DoseBands(*plan, item, r)
			marks = protocol.ProtocolMarks(*plan, item, r)
		}
	}

	return Overlay{Bands: notNil(bands), Marks: notNil(marks)}
}

// overlayPosition is the position the strip belongs to: the titrating one — injectable, with
// more than one phase — and where two titrate, the one whose first phase opens earliest, then
// the smaller id. Where none titrates the first injectable by id is taken: its flat band IS
// the prescribed dose, and a one-compound course without titration is legitimate, so drawing
// it is honester than an axis with no strip at all. No injectables, no strip.
// As the whole rule and not the fallback it is a coin flip: ids are random uuids on the seed.
func overlayPosition(plan protocol.Plan) (protocol.ProtocolItemID, bool) {
	var injectable, titrating []protocol.ProtocolItem
	for _, item := range plan.Items {
		if item.Kind != protocol.KindInjection {
			continue
		}
		injectable = append(injectable, item)
		if len(plan.Phases[item.ID]) > 1 {
			titrating = append(titrating, item)
		}
	}

	if len(titrating) > 0 {
		return slices.MinFunc(titrating, func(a, b protocol.ProtocolItem) int {
			return cmp.Or(
				cmp.Compare(firstPhaseWeek(plan, a.ID), firstPhaseWeek(plan, b.ID)),
				cmp.Compare(a.ID, b.ID),
			)
		}).ID, true
	}
	if len(injectable) > 0 {
		return slices.MinFunc(injectable, func(a, b protocol.ProtocolItem) int {
			return cmp.Compare(a.ID, b.ID)
		}).ID, true
	}

	return "", false
}

// Not phases[0]: Plan.Phases is a field a caller fills by hand, which is why bands.go sorts too.
func firstPhaseWeek(plan protocol.Plan, item protocol.ProtocolItemID) int {
	week := math.MaxInt
	for _, phase := range plan.Phases[item] {
		week = min(week, phase.FromWeek)
	}

	return week
}

// A nil slice renders as `null`, and the empty overlay is the answer a screen asks for most.
func notNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}

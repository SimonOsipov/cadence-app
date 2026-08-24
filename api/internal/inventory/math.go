package inventory

import (
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// §03: «low <25%», «expiring ≤14 d», «≤4 weeks supply».
const (
	lowFraction  = 0.25
	expiringDays = 14
	reorderWeeks = 4
)

type VialID string

// Vial carries no status and no remaining count, and that is the point of it:
// §03's third correction derives both on read. Adding either back is undoing the
// migration that left them out.
type Vial struct {
	ID         VialID
	PatientID  civil.UserID
	CompoundID protocol.CompoundID
	// A label, not a number: «1 мг/мл» is what the vial says, and the clinic
	// transcribes it rather than computing with it.
	ConcentrationLabel string
	TotalDoses         int
	// Null until the vial is opened; that absence is the whole of «sealed».
	OpenedAt       *civil.Date
	ExpiresOn      civil.Date
	Lot            *string
	LocationRU     *string
	DisposedAt     *civil.Date
	LabelPhotoPath *string
}

// RemainingDoses is total minus the number of doses drawn from this vial. There is
// no counter to drift, because there is no counter.
//
// It takes the vials that doses came out of rather than the dose events
// themselves, for the reason OccurrencesFor takes a LoggedSlot: the import runs one
// way, and inventory does not learn what a dose event is.
func RemainingDoses(vial Vial, drawnFrom []VialID) int {
	remaining := vial.TotalDoses
	for _, from := range drawnFrom {
		if from == vial.ID {
			remaining--
		}
	}
	if remaining < 0 {
		return 0
	}

	return remaining
}

// VialStatus is computed on read, per §03's L10 — there is no column for it.
type VialStatus string

const (
	StatusDisposed VialStatus = "disposed"
	StatusExpiring VialStatus = "expiring"
	StatusSealed   VialStatus = "sealed"
	StatusLow      VialStatus = "low"
	StatusActive   VialStatus = "active"
)

// StatusOf reads the cases in precedence order, and the order is load-bearing:
// disposed is a fact about the vial, expiry has a deadline, low stock is the
// softest of the three.
//
// Expiring is tested before sealed deliberately. Unopened stock about to be wasted
// is exactly the vial worth warning about, and an earlier order read sealed first
// and said nothing.
func StatusOf(vial Vial, drawnFrom []VialID, today civil.Date) VialStatus {
	switch {
	case vial.DisposedAt != nil:
		return StatusDisposed
	case today.DaysUntil(vial.ExpiresOn) <= expiringDays:
		return StatusExpiring
	case vial.OpenedAt == nil:
		return StatusSealed
	case float64(RemainingDoses(vial, drawnFrom)) < float64(vial.TotalDoses)*lowFraction:
		return StatusLow
	default:
		return StatusActive
	}
}

// Cabinet is one patient's vials, and it exists so that a mixed slice cannot be
// handed to the arithmetic below.
//
// ReorderHintFor sums across everything it is given, and a doctor-side read holds
// several patients' cabinets in one result set, every row of it legitimate — so the
// database cannot catch the mixing and the type has to: B's sealed spare would
// otherwise silence A's hint.
//
// The field is unexported, so the only way to build a non-empty one is the
// constructor — a bare Cabinet{} is constructible anywhere and holds nothing, which
// yields no hint rather than a mixed answer. Naming the wrong patient does the same.
type Cabinet struct {
	vials []Vial
}

func CabinetOf(patient civil.UserID, vials []Vial) Cabinet {
	var mine []Vial
	for _, vial := range vials {
		if vial.PatientID == patient {
			mine = append(mine, vial)
		}
	}

	return Cabinet{vials: mine}
}

// ReorderHint is «buy more of this, you have about this long».
type ReorderHint struct {
	CompoundID protocol.CompoundID
	WeeksLeft  int
}

// ReorderHintFor answers only when both of §03's conditions hold — «0 sealed spares
// & ≤4 weeks supply». Either alone is a hint people learn to ignore.
//
// The compound and the weekly rate come off the same item so they cannot disagree:
// passed separately, BPC's rate against semaglutide's stock reads as «0 weeks left»,
// silently and plausibly.
//
// today is a parameter because expired stock is neither a spare nor supply — an
// earlier signature dropped it, and an expired sealed vial suppressed the hint for a
// patient two doses from nothing.
func ReorderHintFor(
	item protocol.ProtocolItem,
	cabinet Cabinet,
	drawnFrom []VialID,
	today civil.Date,
) *ReorderHint {
	if item.CompoundID == nil {
		return nil
	}
	compound := *item.CompoundID
	perWeek := protocol.DosesPerWeek(item)

	// One compound at a time: without the filter, an unopened vial of anything else
	// counts as the sealed spare that suppresses the hint.
	var live []Vial
	for _, vial := range cabinet.vials {
		if vial.DisposedAt == nil && vial.CompoundID == compound && !vial.ExpiresOn.Before(today) {
			live = append(live, vial)
		}
	}
	if perWeek <= 0 || len(live) == 0 {
		return nil
	}
	for _, vial := range live {
		if vial.OpenedAt == nil {
			return nil
		}
	}

	doses := 0
	for _, vial := range live {
		doses += RemainingDoses(vial, drawnFrom)
	}

	weeksLeft := int(float64(doses) / perWeek)
	if weeksLeft > reorderWeeks {
		return nil
	}

	return &ReorderHint{CompoundID: compound, WeeksLeft: weeksLeft}
}

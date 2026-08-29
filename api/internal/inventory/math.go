package inventory

import (
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// §03: «low <25%», «expiring ≤14 d», «≤4 weeks supply».
const (
	lowQuarter   = 4
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
	// How much substance the vial holds, and the unit the clinic wrote it in. The
	// unit is carried for rendering; the arithmetic below is in micrograms.
	TotalAmount Amount
	AmountUnit  protocol.DoseUnit
	// Null until the vial is opened; that absence is the whole of «sealed».
	OpenedAt  *civil.Date
	ExpiresOn civil.Date
	// The day the patient put it aside. Held back is not a status but a fact with a
	// date: the vial is theirs and undisposed, and it takes no part in any choice the
	// server makes for them.
	HeldBackAt     *civil.Date
	Lot            *string
	LocationRU     *string
	DisposedAt     *civil.Date
	LabelPhotoPath *string
}

// Draw is one dose taken out of a vial: which vial, and how much.
//
// The quantity travels with the identifier because the subtraction is of substance
// now. It is still not a dose event — inventory does not learn what one of those is,
// for the reason OccurrencesFor takes a LoggedSlot rather than a dose.
type Draw struct {
	VialID VialID
	Amount Amount
}

// RemainingAmount is what the vial holds minus what has been drawn out of it. There
// is no counter to drift, because there is no counter.
func RemainingAmount(vial Vial, draws []Draw) Amount {
	remaining := vial.TotalAmount
	for _, draw := range draws {
		if draw.VialID == vial.ID {
			remaining -= draw.Amount
		}
	}
	if remaining < 0 {
		return 0
	}

	return remaining
}

// RemainingDoses is how many more injections the vial holds at a given dose.
//
// Absent rather than zero when there is no dose to divide by: a course may have
// ended, and a compound the doctor typed has no reference dose to fall back on —
// silence is honester than a number nobody prescribed. This is why the dose arrives
// as a parameter and not as a lookup: the arithmetic must not learn where it came
// from, exactly as it does not learn where today came from.
func RemainingDoses(vial Vial, draws []Draw, dose *protocol.Dose) *int {
	if dose == nil {
		return nil
	}
	each, err := AmountOfDose(*dose)
	if err != nil || each <= 0 {
		return nil
	}

	left := int(RemainingAmount(vial, draws) / each)

	return &left
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
//
// «Low» takes no dose: it is a quarter of the substance, so it answers for a course
// that ended and for a drug with no reference dose — which the count model could not.
func StatusOf(vial Vial, draws []Draw, today civil.Date) VialStatus {
	switch {
	case vial.DisposedAt != nil:
		return StatusDisposed
	case today.DaysUntil(vial.ExpiresOn) <= expiringDays:
		return StatusExpiring
	case vial.OpenedAt == nil:
		return StatusSealed
	// Multiplied rather than divided: a quarter of an odd number of micrograms is
	// not one, and the boundary is «below a quarter», not «below a quarter rounded».
	case RemainingAmount(vial, draws)*lowQuarter < vial.TotalAmount:
		return StatusLow
	default:
		return StatusActive
	}
}

// Cabinet is one patient's vials, and it exists so that a mixed slice cannot be
// handed to the arithmetic below.
//
// ReorderHintFor sums across everything it is given, and a doctor-side read holds several
// patients' cabinets in one legitimate result set — so the database cannot catch the mixing
// and the type has to. The field is unexported: a bare Cabinet{} holds nothing, which yields
// no hint rather than a mixed answer.
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
// The compound and the weekly rate come off the same item so they cannot disagree: passed
// separately, BPC's rate against semaglutide's stock reads «0 weeks left», plausibly. today is
// a parameter because expired stock is neither a spare nor supply.
//
// The dose is the phase's, which is what makes this answer differently at 0,25 and at
// 1,0 мг out of the same vial — the whole of BG-001.
func ReorderHintFor(
	item protocol.ProtocolItem,
	cabinet Cabinet,
	draws []Draw,
	dose *protocol.Dose,
	today civil.Date,
) *ReorderHint {
	if item.CompoundID == nil || dose == nil {
		return nil
	}
	compound := *item.CompoundID
	perWeek := protocol.DosesPerWeek(item)

	// One compound at a time: without the filter, an unopened vial of anything else
	// counts as the sealed spare that suppresses the hint.
	//
	// Held back is in the same filter and closes both halves of the rule with it: a
	// shelved vial is neither a spare that makes reordering unnecessary nor supply whose
	// doses count toward the weeks left.
	var live []Vial
	for _, vial := range cabinet.vials {
		if vial.DisposedAt == nil && vial.HeldBackAt == nil &&
			vial.CompoundID == compound && !vial.ExpiresOn.Before(today) {
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
		left := RemainingDoses(vial, draws, dose)
		if left == nil {
			return nil
		}
		doses += *left
	}

	weeksLeft := int(float64(doses) / perWeek)
	if weeksLeft > reorderWeeks {
		return nil
	}

	return &ReorderHint{CompoundID: compound, WeeksLeft: weeksLeft}
}

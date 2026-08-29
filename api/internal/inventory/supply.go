package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// Supply answers protocol's question about one prescribed item: how many doses its open vial
// has left, and whether it is time to reorder.
//
// Both numbers are computed and neither is stored — §03's third correction makes the
// remaining count a subtraction over dose events, so there is no counter to drift.
type Supply struct{}

func NewSupply() *Supply { return &Supply{} }

// SupplyFor reads the patient's cabinet and the doses drawn from it, then asks the arithmetic
// this package already owns.
//
// Absent rather than zero when the item names no drug or the patient holds no vial of it: a
// «0 доз осталось» card over an empty cabinet says the patient ran out, and they never had one.
func (s *Supply) SupplyFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, item protocol.ProtocolItem,
	dose *protocol.Dose, today civil.Date,
) (*int, *protocol.ReorderHint, error) {
	if item.CompoundID == nil {
		return nil, nil, nil
	}

	vials, err := vialsOf(ctx, tx, patient)
	if err != nil {
		return nil, nil, err
	}
	draws, err := drawsOf(ctx, tx, patient)
	if err != nil {
		return nil, nil, err
	}

	cabinet := CabinetOf(patient, vials)

	var left *int
	// Through the cabinet, so a doctor-side read holding several patients' vials cannot be
	// mixed by the arithmetic below — see math.go's Cabinet.
	//
	// The open vial and the earliest-opened of two: a sealed spare would otherwise be
	// counted (the KMP mock's hazard, not the prototype's — that one guards on `opened`),
	// and without the ORDER BY two requests for one day could answer differently. The write
	// refuses to guess here and leaves the vial empty; a read cannot refuse.
	for _, vial := range cabinet.vials {
		// Held back with the rest: the count on «Сегодня» would otherwise be read off a
		// vial the patient has shelved, while the one they are drawing from stands
		// behind it in the same order.
		if vial.CompoundID != *item.CompoundID || vial.OpenedAt == nil ||
			vial.DisposedAt != nil || vial.HeldBackAt != nil {
			continue
		}
		left = RemainingDoses(vial, draws, dose)

		break
	}

	var hint *protocol.ReorderHint
	if mine := ReorderHintFor(item, cabinet, draws, dose, today); mine != nil {
		hint = &protocol.ReorderHint{CompoundID: mine.CompoundID, WeeksLeft: mine.WeeksLeft}
	}

	return left, hint, nil
}

// vialsOf is the patient's cabinet, whole: two readers taking different halves of one row are
// two shapes of the same vial, and the cabinet's own card is built from what the day card
// already reads.
func vialsOf(ctx context.Context, tx pgx.Tx, patient civil.UserID) ([]Vial, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, patient_id::text, compound_id::text, concentration_label,
		       total_amount, amount_unit, opened_at, expires_on, disposed_at, held_back_at,
		       lot, location_ru, label_photo_path
		FROM app.vials
		WHERE patient_id = $1
		ORDER BY opened_at, id
	`, string(patient))
	if err != nil {
		return nil, fmt.Errorf("reading the cabinet of %s: %w", patient, err)
	}
	defer rows.Close()

	var vials []Vial
	for rows.Next() {
		var (
			vial     Vial
			amount   float64
			opened   *time.Time
			expires  time.Time
			disposed *time.Time
			heldBack *time.Time
		)
		if err := rows.Scan(&vial.ID, &vial.PatientID, &vial.CompoundID,
			&vial.ConcentrationLabel, &amount, &vial.AmountUnit, &opened, &expires,
			&disposed, &heldBack, &vial.Lot, &vial.LocationRU, &vial.LabelPhotoPath); err != nil {
			return nil, err
		}
		// Converted here rather than carried as a float: the schema bounds the scale
		// per unit so nothing is lost, and above this line no sum of quantities exists.
		if vial.TotalAmount, err = AmountOf(amount, vial.AmountUnit); err != nil {
			return nil, fmt.Errorf("the amount in vial %s: %w", vial.ID, err)
		}
		vial.ExpiresOn = civil.NewDate(expires.Year(), expires.Month(), expires.Day())
		vial.OpenedAt = dayOf(opened)
		vial.DisposedAt = dayOf(disposed)
		vial.HeldBackAt = dayOf(heldBack)
		vials = append(vials, vial)
	}

	return vials, rows.Err()
}

// drawsOf is every dose the patient has drawn from any vial, with how much came out —
// the subtraction's other half. Doses with no vial named are not in it, which is what makes
// an unnamed vial cost that vial's contents one dose rather than somebody else's.
func drawsOf(ctx context.Context, tx pgx.Tx, patient civil.UserID) ([]Draw, error) {
	rows, err := tx.Query(ctx, `
		SELECT vial_id::text, dose_value, dose_unit
		FROM app.dose_events
		WHERE patient_id = $1 AND vial_id IS NOT NULL
	`, string(patient))
	if err != nil {
		return nil, fmt.Errorf("reading the doses drawn by %s: %w", patient, err)
	}
	defer rows.Close()

	var drawn []Draw
	for rows.Next() {
		var (
			draw  Draw
			value float64
			unit  protocol.DoseUnit
		)
		if err := rows.Scan(&draw.VialID, &value, &unit); err != nil {
			return nil, err
		}
		if draw.Amount, err = AmountOf(value, unit); err != nil {
			return nil, fmt.Errorf("a dose drawn by %s: %w", patient, err)
		}
		drawn = append(drawn, draw)
	}

	return drawn, rows.Err()
}

func dayOf(t *time.Time) *civil.Date {
	if t == nil {
		return nil
	}
	day := civil.NewDate(t.Year(), t.Month(), t.Day())

	return &day
}

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
	today civil.Date,
) (*int, *protocol.ReorderHint, error) {
	if item.CompoundID == nil {
		return nil, nil, nil
	}

	vials, err := vialsOf(ctx, tx, patient)
	if err != nil {
		return nil, nil, err
	}
	drawnFrom, err := drawnFromOf(ctx, tx, patient)
	if err != nil {
		return nil, nil, err
	}

	cabinet := CabinetOf(patient, vials)

	var left *int
	// Through the cabinet, like the hint below it. CabinetOf exists because a caller
	// reading several patients' vials in one result set — the doctor-side cabinet of M4,
	// or a service-seam sweep — gets rows the database will not have filtered, and this
	// loop walking the raw slice would have shown one patient the remaining count of
	// another's open vial of the same drug, with the guard sitting three lines away.
	//
	// The open vial, not the first in the list: with a sealed spare on the shelf, «doses
	// left» would count the spare's — the prototype's own bug, and the reason the KMP
	// says «the open vial».
	//
	// The earliest-opened when there are two, which the read's ORDER BY fixes: without it
	// the answer was whichever row came back first and could differ between two requests
	// for the same day. The write refuses to guess in this situation and leaves the vial
	// empty; a read cannot refuse, so it answers the same one every time instead.
	for _, vial := range cabinet.Vials() {
		if vial.CompoundID != *item.CompoundID || vial.OpenedAt == nil || vial.DisposedAt != nil {
			continue
		}
		remaining := RemainingDoses(vial, drawnFrom)
		left = &remaining

		break
	}

	var hint *protocol.ReorderHint
	if mine := ReorderHintFor(item, cabinet, drawnFrom, today); mine != nil {
		hint = &protocol.ReorderHint{CompoundID: mine.CompoundID, WeeksLeft: mine.WeeksLeft}
	}

	return left, hint, nil
}

func vialsOf(ctx context.Context, tx pgx.Tx, patient civil.UserID) ([]Vial, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, patient_id::text, compound_id::text, total_doses,
		       opened_at, expires_on, disposed_at
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
			opened   *time.Time
			expires  time.Time
			disposed *time.Time
		)
		if err := rows.Scan(&vial.ID, &vial.PatientID, &vial.CompoundID, &vial.TotalDoses,
			&opened, &expires, &disposed); err != nil {
			return nil, err
		}
		vial.ExpiresOn = civil.NewDate(expires.Year(), expires.Month(), expires.Day())
		vial.OpenedAt = dayOf(opened)
		vial.DisposedAt = dayOf(disposed)
		vials = append(vials, vial)
	}

	return vials, rows.Err()
}

// drawnFromOf is every dose the patient has drawn from any vial — the subtraction's other
// half. Doses with no vial named are not in it, which is what makes an unnamed vial cost that
// vial's count one dose rather than somebody else's.
func drawnFromOf(ctx context.Context, tx pgx.Tx, patient civil.UserID) ([]VialID, error) {
	rows, err := tx.Query(ctx, `
		SELECT vial_id::text
		FROM app.dose_events
		WHERE patient_id = $1 AND vial_id IS NOT NULL
	`, string(patient))
	if err != nil {
		return nil, fmt.Errorf("reading the doses drawn by %s: %w", patient, err)
	}
	defer rows.Close()

	var drawn []VialID
	for rows.Next() {
		var id VialID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		drawn = append(drawn, id)
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

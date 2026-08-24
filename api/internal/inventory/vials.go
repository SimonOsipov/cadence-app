package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// OpenVialFor is the vial a dose of this compound is drawn from, when there is one to name.
//
// Resolved and never chosen: with two open vials the answer is nothing, because the choice is
// the patient's and arrives in the request. Nothing to draw from is a nil, not an error. On the
// service seam the predicate below is the only lock — vials_service_read is USING (true).
func OpenVialFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, compound string,
) (*string, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text
		FROM app.vials
		WHERE patient_id = $1 AND compound_id = $2
		  AND opened_at IS NOT NULL AND disposed_at IS NULL
		LIMIT 2
	`, string(patient), compound)
	if err != nil {
		return nil, fmt.Errorf("reading the cabinet of %s: %w", patient, err)
	}
	defer rows.Close()

	var open []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		open = append(open, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// LIMIT 2 and not LIMIT 1: the question is «is there exactly one», and a query taking
	// the first would answer «yes» to a cabinet holding three.
	if len(open) != 1 {
		return nil, nil
	}

	return &open[0], nil
}

// IsDrawableFor reports whether a vial the request named may have a dose charged to it: the
// patient's own, of that compound, and not disposed of.
//
// «Not yet opened» is deliberately not a refusal, and that is the difference from OpenVialFor:
// a patient naming a sealed vial is starting it, and requiring an opened one here would make
// the first dose from every new vial impossible.
func IsDrawableFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, compound, vial string,
) (bool, error) {
	var drawable bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM app.vials
			WHERE id = $3 AND patient_id = $1 AND compound_id = $2 AND disposed_at IS NULL
		)
	`, string(patient), compound, vial).Scan(&drawable); err != nil {
		return false, fmt.Errorf("reading vial %s of %s: %w", vial, patient, err)
	}

	return drawable, nil
}

package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// OpenVialFor is the vial a dose of this compound is drawn from, when there is one to name.
//
// Resolved and never chosen: with two open vials of one compound the answer is nothing,
// because the choice is the patient's and arrives in the request. A dose with no vial named
// costs one count its dose; a dose charged to the wrong vial costs two counts their accuracy,
// and a remaining count is what the patient reorders on.
//
// Open and not disposed: a disposed vial is still on the shelf as history, and drawing from
// it would put a dose into one the patient threw away. Nothing to draw from is a truth about
// the cabinet rather than a failure, so it is a nil and not an error.
//
// Callable on either seam, and the boundary differs by which. Under WithCaller the patient's
// own policy answers and the predicate below is the second lock. On the service seam —
// vials_service_read is USING (true), and 000016 says in its own comment that this read is
// why — the predicate is the only one there is.
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
// The composite key already refuses another patient's, and that is the tenant boundary. This
// is the rest of the same question — a dose charged to a thrown-away vial, or to a vial of
// another drug, is a wrong remaining count, and that is the number the patient reorders on.
//
// «Not yet opened» is not among the refusals, and the difference from OpenVialFor is the
// point: resolution picks among vials already in use, because nobody wants a spare broached
// by a server guessing — but a patient who names a sealed vial is starting it, which is how
// a vial becomes active at all. Requiring an opened one here would make the first dose from
// every new vial impossible. Setting opened_at is the cabinet's own act and M4's business.
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

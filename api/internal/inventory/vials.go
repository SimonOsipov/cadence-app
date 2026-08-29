package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// OpenVialFor is the vial a dose of this compound is drawn from, when there is one to name,
// opening the patient's last sealed one if that is what naming it takes.
//
// Resolved and never chosen: where the answer is ambiguous it is nothing, because the choice is
// the patient's and arrives in the request. Nothing to draw from is a nil, not an error.
//
// The two layers are locked differently, which matters when this is called from a new seam.
// The read is guarded by its own predicate alone where the policy does not scope — on the
// service seam vials_service_read is USING (true). The write below is refused by the seam
// itself: the service role holds no UPDATE grant on app.vials at all, so it answers 42501
// rather than filtering to no rows.
//
// Two layers, and the order is the whole point: «exactly one undisposed vial» on its own is
// strictly worse than «exactly one open», because a patient holding an open vial and the
// sealed spare the reorder hint told them to buy has two candidates — the arithmetic would
// stop exactly when the supply arrived. So the spare only counts once nothing is open at all,
// and then it is opened by this write rather than by a screen the patient never sees.
//
// Held back takes part in neither layer: it is a vial the patient has set aside, and a choice
// the server makes for them may not reach it.
func OpenVialFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, compound string, today civil.Date,
) (*string, error) {
	open, err := openOnes(ctx, tx, patient, compound)
	if err != nil {
		return nil, err
	}
	// More than one open vial is an answer — «ambiguous, and the patient chooses» — and
	// not an absence, so only none reaches the second layer.
	if len(open) > 0 {
		return soleOf(open), nil
	}

	opened, err := openTheLastSealed(ctx, tx, patient, compound, today)
	if err != nil || opened != nil {
		return opened, err
	}

	// Asked once more, and only where the second layer opened nothing: its guard refuses
	// when another request opened the same vial and committed between the two layers, and
	// by now that vial is open. Answering nil instead would charge the dose to no vial at
	// all, and drawsOf skips events with none — so the remaining count would stay
	// overstated by this dose for the life of the vial. Where there was nothing to open in
	// the first place this repeats the answer already given, at the cost of one indexed
	// read on a path that has already made two.
	again, err := openOnes(ctx, tx, patient, compound)
	if err != nil {
		return nil, err
	}

	return soleOf(again), nil
}

// soleOf is «exactly one, or nothing»: the rule both readings of the first layer answer by,
// written once so the second cannot drift into taking the first of two.
func soleOf(open []string) *string {
	if len(open) != 1 {
		return nil
	}

	return &open[0]
}

// openOnes is the patient's open vials of this compound, two at most: the question every
// caller asks is «is there exactly one», and the third row would not change any answer.
func openOnes(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, compound string,
) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text
		FROM app.vials
		WHERE patient_id = $1 AND compound_id = $2
		  AND opened_at IS NOT NULL AND disposed_at IS NULL AND held_back_at IS NULL
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
	// LIMIT 2 and not LIMIT 1: a query taking the first would answer «yes» to a cabinet
	// holding three, and the count is what separates «the answer» from «the patient's
	// choice».
	return open, rows.Err()
}

// openTheLastSealed opens the patient's single sealed vial of this compound and names it,
// answering nothing where there is not exactly one.
//
// One statement, so the count and the write cannot disagree: a SELECT followed by an UPDATE
// would let a concurrent request open the same vial between them, and `opened_at IS NULL` in
// the UPDATE is what refuses the loser rather than opening it twice on two different days.
// The date is the patient's own day and never now(): a dose logged at half past midnight in
// Yekaterinburg opens the vial on the day the patient is living in, not the server's.
//
// Expiry is a refusal here and deliberately not in IsDrawableFor: a patient who names an
// expired vial is telling the server something it should believe, while this is the server
// choosing for them — and choosing a vial the reorder hint has already written off as neither
// spare nor supply would make the two halves of the cabinet disagree about one row.
func openTheLastSealed(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, compound string, today civil.Date,
) (*string, error) {
	var opened string
	err := tx.QueryRow(ctx, `
		WITH available AS (
		    SELECT id
		    FROM app.vials
		    WHERE patient_id = $1 AND compound_id = $2
		      AND opened_at IS NULL AND disposed_at IS NULL AND held_back_at IS NULL
		      AND expires_on >= $3::date
		    LIMIT 2
		), sole AS (
		    SELECT id FROM available WHERE (SELECT count(*) FROM available) = 1
		)
		UPDATE app.vials v
		SET opened_at = $3::date
		FROM sole
		WHERE v.id = sole.id AND v.opened_at IS NULL
		RETURNING v.id::text
	`, string(patient), compound, today.String()).Scan(&opened)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening the sealed vial of %s: %w", patient, err)
	}

	return &opened, nil
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

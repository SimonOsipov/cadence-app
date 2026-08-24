package protocol

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// ActivePlanFor reads a patient's running course with its items and their phases.
//
// Here rather than in the contexts that need it: a bounded context calls its neighbours and
// does not read their tables. Whole or not at all — an item whose phases failed to load has no
// dose, which reads as a course prescribing nothing rather than as an error. The transaction is
// the caller's, so the seam is too.
func ActivePlanFor(ctx context.Context, tx pgx.Tx, patient civil.UserID) (Plan, bool, error) {
	var plan Plan

	err := tx.QueryRow(ctx, `
		SELECT id::text, patient_id::text, start_date, duration_weeks, status,
		       created_by::text, notes
		FROM app.protocols
		WHERE patient_id = $1 AND status = 'active'
	`, string(patient)).Scan(
		&plan.Protocol.ID, &plan.Protocol.PatientID, &protocolDate{&plan.Protocol.StartDate},
		&plan.Protocol.Weeks, &plan.Protocol.Status, &plan.Protocol.CreatedBy, &plan.Protocol.Notes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, false, nil
	}
	if err != nil {
		return Plan{}, false, fmt.Errorf("reading the course of %s: %w", patient, err)
	}

	if err := readItems(ctx, tx, &plan); err != nil {
		return Plan{}, false, fmt.Errorf("reading the course of %s: %w", patient, err)
	}

	return plan, true, nil
}

// readItems fills the items and their phases.
//
// Ordered by id, which is stable and arbitrary — and it is worth saying that it is not the
// order the course was written in. protocol_items carries no position and no created_at, so
// the request order the write answers its identifiers in is not recoverable from the row.
// Nothing needs it today: the generator keys occurrences on the item, and a dose resolves
// within one item's slots. A surface that lists a course's items has to choose an order of
// its own, and if the editor ever needs the doctor's, that is a column.
func readItems(ctx context.Context, tx pgx.Tx, plan *Plan) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, kind, compound_id::text, cadence, days_of_week, times, loggable
		FROM app.protocol_items
		WHERE protocol_id = $1
		ORDER BY id
	`, string(plan.Protocol.ID))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			item ProtocolItem
			days []int16
			at   []time.Time
		)
		if err := rows.Scan(&item.ID, &item.Kind, &item.CompoundID, &item.Cadence,
			&days, &at, &item.Loggable); err != nil {
			return err
		}

		item.ProtocolID = plan.Protocol.ID
		for _, iso := range days {
			day, ok := civil.WeekdayFromISO(int(iso))
			if !ok {
				return fmt.Errorf("item %s names weekday %d", item.ID, iso)
			}
			item.DaysOfWeek = append(item.DaysOfWeek, day)
		}
		for _, slot := range at {
			item.Times = append(item.Times, civil.Slot{Hour: slot.Hour(), Minute: slot.Minute()})
		}
		// Sorted, and nothing else orders them: the column keeps whatever order the
		// doctor's form sent, and the generator walks the array as it is. A course
		// entered «20:00, 08:00» would put the morning dose against the evening
		// occurrence — «the first open slot» is a rule about the clock, not about the
		// order of entry.
		slices.SortFunc(item.Times, func(a, b civil.Slot) int {
			if a.Hour != b.Hour {
				return a.Hour - b.Hour
			}

			return a.Minute - b.Minute
		})

		plan.Items = append(plan.Items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return readPhases(ctx, tx, plan)
}

func readPhases(ctx context.Context, tx pgx.Tx, plan *Plan) error {
	rows, err := tx.Query(ctx, `
		SELECT p.protocol_item_id::text, p.from_week, p.to_week, p.dose_value, p.dose_unit
		FROM app.protocol_phases p
		JOIN app.protocol_items i ON i.id = p.protocol_item_id
		WHERE i.protocol_id = $1
		ORDER BY p.protocol_item_id, p.from_week
	`, string(plan.Protocol.ID))
	if err != nil {
		return err
	}
	defer rows.Close()

	plan.Phases = map[ProtocolItemID][]ProtocolPhase{}
	for rows.Next() {
		var (
			itemID ProtocolItemID
			phase  ProtocolPhase
		)
		if err := rows.Scan(&itemID, &phase.FromWeek, &phase.ToWeek,
			&phase.Dose.Value, &phase.Dose.Unit); err != nil {
			return err
		}
		plan.Phases[itemID] = append(plan.Phases[itemID], phase)
	}

	return rows.Err()
}

// protocolDate scans a date column into a civil.Date. pgx hands a date back as a time.Time in
// UTC, and taking its Year/Month/Day is the whole conversion — but doing it at each call site
// is where one of them starts using the caller's location instead.
type protocolDate struct{ into *civil.Date }

func (d protocolDate) ScanDate(v pgtype.Date) error {
	if !v.Valid {
		return errors.New("a date column answered NULL")
	}
	*d.into = civil.NewDate(v.Time.Year(), v.Time.Month(), v.Time.Day())

	return nil
}

// CompoundsFor reads the directory rows a plan's items name.
//
// The whole plan's at once and not one per row: the strip names every item, so a lookup per
// row is a query per row on the screen the patient opens most.
func CompoundsFor(ctx context.Context, tx pgx.Tx, plan Plan) ([]Compound, error) {
	ids := make([]string, 0, len(plan.Items))
	for _, item := range plan.Items {
		if item.CompoundID != nil {
			ids = append(ids, string(*item.CompoundID))
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Without `code`, and that is the patient's grant speaking rather than a preference:
	// step 2 kept the seed's key on the server and gave the request roles a column list
	// that omits it. Selecting it here is a 42501 for every patient opening their day.
	rows, err := tx.Query(ctx, `
		SELECT id::text, name_ru, default_unit, route, icon
		FROM app.compounds
		WHERE id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("reading the directory: %w", err)
	}
	defer rows.Close()

	var compounds []Compound
	for rows.Next() {
		var compound Compound
		if err := rows.Scan(&compound.ID, &compound.NameRU,
			&compound.DefaultUnit, &compound.Route, &compound.Icon); err != nil {
			return nil, err
		}
		compounds = append(compounds, compound)
	}

	return compounds, rows.Err()
}

package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The cabinet the stand is meant to show, in the two drugs the course prescribes.
//
// Vials alone are not enough: what is left in one is its amount minus the doses drawn from it, so
// a shelf with no dose events answers «full» to every question the screens ask. The draws are
// seeded with the vials.
const (
	// 2 мг less four draws of 0,25 is 1 мг, which at one injection a week is four weeks —
	// exactly the threshold the reorder hint fires on rather than a number near it.
	semaglutideVial  = "2.0"
	semaglutideDrawn = 4
	semaglutideDose  = "0.25"

	// The spare and the expiring one are the other drug's on purpose: one sealed vial of the
	// compound being injected is what suppresses the hint the shelf exists to show.
	bpcVial = "5000"

	// Semaglutide's, and that is the demonstration: set aside, it is neither a spare that
	// suppresses the hint nor supply that feeds it. On the other drug the rule is invisible.
	heldBackVial = "1.0"
)

// siteRotation is what the body map draws: four draws, four zones, none repeated.
var siteRotation = []string{"l-abdomen", "r-abdomen", "l-thigh", "r-thigh"}

// fillTheCabinet gives the patient the vials and the history the cabinet is computed from.
//
// Re-running the seed is ordinary, so a patient who already holds a vial is left with the
// shelf they have: a second run would double every count the screens answer.
func fillTheCabinet(
	ctx context.Context, writes *pgxpool.Pool, patient civil.UserID, today civil.Date,
) (bool, error) {
	filled := false
	err := database.WithServiceJob(ctx, writes, seedJob, func(ctx context.Context, tx pgx.Tx) error {
		var held bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM app.vials WHERE patient_id = $1)`,
			string(patient)).Scan(&held); err != nil {
			return fmt.Errorf("looking for a cabinet already filled: %w", err)
		}
		if held {
			return nil
		}

		shelf, err := theShelf(ctx, tx, patient, today)
		if err != nil {
			return err
		}
		if err := drawFrom(ctx, tx, patient, shelf, today); err != nil {
			return err
		}
		filled = true

		return nil
	})
	if err != nil {
		return false, err
	}

	return filled, nil
}

// theShelf writes the four vials and answers the one doses are drawn from.
//
// The compounds are read by name rather than created: theCourse entered them through the
// directory, and a second row for one drug would give the cabinet two compounds to divide by.
func theShelf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, today civil.Date,
) (string, error) {
	semaglutide, err := compoundNamed(ctx, tx, "Семаглутид")
	if err != nil {
		return "", err
	}
	bpc, err := compoundNamed(ctx, tx, "BPC-157")
	if err != nil {
		return "", err
	}

	opened := courseStart(today)
	var open string
	for _, vial := range []struct {
		compound   string
		label      string
		amount     string
		unit       string
		openedAt   *civil.Date
		expiresOn  civil.Date
		heldBackAt *civil.Date
		lot        string
		into       *string
	}{
		{
			compound: semaglutide, label: "2,4 мг/0,75 мл", amount: semaglutideVial, unit: "мг",
			openedAt: &opened, expiresOn: today.AddDays(300), lot: "SEM-4471", into: &open,
		},
		{
			// Set aside on the day the course reached its second band, which is the
			// kind of reason a patient shelves one: the dose changed.
			compound: semaglutide, label: "1 мг/мл", amount: heldBackVial, unit: "мг",
			expiresOn: today.AddDays(200), heldBackAt: new(opened.AddDays(28)),
			lot: "SEM-4472",
		},
		{
			compound: bpc, label: "5 мг/мл", amount: bpcVial, unit: "мкг",
			expiresOn: today.AddDays(240), lot: "BPC-0912",
		},
		{
			// Inside the fortnight the status is counted by, so the shelf shows one
			// of each state a patient can be in.
			compound: bpc, label: "5 мг/мл", amount: bpcVial, unit: "мкг",
			expiresOn: today.AddDays(9), lot: "BPC-0913",
		},
	} {
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.vials
			    (patient_id, compound_id, concentration_label, total_amount, amount_unit,
			     opened_at, expires_on, held_back_at, lot)
			VALUES ($1, $2, $3, $4::numeric, $5, $6::date, $7::date, $8::date, $9)
			RETURNING id::text
		`, string(patient), vial.compound, vial.label, vial.amount, vial.unit,
			dayText(vial.openedAt), vial.expiresOn.String(), dayText(vial.heldBackAt),
			vial.lot).Scan(&id); err != nil {
			return "", fmt.Errorf("filling the cabinet of %s: %w", patient, err)
		}
		if vial.into != nil {
			*vial.into = id
		}
	}

	return open, nil
}

// drawFrom writes the doses the remaining amount is a subtraction of.
//
// One a week on the course's own weekday, counted back from the day the seed runs, so the
// last one is this week's and the shelf is at the threshold rather than approaching it.
func drawFrom(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, vial string, today civil.Date,
) error {
	course, item, compound, err := theInjection(ctx, tx, patient)
	if err != nil {
		return err
	}

	opened := courseStart(today)
	for week := range semaglutideDrawn {
		day := opened.AddDays(week * 7)
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
			     scheduled_for_date, scheduled_for_time, injected_at,
			     dose_value, dose_unit, site_code, client_request_id)
			VALUES ($1, $2, $3, $4, $5, $6::date, TIME '08:00',
			        ($6::date + TIME '08:00') AT TIME ZONE $7,
			        $8::numeric, 'мг', $9, $10)
		`, string(patient), course, item, vial, compound, day.String(), seededZone,
			semaglutideDose, siteRotation[week%len(siteRotation)],
			fmt.Sprintf("seed-%s-%d", patient, week)); err != nil {
			return fmt.Errorf("drawing the week %d dose for %s: %w", week+1, patient, err)
		}
	}

	return nil
}

func compoundNamed(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	var id string
	if err := tx.QueryRow(ctx,
		`SELECT id::text FROM app.compounds WHERE name_ru = $1`, name).Scan(&id); err != nil {
		return "", fmt.Errorf("the directory has no %s to fill a vial with: %w", name, err)
	}

	return id, nil
}

// theInjection is the course, the position and the drug a seeded dose is attributed to.
func theInjection(ctx context.Context, tx pgx.Tx, patient civil.UserID) (string, string, string, error) {
	var course, item, compound string
	if err := tx.QueryRow(ctx, `
		SELECT p.id::text, i.id::text, i.compound_id::text
		FROM app.protocols p
		JOIN app.protocol_items i ON i.protocol_id = p.id
		WHERE p.patient_id = $1 AND p.status = 'active' AND i.kind = 'injection'
		  AND i.cadence = 'weekly'
	`, string(patient)).Scan(&course, &item, &compound); err != nil {
		return "", "", "", fmt.Errorf("the weekly injection of %s: %w", patient, err)
	}

	return course, item, compound, nil
}

func dayText(day *civil.Date) *string {
	if day == nil {
		return nil
	}
	text := day.String()

	return &text
}

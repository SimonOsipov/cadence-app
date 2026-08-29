package main

import (
	"context"
	"errors"
	"fmt"
	"time"

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

		course, err := theInjection(ctx, tx, patient)
		if err != nil {
			return err
		}
		shelf, err := theShelf(ctx, tx, patient, course.startedOn, today)
		if err != nil {
			return err
		}
		if err := drawFrom(ctx, tx, patient, shelf, course); err != nil {
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
// directory, and 000013's compounds_one_row_per_name would refuse a second row anyway.
func theShelf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, opened, today civil.Date,
) (string, error) {
	semaglutide, err := compoundNamed(ctx, tx, "Семаглутид")
	if err != nil {
		return "", err
	}
	bpc, err := compoundNamed(ctx, tx, "BPC-157")
	if err != nil {
		return "", err
	}

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
			// Shelved the day the course opened, which is when a patient has the reason:
			// the 2 мг vial was opened instead and this one went back. A date ahead of
			// today would be a card no endpoint can produce — the write path sets this
			// to the patient's own today.
			compound: semaglutide, label: "1 мг/мл", amount: heldBackVial, unit: "мг",
			expiresOn: today.AddDays(200), heldBackAt: &opened,
			lot: "SEM-4472",
		},
		{
			compound: bpc, label: "5 мг/мл", amount: bpcVial, unit: "мкг",
			expiresOn: today.AddDays(240), lot: "BPC-0912",
		},
		{
			// Inside the fortnight the status is counted by. With the open vial and the
			// sealed spare that is three of StatusOf's five states on the stand; «мало»
			// is not among them, because the open one sits at half its amount.
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
			return "", fmt.Errorf("writing a vial: %w", err)
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
//
// At the slot the item is prescribed at rather than one this command picks: a logged event meets
// its occurrence on (item, date, slot), so a draw at the wrong hour leaves the schedule showing
// four missed injections beside the four it was seeded to show.
func drawFrom(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, vial string, course weeklyInjection,
) error {
	for week := range semaglutideDrawn {
		day := course.startedOn.AddDays(week * 7)
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
			     scheduled_for_date, scheduled_for_time, injected_at,
			     dose_value, dose_unit, site_code, client_request_id)
			VALUES ($1, $2, $3, $4, $5, $6::date, $7::time,
			        ($6::date + $7::time) AT TIME ZONE $8,
			        $9::numeric, 'мг', $10, $11)
		`, string(patient), course.protocol, course.item, vial, course.compound,
			day.String(), course.slot, seededZone,
			semaglutideDose, siteRotation[week%len(siteRotation)],
			fmt.Sprintf("seed-%s-%d", patient, week)); err != nil {
			return fmt.Errorf("drawing the week %d dose: %w", week+1, err)
		}
	}

	return nil
}

func compoundNamed(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM app.compounds WHERE name_ru = $1`, name).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", fmt.Errorf("the directory has no %s to fill a vial with", name)
	case err != nil:
		return "", fmt.Errorf("looking for %s in the directory: %w", name, err)
	}

	return id, nil
}

// weeklyInjection is the prescription a seeded draw is attributed to, and the day it opened.
type weeklyInjection struct {
	protocol  string
	item      string
	compound  string
	slot      string
	startedOn civil.Date
}

// theInjection reads that prescription back.
//
// The day comes from the course rather than from courseStart: a stand whose course was prescribed
// by an earlier run and whose cabinet is filled by this one would otherwise anchor the draws to a
// different Sunday, and 0,25 мг would be written against weeks the course prescribes 0,5 for.
//
// Refuses a second weekly injection rather than taking whichever row came back first, the way
// OpenVialFor and CurrentDoseFor refuse: a course carrying two would attribute every draw to one
// of them and leave the other's occurrences open, plausibly.
func theInjection(ctx context.Context, tx pgx.Tx, patient civil.UserID) (weeklyInjection, error) {
	rows, err := tx.Query(ctx, `
		SELECT p.id::text, i.id::text, i.compound_id::text, i.times[1]::text, p.start_date
		FROM app.protocols p
		JOIN app.protocol_items i ON i.protocol_id = p.id
		WHERE p.patient_id = $1 AND p.status = 'active' AND i.kind = 'injection'
		  AND i.cadence = 'weekly'
	`, string(patient))
	if err != nil {
		return weeklyInjection{}, fmt.Errorf("the weekly injection: %w", err)
	}
	defer rows.Close()

	var found int
	var injection weeklyInjection
	for rows.Next() {
		found++
		var startedOn time.Time
		if err := rows.Scan(&injection.protocol, &injection.item, &injection.compound,
			&injection.slot, &startedOn); err != nil {
			return weeklyInjection{}, fmt.Errorf("the weekly injection: %w", err)
		}
		injection.startedOn = civil.NewDate(startedOn.Year(), startedOn.Month(), startedOn.Day())
	}
	if err := rows.Err(); err != nil {
		return weeklyInjection{}, fmt.Errorf("the weekly injection: %w", err)
	}
	if found != 1 {
		return weeklyInjection{}, fmt.Errorf(
			"the patient holds %d weekly injections, and a draw is attributed to one", found,
		)
	}

	return injection, nil
}

func dayText(day *civil.Date) *string {
	if day == nil {
		return nil
	}
	text := day.String()

	return &text
}

//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The numbers the cabinet screen draws, asked the way the screen asks them.
//
// Four vials and four dose events say nothing about what a patient sees: what is left in a vial
// is the amount minus the doses drawn from it, so the shelf is only observable through the
// arithmetic. Counted back from the day the seed runs rather than written down, so the answer is
// the same whenever it is run — the lesson MockSeed's expired course taught.
func TestTheSeededCabinetAnswersTheCabinetScreen(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	patient := thePersona(t, on)
	item := theWeeklyInjection(t, planOf(t, on, patient))
	dose := protocol.CurrentDoseFor(planOf(t, on, patient), *item.CompoundID, on.today)
	if dose == nil || dose.Value != 0.25 {
		t.Fatalf("the seeded course prescribes %v today, want 0,25 мг — week four's band", dose)
	}

	var left *int
	var hint *protocol.ReorderHint
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var err error
			left, hint, err = inventory.NewSupply().SupplyFor(ctx, tx, patient, item, dose, on.today)

			return err
		}); err != nil {
		t.Fatalf("asking the seeded cabinet: %v", err)
	}

	// Two milligrams less four quarters is one milligram, which at 0,25 is four injections —
	// one a week, so the shelf sits on the threshold rather than near it.
	if left == nil || *left != 4 {
		t.Errorf("the open vial buys %v injections, want four", left)
	}
	if hint == nil {
		t.Fatal("the seeded cabinet answers no reorder hint, which is the thing the shelf is for")
	}
	if hint.WeeksLeft != 4 {
		t.Errorf("the hint says %d weeks left, want four", hint.WeeksLeft)
	}
	if hint.CompoundID != *item.CompoundID {
		t.Errorf("the hint is about %s, want the drug the injection prescribes", hint.CompoundID)
	}
}

// The shelf as the cards read it: an open vial, a sealed spare and one inside the fortnight
// expiry is counted by — three of StatusOf's five, «мало» and «утилизирован» being absent by
// arrangement — plus the set-aside one, which is the only fixture the new column has.
//
// The spare and the expiring one are the other drug's, which is what leaves the hint above
// standing at all.
func TestTheSeededShelfHoldsOneOfEachState(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	patient := thePersona(t, on)

	var shelf struct {
		open, sealed, aside, asideOfTheInjected, asideAhead, expiring, disposed, spare int
	}
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT
				    count(*) FILTER (WHERE opened_at IS NOT NULL AND held_back_at IS NULL),
				    count(*) FILTER (WHERE opened_at IS NULL AND held_back_at IS NULL),
				    count(*) FILTER (WHERE held_back_at IS NOT NULL),
				    count(*) FILTER (
				        WHERE held_back_at IS NOT NULL
				          AND compound_id = (
				              SELECT compound_id FROM app.vials
				              WHERE patient_id = $1 AND opened_at IS NOT NULL
				          )
				    ),
				    count(*) FILTER (WHERE held_back_at > $2::date),
				    count(*) FILTER (WHERE expires_on <= $2::date + 14),
				    count(*) FILTER (WHERE disposed_at IS NOT NULL),
				    count(*) FILTER (
				        WHERE opened_at IS NULL AND held_back_at IS NULL
				          AND compound_id = (
				              SELECT compound_id FROM app.vials
				              WHERE patient_id = $1 AND opened_at IS NOT NULL
				          )
				    )
				FROM app.vials WHERE patient_id = $1
			`, string(patient), on.today.String()).Scan(
				&shelf.open, &shelf.sealed, &shelf.aside, &shelf.asideOfTheInjected,
				&shelf.asideAhead, &shelf.expiring, &shelf.disposed, &shelf.spare,
			)
		}); err != nil {
		t.Fatalf("reading the seeded shelf: %v", err)
	}

	for _, want := range []struct {
		what string
		got  int
		want int
	}{
		{"open", shelf.open, 1},
		{"sealed", shelf.sealed, 2},
		{"set aside", shelf.aside, 1},
		// On the injected drug, which is the only placing that makes the exclusion visible:
		// on the other one a shelved vial takes no part in any answer anyway.
		{"set aside on the injected drug", shelf.asideOfTheInjected, 1},
		// The write path stamps this with the patient's own today, so a day ahead of the
		// stand's is a card no endpoint could have produced.
		{"set aside on a day that has not happened", shelf.asideAhead, 0},
		{"inside the fortnight expiry is counted by", shelf.expiring, 1},
		{"disposed", shelf.disposed, 0},
		// The half of the reorder rule the shelf would otherwise hide: one sealed vial of
		// the drug being injected suppresses the hint, and the set-aside one is that drug's
		// on purpose — shelved, it counts as neither spare nor supply.
		{"a sealed spare of the injected drug", shelf.spare, 0},
	} {
		if want.got != want.want {
			t.Errorf("the seeded shelf holds %d vials %s, want %d", want.got, want.what, want.want)
		}
	}
}

// A seed is run again against a stand somebody is already using, and the cabinet is written by a
// pass with its own idempotence to keep: a doubled shelf doubles the amount and the draws both,
// and the arithmetic above would answer eight weeks off a patient who has four.
func TestRunningTheSeedTwiceFillsOneCabinet(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	for range 2 {
		if err := seed(t.Context(), theClinic(), on); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	if got := countOf(t, on.writes, `SELECT count(*) FROM app.vials`); got != 4 {
		t.Errorf("the stand holds %d vials, want four", got)
	}
	if got := countOf(t, on.writes, `SELECT count(*) FROM app.dose_events`); got != semaglutideDrawn {
		t.Errorf("the stand holds %d drawn doses, want %d", got, semaglutideDrawn)
	}
	// What the body map draws. A rotation that repeated would leave two of its four zones
	// unvisited on the stand and the screen looking like it had lost them.
	if got := countOf(t, on.writes, `SELECT count(DISTINCT site_code) FROM app.dose_events`); got != len(siteRotation) {
		t.Errorf("the drawn doses touch %d sites, want %d", got, len(siteRotation))
	}
}

// The history as the schedule reads it: between the day the course opened and today, every
// weekly injection is closed and none is left open.
//
// What is left in the vial is a subtraction of amounts and does not care when a dose came out of
// it, so nothing above would notice four draws on four consecutive days or four at an hour the
// injection is not prescribed at. An occurrence is closed by (item, date, slot) — statusOf in
// internal/protocol/occurrence.go — so either miss draws four missed injections on the stand
// beside the four it was seeded to show. Asked through the same seam the day sheet asks.
func TestTheSeededDrawsCloseThePrescribedOccurrences(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	patient := thePersona(t, on)
	plan := planOf(t, on, patient)
	item := theWeeklyInjection(t, plan)

	var taken, open int
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			history := dosing.NewHistory(func() time.Time {
				return time.Date(on.today.Year, on.today.Month, on.today.Day,
					12, 0, 0, 0, time.UTC)
			})
			// From the start date the course was read back with, not the one the seed
			// computed: an expectation derived from the value under test moves with it.
			for day := plan.Protocol.StartDate; !day.After(on.today); day = day.AddDays(1) {
				oneDay, ok := civil.NewRange(day, day)
				if !ok {
					return fmt.Errorf("%v is not a day", day)
				}
				logged, err := history.LoggedSlotsIn(ctx, tx, patient, oneDay)
				if err != nil {
					return err
				}
				for _, occurrence := range protocol.OccurrencesFor(plan, logged, day, on.today) {
					switch {
					case occurrence.ItemID != item.ID:
					case occurrence.Status == protocol.StatusDone:
						taken++
					default:
						open++
					}
				}
			}

			return nil
		}); err != nil {
		t.Fatalf("walking the course up to today: %v", err)
	}

	if taken != semaglutideDrawn {
		t.Errorf("%d weekly injections read as taken, want %d", taken, semaglutideDrawn)
	}
	if open != 0 {
		t.Errorf("%d weekly injections stand open behind today, want none", open)
	}
}

// A stand whose course an earlier run prescribed and whose cabinet this one fills. The two passes
// are independently idempotent, so they meet in this state, and the cabinet has to follow the
// course rather than the calendar: anchored to the day the seed runs, the draws would land in
// weeks the course prescribes 0,5 мг for and claim 0,25 — a history contradicting the
// prescription it is attributed to.
func TestACabinetFilledAfterItsCourseFollowsTheCourse(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("the first run: %v", err)
	}

	patient := thePersona(t, on)

	// The stand as it stands four weeks on, with the cabinet not yet filled.
	//
	// Arranged as the owner and with FORCE lifted for the two tables: no seam this command runs
	// on deletes a dose or a vial, and the owner is inside the policies like everybody else. The
	// rows affected are counted, because a filtered DELETE reports success over nothing.
	conn := testsupport.Connect(t, db.MigrationURL)
	if _, err := conn.Exec(t.Context(),
		`SELECT set_config('role', $1, false)`, "cadence_owner"); err != nil {
		t.Fatalf("assuming the owner role: %v", err)
	}
	force := func(setting string) {
		t.Helper()

		for _, table := range []string{"app.dose_events", "app.vials", "app.protocols"} {
			if _, err := conn.Exec(t.Context(),
				`ALTER TABLE `+table+` `+setting+` ROW LEVEL SECURITY`); err != nil {
				t.Fatalf("%s on %s: %v", setting, table, err)
			}
		}
	}

	force("NO FORCE")
	for _, arranged := range []struct {
		statement string
		rows      int64
	}{
		{`DELETE FROM app.dose_events WHERE patient_id = $1`, semaglutideDrawn},
		{`DELETE FROM app.vials WHERE patient_id = $1`, 4},
		{`UPDATE app.protocols SET start_date = start_date - 28 WHERE patient_id = $1`, 1},
	} {
		tag, err := conn.Exec(t.Context(), arranged.statement, string(patient))
		if err != nil {
			t.Fatalf("ageing the stand with %q: %v", arranged.statement, err)
		}
		if tag.RowsAffected() != arranged.rows {
			t.Fatalf("%q touched %d rows, want %d",
				arranged.statement, tag.RowsAffected(), arranged.rows)
		}
	}
	force("FORCE")

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("the second run: %v", err)
	}

	drawsMatchThePrescription(t, on, patient)
	theShelfFollowsTheCourse(t, on, patient)
}

// theShelfFollowsTheCourse is the other half of the same anchor: the vial dates.
//
// A shelf left on the calendar puts the open vial's `opened_at` a month after the doses drawn out
// of it, which the card draws as a vial whose history predates its opening. Each comparison stands
// beside a count that says its rows exist: a draw carrying no vial is dropped by the join and one
// drawn from an unopened vial compares against NULL, so both legs of the last pair would otherwise
// be empty and silent.
func theShelfFollowsTheCourse(t *testing.T, on deps, patient civil.UserID) {
	t.Helper()

	var shelf struct{ opened, aside, offTheCourse, drawn, beforeOpening int }
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				WITH course AS (
				    SELECT start_date FROM app.protocols
				    WHERE patient_id = $1 AND status = 'active'
				)
				SELECT
				    count(*) FILTER (WHERE v.opened_at IS NOT NULL),
				    count(*) FILTER (WHERE v.held_back_at IS NOT NULL),
				    count(*) FILTER (
				        WHERE coalesce(v.opened_at, v.held_back_at)
				              IS DISTINCT FROM (SELECT start_date FROM course)
				          AND (v.opened_at IS NOT NULL OR v.held_back_at IS NOT NULL)
				    ),
				    (SELECT count(*) FROM app.dose_events d
				     JOIN app.vials dv ON dv.id = d.vial_id
				     WHERE d.patient_id = $1 AND dv.opened_at IS NOT NULL),
				    (SELECT count(*) FROM app.dose_events d
				     JOIN app.vials dv ON dv.id = d.vial_id
				     WHERE d.patient_id = $1 AND d.scheduled_for_date < dv.opened_at)
				FROM app.vials v WHERE v.patient_id = $1
			`, string(patient)).Scan(
				&shelf.opened, &shelf.aside, &shelf.offTheCourse,
				&shelf.drawn, &shelf.beforeOpening,
			)
		}); err != nil {
		t.Fatalf("reading the seeded shelf: %v", err)
	}

	if shelf.opened != 1 || shelf.aside != 1 {
		t.Fatalf("the shelf holds %d open and %d set-aside vials, want one of each",
			shelf.opened, shelf.aside)
	}
	if shelf.drawn != semaglutideDrawn {
		t.Fatalf("%d doses are drawn from a vial on the shelf, want %d",
			shelf.drawn, semaglutideDrawn)
	}
	if shelf.offTheCourse != 0 {
		t.Errorf("%d vials are dated off the day the course opened", shelf.offTheCourse)
	}
	if shelf.beforeOpening != 0 {
		t.Errorf("%d doses are drawn from a vial before it was opened", shelf.beforeOpening)
	}
}

// drawsMatchThePrescription asks the course what it prescribed on the day of each draw.
func drawsMatchThePrescription(t *testing.T, on deps, patient civil.UserID) {
	t.Helper()

	type draw struct {
		day   civil.Date
		value float64
	}
	var draws []draw
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT scheduled_for_date, dose_value::float8 FROM app.dose_events
				WHERE patient_id = $1 ORDER BY scheduled_for_date
			`, string(patient))
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var day time.Time
				var value float64
				if err := rows.Scan(&day, &value); err != nil {
					return err
				}
				draws = append(draws, draw{
					day:   civil.NewDate(day.Year(), day.Month(), day.Day()),
					value: value,
				})
			}

			return rows.Err()
		}); err != nil {
		t.Fatalf("reading the seeded history: %v", err)
	}

	if len(draws) != semaglutideDrawn {
		t.Fatalf("the seeded history holds %d draws, want %d", len(draws), semaglutideDrawn)
	}

	plan := planOf(t, on, patient)
	item := theWeeklyInjection(t, plan)
	for _, drawn := range draws {
		prescribed := protocol.CurrentDoseFor(plan, *item.CompoundID, drawn.day)
		if prescribed == nil {
			t.Errorf("the draw on %s is attributed to a day the course prescribes nothing on",
				drawn.day)

			continue
		}
		if prescribed.Value != drawn.value {
			t.Errorf("the draw on %s took %v %s, and the course prescribes %v that day",
				drawn.day, drawn.value, prescribed.Unit, prescribed.Value)
		}
	}
}

// theWeeklyInjection is the prescribed position the cabinet is asked about, and the one the
// seeded doses are drawn against.
func theWeeklyInjection(t *testing.T, plan protocol.Plan) protocol.ProtocolItem {
	t.Helper()

	for _, item := range plan.Items {
		if item.Kind == protocol.KindInjection && item.Cadence == protocol.CadenceWeekly {
			return item
		}
	}
	t.Fatal("the seeded course prescribes no weekly injection to count a cabinet against")

	return protocol.ProtocolItem{}
}

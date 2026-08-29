//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
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

// Every state a card can be in stands on the shelf, so no field of the screen is drawn from an
// empty column — and the spare and the expiring one are the other drug's, which is what leaves
// the hint above standing at all.
func TestTheSeededShelfHoldsOneOfEachState(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	patient := thePersona(t, on)

	var shelf struct{ open, sealed, aside, asideOfTheInjected, expiring, disposed, spare int }
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
				&shelf.expiring, &shelf.disposed, &shelf.spare,
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

// The history the journal draws: one draw a week, each on a day the injection is prescribed on,
// the last of them in the week the seed is run.
//
// What is left in the vial is a subtraction of amounts and does not care which day a dose came
// out of it, so nothing above would notice four draws on four consecutive days — and the
// calendar would then show four logged dots against one prescribed occurrence.
func TestTheSeededDrawsFallOnThePrescribedDays(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	patient := thePersona(t, on)
	item := theWeeklyInjection(t, planOf(t, on, patient))

	var days []civil.Date
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT scheduled_for_date FROM app.dose_events
				WHERE patient_id = $1 ORDER BY scheduled_for_date
			`, string(patient))
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var day time.Time
				if err := rows.Scan(&day); err != nil {
					return err
				}
				days = append(days, civil.NewDate(day.Year(), day.Month(), day.Day()))
			}

			return rows.Err()
		}); err != nil {
		t.Fatalf("reading the seeded history: %v", err)
	}

	if len(days) != semaglutideDrawn {
		t.Fatalf("the seeded history holds %d draws, want %d", len(days), semaglutideDrawn)
	}

	prescribed := map[time.Weekday]bool{}
	for _, day := range item.DaysOfWeek {
		prescribed[day] = true
	}
	for week, day := range days {
		if !prescribed[day.Weekday()] {
			t.Errorf("the week %d draw is on a %s, and no injection is prescribed then",
				week+1, day.Weekday())
		}
		if week > 0 {
			if gap := days[week-1].DaysUntil(day); gap != 7 {
				t.Errorf("the week %d draw is %d days after the one before it, want seven",
					week+1, gap)
			}
		}
	}

	// The last of them is this week's, which is what puts the shelf on the reorder threshold on
	// the day somebody opens the stand rather than a fortnight after the seed was run.
	if last := days[len(days)-1].DaysUntil(on.today); last < 0 || last >= 7 {
		t.Errorf("the last draw is %d days before today, want it inside the current week", last)
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

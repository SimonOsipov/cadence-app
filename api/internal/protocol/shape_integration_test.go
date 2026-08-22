//go:build integration

package protocol_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// Its own ids, because the policy suite in this package seeds a clinic of its own and the
// two share a cluster.
const (
	shapePatient  = "7c3f3b7c-0000-4000-8000-0000000000a1"
	shapeCompound = "7c3f3b7c-0000-4000-8000-0000000000d1"
	shapeJob      = "test.protocol.shape"
)

// The pair this file exists for. Draft.Check duplicates rules the schema already holds, and
// the project's own rule is that a fact written twice is fixed once — so the two are tied:
// each refusal Go makes is offered to the database, which must refuse the same row.
//
// Without this, Go's copy drifts and the drift is invisible in both directions: a rule
// loosened here lets a row reach the schema and come back as an unreadable 23514, and a rule
// tightened here refuses a course the clinic is allowed to prescribe.
func TestWhatGoRefusesTheSchemaRefusesToo(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)
	seedForShape(t, pool)

	for _, refused := range []struct {
		name       string
		item       protocol.DraftItem
		weeks      int
		status     protocol.ProtocolStatus
		constraint string
	}{
		{
			"a daily item that also names weekdays",
			itemWith(func(i *protocol.DraftItem) { i.Cadence = protocol.CadenceDaily }),
			12, "", "protocol_items_cadence_matches_days",
		},
		{
			"a weekly item that names no weekday",
			itemWith(func(i *protocol.DraftItem) { i.DaysOfWeek = nil }),
			12, "", "protocol_items_cadence_matches_days",
		},
		{
			"an item with no slot",
			itemWith(func(i *protocol.DraftItem) { i.Times = nil }),
			12, "", "protocol_items_has_a_slot",
		},
		{
			"a phase that runs backwards",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{FromWeek: 4, ToWeek: 1, Dose: aDose}}
			}),
			12, "", "protocol_phases_runs_forwards",
		},
		{
			"a phase opening before the course",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{FromWeek: 0, ToWeek: 4, Dose: aDose}}
			}),
			12, "", "protocol_phases_from_week_check",
		},
		{
			"a dose of nothing",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 0, Unit: protocol.MG},
				}}
			}),
			12, "", "protocol_phases_dose_value_check",
		},
		{
			"a dose in a unit nobody prescribes",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 0.25, Unit: "ме"},
				}}
			}),
			12, "", "protocol_phases_dose_unit_check",
		},
		{
			"phases that overlap",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{
					{FromWeek: 1, ToWeek: 6, Dose: aDose},
					{FromWeek: 4, ToWeek: 12, Dose: aDose},
				}
			}),
			12, "", "protocol_phases_do_not_overlap",
		},
		{
			"a course of no weeks", itemWith(nil), 0, "", "protocols_duration_weeks_check",
		},
		{
			"a course in a status nobody set", itemWith(nil), 12, "paused", "protocols_status_check",
		},
		{
			"a course longer than two years", itemWith(nil), 105, "", "protocols_duration_weeks_check",
		},
		{
			"an item of an unknown kind",
			itemWith(func(i *protocol.DraftItem) { i.Kind = "infusion" }),
			12, "", "protocol_items_kind_check",
		},
		{
			"an item on an unknown cadence",
			itemWith(func(i *protocol.DraftItem) { i.Cadence = "monthly" }),
			12, "", "protocol_items_cadence_check",
		},
	} {
		t.Run(refused.name, func(t *testing.T) {
			draft := protocol.Draft{
				PatientID: shapePatient,
				StartDate: civil.NewDate(2026, time.May, 4),
				Weeks:     refused.weeks,
				Status:    refused.status,
				Items:     []protocol.DraftItem{refused.item},
			}
			if draft.Status == "" {
				draft.Status = protocol.StatusActive
			}
			if draft.Check() == nil {
				t.Fatal("Go accepted it, so the schema is not being asked the same question")
			}

			code, name := offer(t, pool, draft)
			if code != "23514" && code != "23P01" {
				t.Errorf("the schema refused with %s/%s", code, name)
			}
			if name != refused.constraint {
				t.Errorf("the schema refused by %s, want %s", name, refused.constraint)
			}
		})
	}

	// And the other direction, which is the one a suite of refusals cannot supply: what Go
	// accepts, the schema takes. A Check that refused everything would pass every case above.
	t.Run("a course with a washout between its phases", func(t *testing.T) {
		gapped := protocol.Draft{
			PatientID: shapePatient,
			StartDate: civil.NewDate(2026, time.May, 4),
			Weeks:     12,
			Status:    protocol.StatusActive,
			Items: []protocol.DraftItem{itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{
					{FromWeek: 1, ToWeek: 4, Dose: aDose},
					{FromWeek: 9, ToWeek: 12, Dose: aDose},
				}
			})},
		}
		if err := gapped.Check(); err != nil {
			t.Fatalf("Go refused it: %v", err)
		}
		if code, name := offer(t, pool, gapped); code != "" {
			t.Errorf("the schema refused a course Go accepts: %s/%s", code, name)
		}
	})
}

var aDose = protocol.Dose{Value: 0.25, Unit: protocol.MG}

func itemWith(edit func(*protocol.DraftItem)) protocol.DraftItem {
	compound := protocol.CompoundID(shapeCompound)
	item := protocol.DraftItem{
		Kind:       protocol.KindInjection,
		Compound:   protocol.CompoundRef{ID: &compound},
		Cadence:    protocol.CadenceWeekly,
		DaysOfWeek: []time.Weekday{time.Sunday},
		Times:      []civil.Slot{{Hour: 8}},
		Loggable:   true,
		Phases:     []protocol.ProtocolPhase{{FromWeek: 1, ToWeek: 12, Dose: aDose}},
	}
	if edit != nil {
		edit(&item)
	}

	return item
}

// offer writes the draft as rows and reports how the schema answered: the SQLSTATE and the
// constraint by name. Rolled back either way — this asks a question, it does not seed.
func offer(t *testing.T, pool *pgxpool.Pool, draft protocol.Draft) (string, string) {
	t.Helper()

	err := database.WithServiceJob(
		t.Context(), pool, shapeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var protocolID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
				VALUES ($1, $2::date, $3, $4) RETURNING id::text
			`, string(draft.PatientID), draft.StartDate.String(), draft.Weeks,
				string(draft.Status)).Scan(&protocolID); err != nil {
				return err
			}

			for _, item := range draft.Items {
				days := make([]int16, len(item.DaysOfWeek))
				for i, day := range item.DaysOfWeek {
					days[i] = int16(civil.ISOWeekday(day))
				}
				times := make([]string, len(item.Times))
				for i, slot := range item.Times {
					times[i] = slot.String()
				}

				var itemID string
				if err := tx.QueryRow(ctx, `
					INSERT INTO app.protocol_items
					    (protocol_id, kind, compound_id, cadence, days_of_week, times, loggable)
					VALUES ($1, $2, $3, $4, $5::smallint[], $6::time[], $7) RETURNING id::text
				`, protocolID, string(item.Kind), item.Compound.ID, string(item.Cadence),
					days, times, item.Loggable).Scan(&itemID); err != nil {
					return err
				}

				for _, phase := range item.Phases {
					if _, err := tx.Exec(ctx, `
						INSERT INTO app.protocol_phases
						    (protocol_item_id, from_week, to_week, dose_value, dose_unit)
						VALUES ($1, $2, $3, $4, $5)
					`, itemID, phase.FromWeek, phase.ToWeek,
						phase.Dose.Value, string(phase.Dose.Unit)); err != nil {
						return err
					}
				}
			}

			// Never kept: this file asks the schema a question about a row it has no
			// business leaving behind, and a seeded course would change the next case.
			return errRollback
		},
	)
	if errors.Is(err, errRollback) {
		return "", ""
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("refused by something that is not the database: %v", err)
	}

	return pgErr.Code, pgErr.ConstraintName
}

var errRollback = errors.New("asked and answered")

func seedForShape(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), pool, shapeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'patient', 'Пациент', 'Asia/Yekaterinburg')
			`, shapePatient); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.patient_profiles (user_id) VALUES ($1)
			`, shapePatient); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO app.compounds (id, name_ru, default_unit, route, icon)
				VALUES ($1, 'Семаглутид', 'мг', 'sc', 'syringe')
			`, shapeCompound)

			return err
		},
	); err != nil {
		t.Fatalf("seeding: %v", err)
	}
}

// The four closed sets are written twice — as a CHECK in 000013 and as constants in Go — and
// this reads the schema rather than a literal beside it. The first version of this test
// compared Go with a list written in the same file and called it a reconciliation: a fifth
// status added by a future migration left the whole gate green while the API silently refused
// a legitimate prescription.
func TestTheSetsTheSchemaNamesAreTheOnesGoDeclares(t *testing.T) {
	pool := resolving(t)

	for _, set := range []struct {
		constraint string
		declared   []string
	}{
		{"protocols_status_check", asStrings(protocol.Statuses())},
		{"protocol_items_cadence_check", asStrings(protocol.Cadences())},
		{"protocol_items_kind_check", asStrings(protocol.Kinds())},
		{"protocol_phases_dose_unit_check", asStrings(protocol.DoseUnits())},
	} {
		t.Run(set.constraint, func(t *testing.T) {
			var accepted []string
			if err := database.WithServiceJob(
				t.Context(), pool, shapeJob,
				func(ctx context.Context, tx pgx.Tx) error {
					return tx.QueryRow(ctx, `
						SELECT array_agg(literal[1] ORDER BY literal[1])
						FROM pg_constraint,
						     LATERAL regexp_matches(
						         pg_get_constraintdef(oid), '''([^'']*)''', 'g') AS literal
						WHERE conname = $1
					`, set.constraint).Scan(&accepted)
				},
			); err != nil {
				t.Fatalf("reading %s: %v", set.constraint, err)
			}

			want := append([]string(nil), set.declared...)
			slices.Sort(want)
			if !slices.Equal(accepted, want) {
				t.Errorf("the schema names %v, Go declares %v", accepted, want)
			}
		})
	}
}

func asStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}

	return out
}

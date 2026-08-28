//go:build integration

package protocol_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
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
		start      civil.Date
		drug       *protocol.NewCompound
		constraint string
		code       string
	}{
		{
			"a daily item that also names weekdays",
			itemWith(func(i *protocol.DraftItem) { i.Cadence = protocol.CadenceDaily }),
			12, "",
			civil.Date{},
			nil, "protocol_items_cadence_matches_days", "23514",
		},
		{
			"a weekly item that names no weekday",
			itemWith(func(i *protocol.DraftItem) { i.DaysOfWeek = nil }),
			12, "",
			civil.Date{},
			nil, "protocol_items_cadence_matches_days", "23514",
		},
		{
			"an item with no slot",
			itemWith(func(i *protocol.DraftItem) { i.Times = nil }),
			12, "",
			civil.Date{},
			nil, "protocol_items_has_a_slot", "23514",
		},
		{
			"a phase that runs backwards",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{FromWeek: 4, ToWeek: 1, Dose: aDose}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_runs_forwards", "23514",
		},
		{
			"a phase opening before the course",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{FromWeek: 0, ToWeek: 4, Dose: aDose}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_from_week_check", "23514",
		},
		{
			"a dose of nothing",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 0, Unit: protocol.MG},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_value_check", "23514",
		},
		{
			// 0,0001 мг is zero micrograms and 250,5 мкг is a tail the cabinet's
			// integer arithmetic drops: both are doses nothing can be divided by.
			"a dose finer than the microgram it is counted in",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 0.0001, Unit: protocol.MG},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_value_scale_check", "23514",
		},
		{
			"a microgram dose with a tail",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 250.5, Unit: protocol.MCG},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_value_scale_check", "23514",
		},
		{
			// The ceiling, whose absence made one row answer differently by machine:
			// int64 micrograms saturate on arm64 and wrap on amd64.
			"a dose past a gram",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 1e19, Unit: protocol.MG},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_value_magnitude_check", "23514",
		},
		{
			// The boundary, beside the absurd value: 1e19 alone leaves the constant
			// itself unmeasured on both sides of the pair.
			"a dose one milligram past a gram",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 1001, Unit: protocol.MG},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_value_magnitude_check", "23514",
		},
		{
			// The мкг arm of the ceiling on this table, which the мг cases leave
			// unmeasured: written a thousand times higher it would take a dose Go
			// refuses, and the pair this file exists for would not notice.
			"a microgram dose one microgram past a gram",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 1_000_001, Unit: protocol.MCG},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_value_magnitude_check", "23514",
		},
		{
			"a dose in a unit nobody prescribes",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{{
					FromWeek: 1, ToWeek: 4,
					Dose: protocol.Dose{Value: 0.25, Unit: "ме"},
				}}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_dose_unit_check", "23514",
		},
		{
			"phases that overlap",
			itemWith(func(i *protocol.DraftItem) {
				i.Phases = []protocol.ProtocolPhase{
					{FromWeek: 1, ToWeek: 6, Dose: aDose},
					{FromWeek: 4, ToWeek: 12, Dose: aDose},
				}
			}),
			12, "",
			civil.Date{},
			nil, "protocol_phases_do_not_overlap", "23P01",
		},
		{
			"a course of no weeks", itemWith(nil), 0, "", civil.Date{}, nil, "protocols_duration_weeks_check", "23514",
		},
		{
			"a course in a status nobody set", itemWith(nil), 12, "paused", civil.Date{}, nil, "protocols_status_check", "23514",
		},
		{
			"a course longer than two years", itemWith(nil), 105, "", civil.Date{}, nil, "protocols_duration_weeks_check", "23514",
		},
		{
			"an item of an unknown kind",
			itemWith(func(i *protocol.DraftItem) { i.Kind = "infusion" }),
			12, "",
			civil.Date{},
			nil, "protocol_items_kind_check", "23514",
		},
		{
			"an item on an unknown cadence",
			itemWith(func(i *protocol.DraftItem) { i.Cadence = "monthly" }),
			12, "",
			civil.Date{},
			nil, "protocol_items_cadence_check", "23514",
		},
		{
			// The date the type admits and the calendar does not. It answers 22008
			// and names no constraint — the schema refuses it as a value, not by a
			// rule — which is why the code is asserted separately from the name.
			"a start date the calendar does not have", itemWith(nil), 12, "",
			civil.Date{Year: 2026, Month: time.February, Day: 30},
			nil, "", "22008",
		},
		{
			// The directory's own CHECKs, reached through the same resolver the write
			// path uses.
			"a drug in a unit the directory refuses", itemWith(nil), 12, "",
			civil.Date{},
			&protocol.NewCompound{NameRU: "Ретатрутид", DefaultUnit: "ме", Route: "sc", Icon: "syringe"},
			"compounds_default_unit_check", "23514",
		},
		{
			"a drug whose name is past its bound", itemWith(nil), 12, "",
			civil.Date{},
			&protocol.NewCompound{
				NameRU: strings.Repeat("я", 201), DefaultUnit: protocol.MG, Route: "sc", Icon: "syringe",
			},
			"compounds_name_ru_check", "23514",
		},
		{
			"a drug whose route is past its bound", itemWith(nil), 12, "",
			civil.Date{},
			&protocol.NewCompound{
				NameRU: "Ретатрутид", DefaultUnit: protocol.MG, Route: strings.Repeat("s", 51), Icon: "syringe",
			},
			"compounds_route_check", "23514",
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
			if refused.start != (civil.Date{}) {
				draft.StartDate = refused.start
			}
			if refused.drug != nil {
				draft.Items[0].Compound = protocol.CompoundRef{New: refused.drug}
			}
			if draft.Check() == nil {
				t.Fatal("Go accepted it, so the schema is not being asked the same question")
			}

			code, name := offer(t, pool, draft)
			// 22008 is the date case: the schema refuses «30 February» as a value
			// rather than by a constraint, so it names none and the code is what
			// says which refusal it was.
			if code != refused.code {
				t.Errorf("the schema refused with %s/%s, want %s", code, name, refused.code)
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

	// The scale bound in the accept direction, which is where the two copies of it drift
	// apart unseen: a refusal both sides make is measured above, and a dose both sides
	// take only here. 2,01 мг is the case that caught the drift — Go refused it as too
	// fine while the schema took it, and 2,00 and 2,02 went through either way.
	for _, taken := range []protocol.Dose{
		{Value: 2.01, Unit: protocol.MG},
		{Value: 1.005, Unit: protocol.MG},
		{Value: 250, Unit: protocol.MCG},
		{Value: 1000, Unit: protocol.MG},
		// The microgram arm of the ceiling sits nowhere else in the suite: every other
		// мкг fixture is 250 or 500, so a CHECK written 1000 instead of 1000000 would
		// refuse a dose the writer takes and nothing would fail.
		{Value: 1_000_000, Unit: protocol.MCG},
	} {
		t.Run(fmt.Sprintf("a phase of %v %s", taken.Value, taken.Unit), func(t *testing.T) {
			dosed := protocol.Draft{
				PatientID: shapePatient,
				StartDate: civil.NewDate(2026, time.May, 4),
				Weeks:     12,
				Status:    protocol.StatusActive,
				Items: []protocol.DraftItem{itemWith(func(i *protocol.DraftItem) {
					i.Phases = []protocol.ProtocolPhase{{FromWeek: 1, ToWeek: 12, Dose: taken}}
				})},
			}
			if err := dosed.Check(); err != nil {
				t.Fatalf("Go refused it: %v", err)
			}
			if code, name := offer(t, pool, dosed); code != "" {
				t.Errorf("the schema refused a dose Go accepts: %s/%s", code, name)
			}
		})
	}
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
				if item.Compound.New != nil {
					if _, err := tx.Exec(ctx, `
						INSERT INTO app.compounds (name_ru, default_unit, route, icon)
						VALUES ($1, $2, $3, $4)
					`, item.Compound.New.NameRU, string(item.Compound.New.DefaultUnit),
						item.Compound.New.Route, item.Compound.New.Icon); err != nil {
						return err
					}
				}

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

// Four of the five closed sets 000013 carries are written twice — as a CHECK there and as
// constants in Go — and this reads the schema rather than a literal beside it. A status added
// by a future migration would otherwise leave the gate green while the API silently refused a
// legitimate prescription. The fifth, compounds_default_unit_check, is read back by nothing.
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

// 000023's rollback, over the object it names.
//
// The chain-level tests unwind to zero, where 000013 drops app.protocol_phases outright,
// so a down file that did nothing at all would pass both of them. This asks the question
// they cannot: with the table still standing, is the constraint gone.
func TestRollingBackTheScaleBoundLeavesTheTableWithoutIt(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	migrator := testsupport.Connect(t, db.MigrationURL)

	// The premise, so the assertion below cannot pass by the constraint never having
	// been there: the chain the fixture starts from must carry it.
	if held := scaleBounds(t, conn); held != 1 {
		t.Fatalf("the chain starts with %d scale bounds on protocol_phases, want 1", held)
	}

	applyMigration(t, migrator, "000023_a_prescribed_dose_is_no_finer_than_its_atom.down.sql")

	if held := scaleBounds(t, conn); held != 0 {
		t.Error("protocol_phases_dose_value_scale_check survived the rollback")
	}
}

// 000024's rollback, over all three tables it touched.
//
// One migration, three constraints, and the chain-level tests unwind to zero where every
// one of those tables is dropped — so a down file that dropped two of the three, or none,
// would pass them. Counted by name rather than by number: the count is what stayed the
// same when a constraint was added under a different name once already.
func TestRollingBackTheCeilingLeavesNoneOfItsThreeBounds(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	migrator := testsupport.Connect(t, db.MigrationURL)

	ceilings := map[string]string{
		"protocol_phases_dose_value_magnitude_check": "app.protocol_phases",
		"dose_events_dose_value_magnitude_check":     "app.dose_events",
		"vials_total_amount_magnitude_check":         "app.vials",
	}
	for name, table := range ceilings {
		if held := constraints(t, conn, name, table); held != 1 {
			t.Fatalf("the chain starts with %d of %s, want 1", held, name)
		}
	}

	applyMigration(t, migrator, "000024_a_dose_has_a_ceiling_as_well_as_an_atom.down.sql")

	for name, table := range ceilings {
		if held := constraints(t, conn, name, table); held != 0 {
			t.Errorf("%s survived the rollback", name)
		}
	}
}

func scaleBounds(t *testing.T, conn *pgx.Conn) int {
	t.Helper()

	var held int
	if err := conn.QueryRow(t.Context(), `
		SELECT count(*) FROM pg_constraint
		WHERE conname = 'protocol_phases_dose_value_scale_check'
		  AND conrelid = 'app.protocol_phases'::regclass
	`).Scan(&held); err != nil {
		t.Fatalf("reading the constraint: %v", err)
	}

	return held
}

func constraints(t *testing.T, conn *pgx.Conn, name, table string) int {
	t.Helper()

	var held int
	if err := conn.QueryRow(t.Context(), `
		SELECT count(*) FROM pg_constraint
		WHERE conname = $1 AND conrelid = $2::regclass
	`, name, table).Scan(&held); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return held
}

func applyMigration(t *testing.T, conn *pgx.Conn, name string) {
	t.Helper()

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if _, err := conn.Exec(t.Context(), string(statements)); err != nil {
		t.Fatalf("applying %s: %v", name, err)
	}
}

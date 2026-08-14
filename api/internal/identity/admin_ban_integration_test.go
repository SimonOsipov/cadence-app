//go:build integration

package identity_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// insufficientPrivilege is what Postgres answers when a WITH CHECK refuses the
// row being written. The same code covers a missing grant, which is why the
// subtests below also require the other values of the same closed set to go
// through: a policy that refused everything would produce this code too.
const insufficientPrivilege = "42501"

// The subjects are per-role so the subtests can write and then read each other's
// rows inside one database.
const (
	bannedPatientID = "8a1f3b7c-0000-4000-8000-0000000000a1"
	bannedDoctorID  = "8a1f3b7c-0000-4000-8000-0000000000a2"
	bannedAdminID   = "8a1f3b7c-0000-4000-8000-0000000000a3"
)

// A compromised API holds cadence_service_app and nothing else, and until now
// the service policies on profiles carried no predicate at all — so inviting an
// arbitrary address and writing it a profile with role = 'admin' was one
// transaction. Closing that in Go would be closing it inside the process that is
// assumed compromised; the refusal has to come from the database.
//
// Every value the column's CHECK admits is tried, not the one the test is named
// for: a WITH CHECK narrower than intended — role = 'patient', say — refuses
// 'admin' exactly as well, and would pass a test that only tried 'admin'.
func TestTheServicePathCannotWriteAnAdminProfile(t *testing.T) {
	db := cluster.NewDatabase(t)

	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	for role, subject := range map[string]string{
		"patient": bannedPatientID,
		"doctor":  bannedDoctorID,
	} {
		t.Run("it still writes a "+role, func(t *testing.T) {
			if err := asTheServicePath(t, pool, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO app.profiles (user_id, role, full_name, timezone)
					VALUES ($1, $2, 'Роман Ильин', 'Europe/Moscow')
				`, subject, role)

				return err
			}); err != nil {
				t.Errorf("the service path can no longer create a %s: %v", role, err)
			}
		})
	}

	t.Run("it refuses to create an admin", func(t *testing.T) {
		err := asTheServicePath(t, pool, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
			`, bannedAdminID)

			return err
		})
		requireRefusedByAPolicy(t, err)

		requireStoredRole(t, db, bannedAdminID, "")
	})

	// The other half of the door. INSERT and UPDATE are separate policies with
	// separate WITH CHECK clauses, so a ban written on one alone leaves "create a
	// patient, then promote them" as the same escalation in two statements.
	t.Run("it refuses to promote a patient to admin", func(t *testing.T) {
		err := asTheServicePath(t, pool, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(
				ctx, `UPDATE app.profiles SET role = 'admin' WHERE user_id = $1`, bannedPatientID,
			)

			return err
		})
		requireRefusedByAPolicy(t, err)

		requireStoredRole(t, db, bannedPatientID, "patient")
	})
}

// asTheServicePath is the seam every one of these writes goes through. Which
// actor it carries is beside the point here — the refusal comes from the policy
// on the row's value, and it comes for a person exactly as it does for a job.
func asTheServicePath(
	t *testing.T, pool *pgxpool.Pool, fn func(context.Context, pgx.Tx) error,
) error {
	t.Helper()

	return database.WithServiceJob(t.Context(), pool, provisioningJob, fn)
}

func requireRefusedByAPolicy(t *testing.T, err error) {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("the write was not refused by the database: %v", err)
	}
	if pgErr.Code != insufficientPrivilege {
		t.Fatalf("refused with SQLSTATE %s (%s), want %s — a policy refusal",
			pgErr.Code, pgErr.Message, insufficientPrivilege)
	}
}

// A refusal is not the same fact as an unchanged row: a filtered UPDATE returns
// success having touched nothing, and a rolled-back INSERT and a refused one
// look alike from the caller's side. The row is read under the superuser, the
// one reader here that no policy filters, so this sees what is actually stored.
// The empty string means no row at all.
func requireStoredRole(t *testing.T, db *testsupport.Database, subject, want string) {
	t.Helper()

	conn := testsupport.Connect(t, db.SuperuserURL)

	var stored string
	err := conn.QueryRow(
		t.Context(), `SELECT role FROM app.profiles WHERE user_id = $1`, subject,
	).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		stored = ""
	} else if err != nil {
		t.Fatalf("reading profile %s back: %v", subject, err)
	}

	if stored != want {
		t.Errorf("profile %s is stored as role %q, want %q", subject, stored, want)
	}
}

//go:build integration

package identity_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The regression suite the invites migration owes, and the behaviour of the two policies the one-off command stands
// on.
//
// The registries in the database package declare what the catalogue holds; measured here is what the arrangement does
// to a row on the paths that carry it in production — the request seam for the three product roles, the service seam
// for the transaction, and cadence_owner for a command that runs through no seam at all.

const (
	inviteeID       = "8a1f3b7c-0000-4000-8000-000000000021"
	otherInviteeID  = "8a1f3b7c-0000-4000-8000-000000000022"
	thirdInviteeID  = "8a1f3b7c-0000-4000-8000-000000000023"
	fourthInviteeID = "8a1f3b7c-0000-4000-8000-000000000024"

	inviteeAddress      = "anna@example.test"
	otherInviteeAddress = "boris@example.test"
	thirdInviteeAddress = "vera@example.test"
)

// The two refusals Postgres spells 42501. A missing grant and a rejected row are different claims about the schema,
// and a test that reads only the code cannot tell which one it just proved.
const (
	grantRefusal  = "the grant"
	policyRefusal = "the policy"
)

// foreignKeyViolation is what a reference refuses with when the row it points at is being deleted.
const foreignKeyViolation = "23503"

func refusedBy(t *testing.T, err error) string {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("refused with %v, want an error from Postgres", err)
	}
	if pgErr.Code != insufficientPrivilege {
		t.Fatalf("refused with SQLSTATE %s (%s), want %s",
			pgErr.Code, pgErr.Message, insufficientPrivilege)
	}

	switch {
	case strings.Contains(pgErr.Message, "permission denied"):
		return grantRefusal
	case strings.Contains(pgErr.Message, "row-level security policy"):
		return policyRefusal
	}

	t.Fatalf("refused with 42501 and a message neither refusal spells: %s", pgErr.Message)

	return ""
}

func recordInvitation(t *testing.T, c clinic, userID, address, invitedBy string) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), c.service, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')
			`, userID, address, invitedBy)

			return err
		},
	); err != nil {
		t.Fatalf("recording the invitation of %s: %v", address, err)
	}
}

// §04 gives the doctor create and nothing else: the absent read is why a doctor cannot enumerate which addresses the
// clinic knows.
func TestADoctorRecordsAnInvitationAndCannotReadItBack(t *testing.T) {
	c := newClinic(t)

	if affected := c.changed(
		t, doctorID, "doctor",
		`INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')`,
		inviteeID, inviteeAddress, doctorID,
	); affected != 1 {
		t.Errorf("the doctor recorded %d invitation(s), want 1", affected)
	}

	err := c.as(t, doctorID, "doctor", func(ctx context.Context, tx pgx.Tx) error {
		var rows int

		return tx.QueryRow(ctx, `SELECT count(*) FROM app.invites`).Scan(&rows)
	})
	if err == nil {
		t.Fatal("a doctor read the invitations back")
	}
	if by := refusedBy(t, err); by != grantRefusal {
		t.Errorf("the read was refused by %s, want %s", by, grantRefusal)
	}
}

// The predicate rather than the grant: without it a doctor signs an invitation in a colleague's name.
func TestADoctorCannotSignAnInvitationWithAnotherDoctorsName(t *testing.T) {
	c := newClinic(t)

	err := c.as(t, doctorID, "doctor", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')
		`, inviteeID, inviteeAddress, otherDoctorID)

		return err
	})
	if err == nil {
		t.Fatal("a doctor recorded an invitation in another doctor's name")
	}
	if by := refusedBy(t, err); by != policyRefusal {
		t.Errorf("the write was refused by %s, want %s", by, policyRefusal)
	}
}

// The admin's side: every invitation and no write at all. Two rows from two authors, because "reads everything" is
// also true of a table holding the one row the reader wrote.
func TestTheAdminReadsEveryInvitationAndRecordsNone(t *testing.T) {
	c := newClinic(t)

	recordInvitation(t, c, inviteeID, inviteeAddress, doctorID)
	recordInvitation(t, c, otherInviteeID, otherInviteeAddress, otherDoctorID)

	seen := c.visible(
		t, adminID, "admin", `SELECT user_id::text FROM app.invites ORDER BY user_id`,
	)
	if want := []string{inviteeID, otherInviteeID}; !slices.Equal(seen, want) {
		t.Errorf("the admin sees %v, want exactly %v", seen, want)
	}

	err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')
		`, thirdInviteeID, thirdInviteeAddress, adminID)

		return err
	})
	if err == nil {
		t.Fatal("the admin recorded an invitation")
	}
	if by := refusedBy(t, err); by != grantRefusal {
		t.Errorf("the write was refused by %s, want %s", by, grantRefusal)
	}
}

// Empty in both directions, stated rather than inferred: a patient holds no verb over the clinic's addresses.
func TestAPatientReachesNothingOnTheInvitations(t *testing.T) {
	c := newClinic(t)

	recordInvitation(t, c, inviteeID, inviteeAddress, doctorID)

	// The address is the value a patient must not reach, so the second probe names it rather than counting.
	for name, probe := range map[string]struct {
		statement string
		args      []any
	}{
		"counting the whole table": {`SELECT count(*) FROM app.invites`, nil},
		"looking for one address": {
			`SELECT count(*) FROM app.invites WHERE email = $1`, []any{inviteeAddress},
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
				var rows int

				return tx.QueryRow(ctx, probe.statement, probe.args...).Scan(&rows)
			})
			if err == nil {
				t.Fatal("a patient read the invitations")
			}
			if by := refusedBy(t, err); by != grantRefusal {
				t.Errorf("refused by %s, want %s", by, grantRefusal)
			}
		})
	}

	err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')
		`, otherInviteeID, otherInviteeAddress, doctorID)

		return err
	})
	if err == nil {
		t.Fatal("a patient recorded an invitation")
	}
	if by := refusedBy(t, err); by != grantRefusal {
		t.Errorf("the write was refused by %s, want %s", by, grantRefusal)
	}
}

// The two verbs the service path is trusted with, and the two it is not: a record it could rewrite witnesses nothing.
func TestTheServicePathRecordsAndReadsInvitationsAndChangesNone(t *testing.T) {
	c := newClinic(t)

	recordInvitation(t, c, inviteeID, inviteeAddress, doctorID)

	// The account is the table's only key — addresses are deliberately not unique — and without it a retried
	// invitation writes a second row, leaving the invite-versus-claim lookup answering with a set. Measured:
	// PRIMARY KEY degraded to NOT NULL passed every other test.
	repeated := database.WithServiceJob(
		t.Context(), c.service, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')
			`, inviteeID, otherInviteeAddress, doctorID)
			return err
		},
	)

	var duplicate *pgconn.PgError
	if !errors.As(repeated, &duplicate) || duplicate.Code != "23505" {
		t.Fatalf("a second invitation of the same account was accepted: %v", repeated)
	}

	var seen int
	if err := database.WithServiceJob(
		t.Context(), c.service, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT count(*) FROM app.invites WHERE user_id = $1
			`, inviteeID).Scan(&seen)
		},
	); err != nil {
		t.Fatalf("reading the invitation back on the service path: %v", err)
	}
	if seen != 1 {
		t.Errorf("the service path sees %d invitation(s), want 1", seen)
	}

	for name, probe := range map[string]struct {
		statement string
		args      []any
	}{
		"rewriting": {`UPDATE app.invites SET email = $1`, []any{thirdInviteeAddress}},
		"deleting":  {`DELETE FROM app.invites`, nil},
	} {
		t.Run(name, func(t *testing.T) {
			err := database.WithServiceJob(
				t.Context(), c.service, provisioningJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, probe.statement, probe.args...)

					return err
				},
			)
			if err == nil {
				t.Fatal("accepted")
			}
			if by := refusedBy(t, err); by != grantRefusal {
				t.Errorf("refused by %s, want %s", by, grantRefusal)
			}
		})
	}
}

// The constraint at invites.email from both sides: what it refuses, and that it takes what NormalizeAddress produces.
func TestAnInvitationCarriesTheAddressTheFoldProduces(t *testing.T) {
	c := newClinic(t)

	const raw = "  Anna@Example.TEST  "

	// An identifier per case: sharing one turns the second refusal into a duplicate key from a different constraint.
	for name, unfolded := range map[string]struct {
		userID, address string
	}{
		"a spelling the provider would not store": {inviteeID, strings.TrimSpace(raw)},
		"an address nobody trimmed":               {otherInviteeID, raw},
		// The empty string is its own lower and its own trim, so the constraint's second half is all that refuses it.
		"no address at all": {thirdInviteeID, ""},
		// The one shape btrim alone refuses: the cases above carry capitals, which lower() answers, and dropping
		// btrim from the constraint left the suite green.
		"an address folded in case but not in space": {fourthInviteeID, " anna@example.test "},
	} {
		t.Run(name, func(t *testing.T) {
			err := database.WithServiceJob(
				t.Context(), c.service, provisioningJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, `
						INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'patient')
					`, unfolded.userID, unfolded.address, doctorID)

					return err
				},
			)
			if err == nil {
				t.Fatal("accepted")
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("refused with %v, want SQLSTATE 23514 (check_violation)", err)
			}
		})
	}

	// The control: without it the refusals above hold just as well against a table that stores no address at all.
	recordInvitation(t, c, inviteeID, identity.NormalizeAddress(raw), doctorID)
}

// What the absent ON DELETE on invited_by costs the admin, the one role holding DELETE on profiles. The control
// matters: without a doctor nobody invited through, the refusal below is equally consistent with an admin who cannot
// delete a profile at all.
func TestAnAdminCannotRemoveADoctorWhoseInvitationsStand(t *testing.T) {
	c := newClinic(t)

	recordInvitation(t, c, inviteeID, inviteeAddress, doctorID)

	if affected := c.changed(
		t, adminID, "admin", `DELETE FROM app.profiles WHERE user_id = $1`, otherDoctorID,
	); affected != 1 {
		t.Fatalf("removing a doctor no invitation names touched %d row(s), want 1", affected)
	}

	err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM app.profiles WHERE user_id = $1`, doctorID)

		return err
	})
	if err == nil {
		t.Fatal("a doctor named by an invitation was removed")
	}

	// The code and not merely a refusal: any other error here would be a policy or a grant saying something else.
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != foreignKeyViolation {
		t.Fatalf("the removal was refused with %v, want SQLSTATE %s (foreign_key_violation)",
			err, foreignKeyViolation)
	}
}

// The role cmd/bootstrap-admin runs under, reached as the command reaches it: the migration connection, not a seam.
func asTheOwner(t *testing.T) (*pgx.Conn, *testsupport.Database) {
	t.Helper()

	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.MigrationURL)
	if _, err := conn.Exec(t.Context(), `SET ROLE `+testsupport.OwnerRole); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}

	return conn, db
}

// Both halves of 000009's profiles policy: the row it permits, and the column's other two values it must refuse.
func TestTheOwnerCreatesAnAdminAndNoOtherRole(t *testing.T) {
	conn, _ := asTheOwner(t)
	ctx := t.Context()

	tag, err := conn.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
	`, adminID)
	if err != nil {
		t.Fatalf("creating the first admin: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("creating the first admin wrote %d row(s), want 1", tag.RowsAffected())
	}

	// Read back through profiles_hook_read, the owner's only sight of this table.
	var role string
	if err := conn.QueryRow(
		ctx, `SELECT role FROM app.profiles WHERE user_id = $1`, adminID,
	).Scan(&role); err != nil {
		t.Fatalf("reading the admin back: %v", err)
	}
	if role != "admin" {
		t.Errorf("the row the owner wrote is a %s", role)
	}

	for id, refused := range map[string]string{
		otherDoctorID:  "doctor",
		otherPatientID: "patient",
	} {
		t.Run(refused, func(t *testing.T) {
			_, err := conn.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, $2, 'Марина Крылова', 'Europe/Moscow')
			`, id, refused)
			if err == nil {
				t.Fatal("accepted")
			}
			if by := refusedBy(t, err); by != policyRefusal {
				t.Errorf("refused by %s, want %s", by, policyRefusal)
			}
		})
	}
}

// The two verbs the owner did not gain answer without an error: with no policy to name them, UPDATE and DELETE see no
// row to touch. A test asserting err == nil here would be green on a schema where the owner rewrites every profile.
func TestTheOwnerStillCannotRewriteOrRemoveAProfile(t *testing.T) {
	conn, db := asTheOwner(t)
	ctx := t.Context()

	if _, err := conn.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
	`, adminID); err != nil {
		t.Fatalf("creating the first admin: %v", err)
	}

	for name, statement := range map[string]string{
		"rewriting": `UPDATE app.profiles SET full_name = 'Кто-то Другой'`,
		"removing":  `DELETE FROM app.profiles`,
	} {
		t.Run(name, func(t *testing.T) {
			tag, err := conn.Exec(ctx, statement)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if tag.RowsAffected() != 0 {
				t.Errorf("%s touched %d row(s), want 0", name, tag.RowsAffected())
			}
		})
	}

	// The witness the owner cannot be: it sees this row only through the hook policy, so the superuser is asked.
	superuser := testsupport.Connect(t, db.SuperuserURL)

	var name string
	if err := superuser.QueryRow(
		ctx, `SELECT full_name FROM app.profiles WHERE user_id = $1`, adminID,
	).Scan(&name); err != nil {
		t.Fatalf("reading the profile as the superuser: %v", err)
	}
	if name != "Пётр Аверин" {
		t.Errorf("the profile is now named %q", name)
	}
}

// The signature the owner may not forge: a role holding TRUNCATE must at least not write a row in a person's name.
func TestTheOwnerSignsItsAuditRowWithAJobAndNeverAsAPerson(t *testing.T) {
	conn, db := asTheOwner(t)
	ctx := t.Context()

	if _, err := conn.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
	`, adminID); err != nil {
		t.Fatalf("creating the first admin: %v", err)
	}

	tag, err := conn.Exec(ctx, `
		INSERT INTO app.audit_log (actor_job, action, entity, entity_id)
		VALUES ('bootstrap-admin', 'admin.bootstrap', 'profiles', $1)
	`, adminID)
	if err != nil {
		t.Fatalf("signing the audit row: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("the audit row wrote %d row(s), want 1", tag.RowsAffected())
	}

	_, err = conn.Exec(ctx, `
		INSERT INTO app.audit_log (actor_id, action, entity, entity_id)
		VALUES ($1, 'admin.bootstrap', 'profiles', $1)
	`, adminID)
	if err == nil {
		t.Fatal("the owner signed an audit row in a person's name")
	}
	if by := refusedBy(t, err); by != policyRefusal {
		t.Errorf("the forged signature was refused by %s, want %s", by, policyRefusal)
	}

	// No SELECT policy for the owner, and no policy means an empty answer rather than a refusal. The superuser
	// supplies the witness the writer cannot.
	var visibleToTheOwner int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM app.audit_log`).Scan(&visibleToTheOwner); err != nil {
		t.Fatalf("counting the audit rows as the owner: %v", err)
	}
	if visibleToTheOwner != 0 {
		t.Errorf("the owner reads %d audit row(s), want none", visibleToTheOwner)
	}

	superuser := testsupport.Connect(t, db.SuperuserURL)

	var job string
	if err := superuser.QueryRow(
		ctx, `SELECT actor_job FROM app.audit_log WHERE entity_id = $1`, adminID,
	).Scan(&job); err != nil {
		t.Fatalf("reading the audit row as the superuser: %v", err)
	}
	if job != "bootstrap-admin" {
		t.Errorf("the audit row is signed %q", job)
	}
}

// The column the whole role arm rests on, checked at the schema rather than at the caller. Both halves fail closed:
// a statement that forgets the role is refused instead of recording a guess, and a role outside the closed set is
// refused instead of becoming one ClaimFor can never match.
//
// The first is the load-bearing one. With a DEFAULT — the obvious way to make this column easy to add — the omitting
// statement below would succeed and write 'patient', reopening the window this column closes, in the column whose
// only job is to keep it shut.
func TestAnInvitationCannotBeRecordedWithoutSayingWhatItIsFor(t *testing.T) {
	c := newClinic(t)

	// The SQLSTATE and not merely a refusal: «no role at all» exists to fail against a DEFAULT, and a bare
	// err != nil is equally green when the row is refused for something else entirely — a predicate added to
	// invites_doctor_insert, a renamed column, a foreign key. Then the subcase passes while measuring nothing.
	for _, tc := range []struct {
		name      string
		statement string
		args      []any
		want      string
	}{
		{
			name:      "no role at all",
			statement: `INSERT INTO app.invites (user_id, email, invited_by) VALUES ($1, $2, $3)`,
			args:      []any{inviteeID, inviteeAddress, doctorID},
			want:      "23502",
		},
		{
			name:      "an explicit NULL",
			statement: `INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, NULL)`,
			args:      []any{inviteeID, inviteeAddress, doctorID},
			want:      "23502",
		},
		{
			// Not a role profiles.role accepts, so an invitation for it could never be completed.
			name:      "a role no profile can carry",
			statement: `INSERT INTO app.invites (user_id, email, invited_by, role) VALUES ($1, $2, $3, 'nurse')`,
			args:      []any{inviteeID, inviteeAddress, doctorID},
			want:      "23514",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := c.as(t, doctorID, "doctor", func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, tc.statement, tc.args...)

				return err
			})
			if err == nil {
				t.Fatal("the row was recorded, so the invitation says nothing about what it was for")
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.want {
				t.Fatalf("refused with %v, want SQLSTATE %s", err, tc.want)
			}
		})
	}
}

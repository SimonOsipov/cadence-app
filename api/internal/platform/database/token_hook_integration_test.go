//go:build integration

package database_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// asHook runs the hook the way the identity provider will: as a member of
// cadence_auth_hook, which is the only role the grant names.
func asHook(t *testing.T, db *testsupport.Database) *pgx.Conn {
	t.Helper()

	conn := testsupport.Connect(t, db.MigrationURL)

	// From PostgreSQL 16 a CREATEROLE role is granted the roles it creates with
	// SET FALSE, so the role that ran the migration cannot assume the role that
	// migration made without saying so explicitly.
	if _, err := conn.Exec(t.Context(), `GRANT `+testsupport.AuthHookRole+` TO CURRENT_USER`); err != nil {
		t.Fatalf("taking the hook role: %v", err)
	}

	if _, err := conn.Exec(t.Context(), `SET ROLE `+testsupport.AuthHookRole); err != nil {
		t.Fatalf("becoming the hook role: %v", err)
	}

	return conn
}

// call invokes the hook with an event and returns the claims it hands back.
func call(t *testing.T, conn *pgx.Conn, event string) string {
	t.Helper()

	var out string
	if err := conn.QueryRow(
		t.Context(), `SELECT app.access_token_hook($1::jsonb)::text`, event,
	).Scan(&out); err != nil {
		t.Fatalf("calling the hook with %s: %v", event, err)
	}

	return out
}

// The hook writes cadence_role deterministically, both ways: a user with a
// profile gets their role, a user without one gets the key removed. The second
// half is not an edge case — an invited account that has not been provisioned is
// a real state, and the seam refuses it with a reason of its own.
func TestTheHookWritesTheRoleDeterministically(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	// A profile written the only way there is: through the service path.
	service := testsupport.Connect(t, db.ServiceAppURL)
	if _, err := service.Exec(ctx, `
		SELECT set_config('app.actor_job', 'test.seed', false);
		SET ROLE cadence_service;
	`); err != nil {
		t.Fatalf("assuming the service role: %v", err)
	}
	if _, err := service.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ('8a1f3b7c-0000-4000-8000-000000000001', 'doctor', 'Марина Крылова', 'Europe/Moscow')
	`); err != nil {
		t.Fatalf("seeding a profile: %v", err)
	}

	hook := asHook(t, db)

	withProfile := call(t, hook, `{
		"user_id": "8a1f3b7c-0000-4000-8000-000000000001",
		"claims": {"sub": "8a1f3b7c-0000-4000-8000-000000000001", "aud": "authenticated",
		           "exp": 1, "iss": "http://localhost:9999", "role": "authenticated"}
	}`)

	for _, mandatory := range []string{`"sub"`, `"aud"`, `"exp"`, `"iss"`, `"role"`} {
		if !contains(withProfile, mandatory) {
			t.Errorf("the hook dropped %s; GoTrue answers 500 rather than issuing a token",
				mandatory)
		}
	}
	if !contains(withProfile, `"cadence_role": "doctor"`) {
		t.Errorf("claims came back as %s, want the product role in them", withProfile)
	}

	// No profile: the key is removed rather than left at some default.
	withoutProfile := call(t, hook, `{
		"user_id": "8a1f3b7c-0000-4000-8000-0000000000ff",
		"claims": {"sub": "8a1f3b7c-0000-4000-8000-0000000000ff", "role": "authenticated"}
	}`)
	if contains(withoutProfile, "cadence_role") {
		t.Errorf("claims came back as %s, want no product role at all", withoutProfile)
	}

	// An event arriving with the role already substituted does not come back
	// with it: the input is formed by GoTrue from user data, so it is not
	// trusted.
	substituted := call(t, hook, `{
		"user_id": "8a1f3b7c-0000-4000-8000-0000000000ff",
		"claims": {"sub": "8a1f3b7c-0000-4000-8000-0000000000ff", "cadence_role": "admin"}
	}`)
	if contains(substituted, "admin") {
		t.Errorf("claims came back as %s, want the substituted role removed", substituted)
	}
}

// Token issuance must not fail. An event the hook does not understand comes back
// unchanged: a hook that raised would take sign-in down for everybody, including
// whoever would have to fix it.
func TestTheHookNeverRefusesToIssueAToken(t *testing.T) {
	db := cluster.NewDatabase(t)
	hook := asHook(t, db)

	for name, event := range map[string]string{
		"no claims at all": `{"user_id": "8a1f3b7c-0000-4000-8000-000000000001"}`,
		"claims that are not an object": `{"user_id": "8a1f3b7c-0000-4000-8000-000000000001",
			"claims": "nonsense"}`,
		"no user at all":            `{"claims": {"sub": "x"}}`,
		"a user that is not a UUID": `{"user_id": "not-a-uuid", "claims": {"sub": "x"}}`,
		"an empty event":            `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			// The assertion is that it answers at all. call fails the test on an
			// error, which is the whole property.
			_ = call(t, hook, event)
		})
	}
}

// The identity provider's role reaches the hook and nothing else. Its own
// database user is made a member of cadence_auth_hook, so what that membership
// carries is the whole of what GoTrue can do to the application schema.
func TestTheHookRoleReachesNothingButTheHook(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	hook := asHook(t, db)

	for name, statement := range map[string]string{
		"reading profiles":             `SELECT count(*) FROM app.profiles`,
		"reading the subject function": `SELECT app.jwt_subject()`,
		"reading the audit log":        `SELECT count(*) FROM app.audit_log`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := hook.Exec(ctx, statement)
			if err == nil {
				t.Fatal("the hook role reached it")
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("refused with %v, want SQLSTATE 42501", err)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}

		return false
	}()
}

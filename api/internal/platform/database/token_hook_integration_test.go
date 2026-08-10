//go:build integration

package database_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// mandatoryClaims is what GoTrue v2.194.0 requires of the hook's output. Its
// MinimumViableTokenSchema lists exactly these, and an output missing any one of
// them is answered with 500 rather than a token.
//
// It is documentation and a failure message, not the assertion: the assertion is
// that the claims come back exactly as they went in, bar the one key this hook
// owns. A list of ten names cannot see an eleventh going missing, and `iss` — no
// longer required by GoTrue and so absent from this list — is exactly such an
// eleventh: dropping it does not stop issuance, it produces a signed token with
// no issuer, which this API's verifier refuses on every subsequent request. A
// different failure in a different place, from a mutation a named-key loop would
// never notice.
var mandatoryClaims = []string{
	"aud", "exp", "iat", "sub", "email", "phone", "role", "aal",
	"session_id", "is_anonymous",
}

// provisioned maps a subject to the product role its profile carries — one per
// value the schema's CHECK admits. unprovisioned is an invited account that has
// not been given a profile, which is a real state rather than an edge case.
var provisioned = map[string]string{
	"8a1f3b7c-0000-4000-8000-000000000001": "patient",
	"8a1f3b7c-0000-4000-8000-000000000002": "doctor",
	"8a1f3b7c-0000-4000-8000-000000000003": "admin",
}

const (
	aDoctor       = "8a1f3b7c-0000-4000-8000-000000000002"
	unprovisioned = "8a1f3b7c-0000-4000-8000-0000000000ff"
)

// event carries the four top-level keys GoTrue actually sends and all ten claims
// its schema requires, plus `iss` — measured off supabase/gotrue:v2.194.0 on
// 2026-08-10. It is not the whole of a real event: GoTrue also emits
// `app_metadata`, `user_metadata`, usually `amr`, and a `time` in the metadata.
// They are left out because nothing here turns on them, and `iss` is in because
// it is the member most likely to be dropped by a plausible rewrite of the hook.
//
// The ten are here rather than a handful because a test asserting "the mandatory
// claims survive" against a fixture holding five of them is asserting it about
// five.
func event(subject string, extra map[string]any) map[string]any {
	claims := map[string]any{
		"aud":          "authenticated",
		"exp":          1786401144,
		"iat":          1786397544,
		"sub":          subject,
		"email":        "marina@example.com",
		"phone":        "",
		"role":         "authenticated",
		"aal":          "aal1",
		"session_id":   "0746faab-0495-4cc9-8e53-c9cc2fd5249b",
		"is_anonymous": false,
		"iss":          "http://localhost:9999",
	}
	maps.Copy(claims, extra)

	return map[string]any{
		"user_id":               subject,
		"authentication_method": "password",
		"claims":                claims,
		"metadata": map[string]any{
			"name": "customize-access-token",
			"uuid": "62fa5ebb-5dd3-4bd2-9b3f-0d498bebd5da",
		},
	}
}

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

// call invokes the hook and returns the event it hands back, decoded.
//
// Decoded rather than compared as text: an assertion by substring cannot tell
// where in the document a key landed, and `jsonb_set(event, …)` in place of
// `jsonb_set(claims, …)` writes the product role next to the claims instead of
// inside them. GoTrue signs the claims, so that mutation issues tokens carrying
// no role at all — and every substring assertion stays green through it.
func call(t *testing.T, conn *pgx.Conn, in map[string]any) map[string]any {
	t.Helper()

	encoded, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("encoding the event: %v", err)
	}

	var raw string
	if err := conn.QueryRow(
		t.Context(), `SELECT app.access_token_hook($1::jsonb)::text`, string(encoded),
	).Scan(&raw); err != nil {
		t.Fatalf("calling the hook with %s: %v", encoded, err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decoding what the hook returned (%s): %v", raw, err)
	}

	return out
}

// claimsIn is the claims of an event on its way in. The assertion is checked
// rather than bare so that a fixture built wrong fails as a fixture, at the line
// that built it, instead of as a panic inside an assertion.
func claimsIn(t *testing.T, in map[string]any) map[string]any {
	t.Helper()

	claims, ok := in["claims"].(map[string]any)
	if !ok {
		t.Fatalf("the fixture %v carries no claims object", in)
	}

	return claims
}

// claimsOf is the half of the event GoTrue signs. Everything this hook exists to
// do has to land in here and nowhere else.
func claimsOf(t *testing.T, out map[string]any) map[string]any {
	t.Helper()

	claims, ok := out["claims"].(map[string]any)
	if !ok {
		t.Fatalf("the hook returned %v, which carries no claims object", out)
	}

	if _, stray := out["cadence_role"]; stray {
		t.Errorf("the product role landed beside the claims rather than inside them: %v", out)
	}

	return claims
}

// requireClaims asserts the whole set: what came back must be what went in, with
// the one key this hook owns set to want — or removed, when want is empty.
//
// The set rather than a list of names, because both directions matter. A hook
// that rebuilt the claims from the members it knows about would drop `iss` and
// `app_metadata` and pass any presence check; a hook that added a key nobody
// asked for would pass it too. Asserted on every path and not only the happy
// one: replacing the claims with an empty object satisfies "no product role"
// perfectly and takes sign-in down for every invited account.
func requireClaims(t *testing.T, in, out map[string]any, want string) {
	t.Helper()

	claims := claimsOf(t, out)

	expected := maps.Clone(claimsIn(t, in))
	if want == "" {
		delete(expected, "cadence_role")
	} else {
		expected["cadence_role"] = want
	}

	// Both sides through encoding/json — it sorts object keys, and it puts the
	// fixture's numbers into the same representation the returned document was
	// decoded into.
	if normalise(t, expected) == normalise(t, claims) {
		return
	}

	for _, name := range mandatoryClaims {
		if _, ok := claims[name]; !ok {
			t.Errorf("the hook dropped %q; GoTrue answers 500 rather than issuing a token", name)
		}
	}

	t.Errorf("claims came back as %s, want %s", normalise(t, claims), normalise(t, expected))
}

func normalise(t *testing.T, value map[string]any) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encoding claims: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decoding claims: %v", err)
	}

	again, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-encoding claims: %v", err)
	}

	return string(again)
}

// seedProfiles writes one profile per product role, the only way there is:
// through the service path.
func seedProfiles(t *testing.T, db *testsupport.Database) {
	t.Helper()

	service := testsupport.Connect(t, db.ServiceAppURL)
	if _, err := service.Exec(t.Context(), `
		SELECT set_config('app.actor_job', 'test.seed', false);
		SET ROLE cadence_service;
	`); err != nil {
		t.Fatalf("assuming the service role: %v", err)
	}

	for subject, role := range provisioned {
		if _, err := service.Exec(t.Context(), `
			INSERT INTO app.profiles (user_id, role, full_name, timezone)
			VALUES ($1, $2, 'Марина Крылова', 'Europe/Moscow')
		`, subject, role); err != nil {
			t.Fatalf("seeding the %s profile: %v", role, err)
		}
	}
}

// The hook writes cadence_role deterministically, both ways: a user with a
// profile gets their role, a user without one gets the key removed. The second
// half is not an edge case — an invited account that has not been provisioned is
// a real state, and the seam refuses it with a reason of its own.
func TestTheHookWritesTheRoleDeterministically(t *testing.T) {
	db := cluster.NewDatabase(t)
	seedProfiles(t, db)
	hook := asHook(t, db)

	// All three, not one. The hook copies profiles.role verbatim and the seam
	// resolves it through a closed map of the same three values; a suite that
	// only ever sends a doctor through would not notice the day the two sets
	// stopped agreeing. TestTheRoleTheHookIssuesIsOneTheSeamAccepts closes the
	// other half of that.
	for subject, want := range provisioned {
		t.Run(want, func(t *testing.T) {
			in := event(subject, nil)
			requireClaims(t, in, call(t, hook, in), want)
		})
	}

	t.Run("an unprovisioned user gets no role and keeps everything else", func(t *testing.T) {
		in := event(unprovisioned, nil)
		requireClaims(t, in, call(t, hook, in), "")
	})
}

// The value the hook puts in the token has to be one the seam will take. The
// hook reads app.profiles.role, whose CHECK admits three values; WithCaller
// resolves the claim through a closed map with three keys. Nothing but this
// holds the two sets together — a migration widening the CHECK would mint tokens
// carrying a role the seam answers with ErrUnknownRole, which is a 403 for a
// correctly provisioned user and a green test suite.
func TestTheRoleTheHookIssuesIsOneTheSeamAccepts(t *testing.T) {
	db := cluster.NewDatabase(t)
	seedProfiles(t, db)
	hook := asHook(t, db)

	pool, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request-path pool: %v", err)
	}
	t.Cleanup(pool.Close)

	for subject := range provisioned {
		claims := claimsOf(t, call(t, hook, event(subject, nil)))

		role, ok := claims["cadence_role"].(string)
		if !ok {
			t.Fatalf("the hook issued no role for %s", subject)
		}

		caller := database.Caller{Subject: subject, Role: role}
		if err := database.WithCaller(
			t.Context(), pool, caller, func(context.Context, pgx.Tx) error { return nil },
		); err != nil {
			t.Errorf("the seam refused the role the hook issued (%q): %v", role, err)
		}
	}
}

// The event is formed by GoTrue out of user data, so a role already sitting in
// it is not trusted. Both branches are exercised, and the first one is the one
// that matters: a patient whose event arrives carrying `cadence_role: admin`
// must come back a patient, not an admin. The branch that merely deletes the key
// would satisfy the requirement on its own while the overwrite branch happily
// preserved a substituted role.
func TestTheHookDoesNotTrustARoleItWasHanded(t *testing.T) {
	db := cluster.NewDatabase(t)
	seedProfiles(t, db)
	hook := asHook(t, db)

	substituted := map[string]any{"cadence_role": "admin"}

	t.Run("overwritten for a user with a profile", func(t *testing.T) {
		in := event(aDoctor, substituted)
		requireClaims(t, in, call(t, hook, in), "doctor")
	})

	t.Run("removed for a user without one", func(t *testing.T) {
		in := event(unprovisioned, substituted)
		requireClaims(t, in, call(t, hook, in), "")
	})
}

// The subject the token will carry is claims.sub; the profile is looked up by
// user_id. GoTrue fills both from one user record, so they cannot diverge today
// — but this is a SECURITY DEFINER function, and "the role of one user with the
// subject of another" is the one output that must never be produced. On a
// mismatch the hook issues no role rather than choosing which half to believe.
func TestTheHookRefusesAnEventNamingTwoDifferentUsers(t *testing.T) {
	db := cluster.NewDatabase(t)
	seedProfiles(t, db)
	hook := asHook(t, db)

	// The `sub` that is not UUID-shaped at all is the case that matters most.
	// The comparison casts, and a cast of `"x"` raises 22P02 — an exception out
	// of a SECURITY DEFINER function is a 500 on POST /token, which is the one
	// thing this hook is built never to do. The other fixture with a malformed
	// sub also has a malformed user_id, so it takes the earlier branch and never
	// reaches here.
	for name, sub := range map[string]string{
		"a different user":         unprovisioned,
		"a sub that is not a UUID": "x",
		"an empty sub":             "",
	} {
		t.Run(name, func(t *testing.T) {
			in := event(aDoctor, map[string]any{"sub": sub})
			requireClaims(t, in, call(t, hook, in), "")
		})
	}

	// Agreement is not string equality: `sub` and `user_id` are UUIDs, and two
	// spellings of one UUID name one user. A text comparison would strip the role
	// off a legitimate caller the day GoTrue changed its casing.
	t.Run("the same user in another case", func(t *testing.T) {
		in := event(aDoctor, map[string]any{"sub": "8A1F3B7C-0000-4000-8000-000000000002"})
		requireClaims(t, in, call(t, hook, in), "doctor")
	})

	// No sub to disagree with: the guard has nothing to compare and stands aside.
	// Such a token is refused downstream anyway — jwt_subject() returns NULL and
	// every policy then matches nothing — so the role in it is inert.
	t.Run("no sub at all", func(t *testing.T) {
		in := event(aDoctor, nil)
		delete(claimsIn(t, in), "sub")

		requireClaims(t, in, call(t, hook, in), "doctor")
	})
}

// Token issuance must not fail on the database's account. An event the hook does
// not understand comes back byte for byte, so that a hook meeting a GoTrue whose
// event shape has changed degrades to "no product role" rather than raising.
//
// What that does *not* buy is a token: GoTrue refuses an output with no claims
// object itself, with 500 and "output claims field is missing". The property
// here is narrower than "sign-in keeps working", and is written as what it is.
func TestTheHookReturnsAnEventItDoesNotUnderstandUnchanged(t *testing.T) {
	db := cluster.NewDatabase(t)
	hook := asHook(t, db)

	for name, in := range map[string]map[string]any{
		"no claims at all":              {"user_id": aDoctor},
		"claims that are not an object": {"user_id": aDoctor, "claims": "nonsense"},
		"no user at all":                {"claims": map[string]any{"sub": aDoctor}},
		"a user that is not a UUID":     {"user_id": "not-a-uuid", "claims": map[string]any{"sub": "x"}},
		"an empty event":                {},
	} {
		t.Run(name, func(t *testing.T) {
			before, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("encoding the event: %v", err)
			}

			after, err := json.Marshal(call(t, hook, in))
			if err != nil {
				t.Fatalf("re-encoding what came back: %v", err)
			}

			// Both sides through encoding/json, whose object keys are sorted, so
			// this compares documents rather than jsonb's storage order.
			if string(after) != string(before) {
				t.Errorf("the hook returned %s, want %s unchanged", after, before)
			}
		})
	}
}

// The identity provider's role reaches the hook and nothing else. Its own
// database user is made a member of cadence_auth_hook, so what that membership
// carries is the whole of what GoTrue can do to the application schema.
func TestTheHookRoleReachesNothingButTheHook(t *testing.T) {
	db := cluster.NewDatabase(t)
	hook := asHook(t, db)

	for name, statement := range map[string]string{
		"reading profiles":             `SELECT count(*) FROM app.profiles`,
		"reading the subject function": `SELECT app.jwt_subject()`,
		"reading the audit log":        `SELECT count(*) FROM app.audit_log`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := hook.Exec(t.Context(), statement)
			requireInsufficientPrivilege(t, err)
		})
	}
}

// The membership is the whole of the hook's access control, which makes its
// absence worth a witness. This is the database half of "revoking the membership
// breaks token issuance" — the other half needs a GoTrue, which the harness does
// not have; it was measured by hand and recorded in the spec.
//
// Every role in the chain that is not a member is tried, rather than one of
// them: a grant that drifted to a wider role — PUBLIC, or one of the
// impersonation targets — is exactly the mistake this is here to catch, and a
// single probe would only catch it for the role it happened to name.
func TestOnlyAMemberOfTheHookRoleMayCallTheHook(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	// The two pools, as themselves: a different session_user rather than a role
	// assumed inside one.
	for name, dsn := range map[string]string{
		testsupport.AppRole:        db.AppURL,
		testsupport.ServiceAppRole: db.ServiceAppURL,
	} {
		t.Run(name, func(t *testing.T) {
			conn := testsupport.Connect(t, dsn)

			_, err := conn.Exec(ctx, `SELECT app.access_token_hook('{}'::jsonb)`)
			requireInsufficientPrivilege(t, err)
		})
	}

	// And every role a request or the service path is impersonated into.
	for _, role := range testsupport.ImpersonationRoles() {
		t.Run(role, func(t *testing.T) {
			conn := testsupport.Connect(t, db.MigrationURL)

			if _, err := conn.Exec(ctx, `GRANT `+role+` TO CURRENT_USER`); err != nil {
				t.Fatalf("taking %s: %v", role, err)
			}
			if _, err := conn.Exec(ctx, `SET ROLE `+role); err != nil {
				t.Fatalf("becoming %s: %v", role, err)
			}

			_, err := conn.Exec(ctx, `SELECT app.access_token_hook('{}'::jsonb)`)
			requireInsufficientPrivilege(t, err)
		})
	}
}

// A refusal from the database is not swallowed. The hook returns an event it
// cannot parse unchanged, and the temptation on the next reading of that comment
// is to widen it into `EXCEPTION WHEN OTHERS THEN RETURN event` — after which a
// revoked privilege stops being an error and becomes every user silently losing
// their product role, for as long as nobody looks.
//
// The privilege revoked here is the definer's own read of profiles. It is the
// dependency the function cannot do without, and taking it away is the one thing
// that must reach the caller as an error.
func TestTheHookDoesNotSwallowARefusalFromTheDatabase(t *testing.T) {
	db := cluster.NewDatabase(t)
	seedProfiles(t, db)
	ctx := t.Context()

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `REVOKE SELECT ON app.profiles FROM cadence_owner`); err != nil {
		t.Fatalf("revoking the definer's read: %v", err)
	}

	hook := asHook(t, db)

	encoded, err := json.Marshal(event(aDoctor, nil))
	if err != nil {
		t.Fatalf("encoding the event: %v", err)
	}

	_, err = hook.Exec(ctx, `SELECT app.access_token_hook($1::jsonb)`, string(encoded))
	requireInsufficientPrivilege(t, err)
}

// requireInsufficientPrivilege pins a refusal to one SQLSTATE. 42501 has three
// possible causes in this schema — a missing grant, a missing policy, and forced
// row level security — so a test naming a refusal has to name which, and the
// message is carried into the failure for exactly that reason.
func requireInsufficientPrivilege(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("it was reached")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("refused with %v, want SQLSTATE 42501", err)
	}
}

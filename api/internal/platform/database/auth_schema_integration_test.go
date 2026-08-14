//go:build integration

package database_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// Zero access to the `auth` schema.
//
// POST /recover is public and unauthenticated, and auth.users.confirmation_token
// is not a hash of the credential — it is the `token` parameter of the link
// itself. One SELECT on that column is therefore a password reset for any
// doctor, performed without the admin key and without anything the audit can
// attribute. That is why the absence of access is a control rather than an
// omission, and why it is verified here instead of being left to the fact that
// nobody has written the GRANT yet.
//
// The three registries over `app` declare what exists and reconcile in both
// directions. This one is inverted: there is nothing to declare, so the walk
// asks the catalogue and fails when it answers yes. Both halves are enumerated
// rather than listed — every role reachable by transition from either LOGIN
// role, and every relation the identity provider created — so a membership
// added later widens the walk instead of slipping past it.
//
// A migration cannot be what proves this: the chain never sees the `auth`
// schema, which GoTrue creates and migrates on its own. What the walk needs is
// the arrangement a deployment has — one database holding both schemas — and
// that is what the harness builds here.

// authObjectsTheWalkMustHaveSeen: the two relations the whole control is about,
// and the one sequence GoTrue creates. Named so that a walk over an `auth`
// schema the identity provider never migrated cannot come out green on an empty
// catalogue — and so that the sequence branch of the walk is known to have
// something to walk over.
func authObjectsTheWalkMustHaveSeen() []string {
	return []string{"one_time_tokens", "refresh_tokens_id_seq", "users"}
}

// rolesReachableFromTheConnectionRoles is the transitive closure of role
// membership over the two roles the API connects as.
//
// Membership rather than inheritance: SET ROLE is permitted by the membership,
// and every one of these grants carries WITH INHERIT FALSE, so a walk written
// against pg_has_role(..., 'USAGE') would see none of the four roles a request
// actually runs as.
const rolesReachableFromTheConnectionRoles = `
	WITH RECURSIVE reachable(oid) AS (
			SELECT oid FROM pg_roles WHERE rolname = ANY($1::text[])
		UNION
			SELECT m.roleid FROM pg_auth_members m JOIN reachable ON m.member = reachable.oid
	)
	SELECT r.rolname FROM pg_roles r JOIN reachable ON reachable.oid = r.oid ORDER BY r.rolname`

// authSchemaWalk asks the catalogue for every privilege the given roles hold
// anywhere in `auth`, and returns one line per answer of yes.
//
// Four branches because Postgres has four questions here and they are not
// interchangeable: the schema itself, a relation, a sequence, and a grant made
// on a column of a relation whose table-wide privilege is absent. The last is
// the one a hand-written REVOKE leaves behind, and it is the shape a grant on
// auth.users.confirmation_token alone would take.
const authSchemaWalk = `
	WITH walked AS (
			SELECT r.rolname, v.verb, 'schema auth' AS object,
			       has_schema_privilege(r.rolname, 'auth', v.verb) AS held
			  FROM unnest($1::text[]) AS r(rolname)
			  CROSS JOIN unnest(ARRAY['USAGE', 'CREATE']) AS v(verb)
		UNION ALL
			SELECT r.rolname, v.verb, 'auth.' || c.relname,
			       has_table_privilege(r.rolname, c.oid, v.verb)
			  FROM unnest($1::text[]) AS r(rolname)
			  CROSS JOIN unnest(ARRAY[
			      'SELECT', 'INSERT', 'UPDATE', 'DELETE', 'TRUNCATE', 'REFERENCES', 'TRIGGER'
			  ]) AS v(verb)
			  CROSS JOIN pg_class c
			  JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = 'auth' AND c.relkind = ANY(ARRAY['r', 'p', 'v', 'm', 'f'])
		UNION ALL
			SELECT r.rolname, v.verb, 'column of auth.' || c.relname,
			       has_any_column_privilege(r.rolname, c.oid, v.verb)
			  FROM unnest($1::text[]) AS r(rolname)
			  CROSS JOIN unnest(ARRAY['SELECT', 'INSERT', 'UPDATE', 'REFERENCES']) AS v(verb)
			  CROSS JOIN pg_class c
			  JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = 'auth' AND c.relkind = ANY(ARRAY['r', 'p', 'v', 'm', 'f'])
		UNION ALL
			SELECT r.rolname, v.verb, 'sequence auth.' || c.relname,
			       has_sequence_privilege(r.rolname, c.oid, v.verb)
			  FROM unnest($1::text[]) AS r(rolname)
			  CROSS JOIN unnest(ARRAY['USAGE', 'SELECT', 'UPDATE']) AS v(verb)
			  CROSS JOIN pg_class c
			  JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = 'auth' AND c.relkind = 'S'
	)
	SELECT DISTINCT rolname || ' holds ' || verb || ' on ' || object
	  FROM walked WHERE held ORDER BY 1`

const authObjects = `
	SELECT c.relname FROM pg_class c
	  JOIN pg_namespace n ON n.oid = c.relnamespace
	 WHERE n.nspname = 'auth' AND c.relkind = ANY(ARRAY['r', 'p', 'v', 'm', 'f', 'S'])
	 ORDER BY c.relname`

const authSchemaOwner = `SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'auth'`

func TestNothingTheRequestPathCanBecomeReachesTheAuthSchema(t *testing.T) {
	db := cluster.NewDatabase(t)

	// The identity provider migrates its schema into the database the chain
	// already owns. Without it the walk would run over a schema with no
	// relations in it and report nothing, which is indistinguishable from a
	// walk that found nothing.
	key := testsupport.NewES256Key(t, "auth-schema-walk")
	testsupport.StartGoTrueOn(t, db, testsupport.GoTrueJWKS(t,
		testsupport.JWKEntry{Key: key, Signing: true}))

	conn := testsupport.Connect(t, db.SuperuserURL)
	ctx := t.Context()

	roles := scanStrings(t, conn, rolesReachableFromTheConnectionRoles,
		[]string{testsupport.AppRole, testsupport.ServiceAppRole})

	// The walk's reach, asserted rather than assumed: a closure query that
	// returned only its two seeds would leave the four roles a request actually
	// runs as unmeasured, and every assertion below would still be green.
	for _, role := range append(testsupport.ImpersonationRoles(),
		testsupport.AppRole, testsupport.ServiceAppRole) {
		if !slices.Contains(roles, role) {
			t.Fatalf("the walk does not reach %s; it covers %v", role, roles)
		}
	}

	objects := scanStrings(t, conn, authObjects, nil)
	for _, want := range authObjectsTheWalkMustHaveSeen() {
		if !slices.Contains(objects, want) {
			t.Fatalf("auth.%s is not in the schema, so the walk measured nothing; it saw %v", want, objects)
		}
	}

	t.Run("the schema belongs to the identity provider", func(t *testing.T) {
		// Owned by the provider's own role and by nothing the chain creates: the
		// owner holds every privilege on every relation inherently, so an `auth`
		// owned by cadence_owner would hand the whole schema to the chain
		// without a single GRANT for the walk to find.
		var owner string
		if err := conn.QueryRow(ctx, authSchemaOwner).Scan(&owner); err != nil {
			t.Fatalf("reading the owner of the auth schema: %v", err)
		}
		if owner != testsupport.GoTrueRole {
			t.Errorf("the auth schema is owned by %s, want %s", owner, testsupport.GoTrueRole)
		}
		if slices.Contains(roles, owner) {
			t.Errorf("the auth schema is owned by %s, which the request path can become", owner)
		}
	})

	t.Run("no role holds anything in it", func(t *testing.T) {
		if held := scanStrings(t, conn, authSchemaWalk, roles); len(held) > 0 {
			t.Errorf("the request path can reach the identity provider's schema:\n  %v", held)
		}
	})

	// The walk is a guard, and a guard that cannot go red is a comment. Each
	// grant below is issued by hand, walked, and taken away again — the middle
	// step naming the exact line the walk has to produce, because a walk that
	// answered "something is wrong" for any grant at all would pass this while
	// enumerating one relation and one verb.
	for _, planted := range []struct {
		name    string
		grant   string
		revoke  string
		finding string
	}{
		{
			name:    "USAGE on the schema",
			grant:   `GRANT USAGE ON SCHEMA auth TO cadence_app`,
			revoke:  `REVOKE USAGE ON SCHEMA auth FROM cadence_app`,
			finding: "cadence_app holds USAGE on schema auth",
		},
		{
			// Through a role rather than on one of the two the API connects as:
			// the grant lands where only the membership closure looks.
			name:    "SELECT on the table holding the credential",
			grant:   `GRANT SELECT ON auth.users TO cadence_doctor`,
			revoke:  `REVOKE SELECT ON auth.users FROM cadence_doctor`,
			finding: "cadence_doctor holds SELECT on auth.users",
		},
		{
			// The credential itself, granted the way a REVOKE on the table
			// leaves it: no table-wide privilege anywhere, and the column still
			// readable.
			name:    "SELECT on the credential column alone",
			grant:   `GRANT SELECT (confirmation_token) ON auth.users TO cadence_service`,
			revoke:  `REVOKE SELECT (confirmation_token) ON auth.users FROM cadence_service`,
			finding: "cadence_service holds SELECT on column of auth.users",
		},
		{
			// A sequence is not a table to Postgres and answers a different
			// question, so without this the fourth branch of the walk would be
			// a query nobody has ever seen return a row.
			name:    "USAGE on the one sequence in the schema",
			grant:   `GRANT USAGE ON SEQUENCE auth.refresh_tokens_id_seq TO cadence_admin`,
			revoke:  `REVOKE USAGE ON SEQUENCE auth.refresh_tokens_id_seq FROM cadence_admin`,
			finding: "cadence_admin holds USAGE on sequence auth.refresh_tokens_id_seq",
		},
	} {
		t.Run("the walk sees "+planted.name, func(t *testing.T) {
			if _, err := conn.Exec(ctx, planted.grant); err != nil {
				t.Fatalf("planting the grant: %v", err)
			}
			t.Cleanup(func() {
				if _, err := conn.Exec(ctx, planted.revoke); err != nil {
					t.Errorf("taking the grant away: %v", err)
				}
			})

			held := scanStrings(t, conn, authSchemaWalk, roles)
			if !slices.Contains(held, planted.finding) {
				t.Fatalf("the walk did not report %q; it reported %v", planted.finding, held)
			}
		})
	}

	// A grant to a role two steps out. Nothing in the schema today is deeper
	// than one membership, so the recursion in the closure query is the one
	// thing above that a flat join would satisfy just as well — and a flat join
	// would let exactly this grant through.
	t.Run("the walk sees a grant to a role reached in two steps", func(t *testing.T) {
		const (
			probe = "auth_walk_probe"

			create = `CREATE ROLE ` + probe + ` NOLOGIN`
			grant  = `GRANT SELECT ON auth.users TO ` + probe

			// INHERIT FALSE, as every membership the chain grants: the privilege
			// then belongs to nobody until a SET ROLE, so has_table_privilege
			// answers no for cadence_doctor and the grant is invisible to
			// anything that does not walk the membership through to the end.
			member = `GRANT ` + probe + ` TO cadence_doctor WITH INHERIT FALSE`

			// DROP ROLE alone is refused while the role holds a privilege on
			// somebody's table; the membership goes with the role.
			disown = `DROP OWNED BY ` + probe
			drop   = `DROP ROLE ` + probe
		)

		for _, statement := range []string{create, member, grant} {
			if _, err := conn.Exec(ctx, statement); err != nil {
				t.Fatalf("planting the two-step grant with %q: %v", statement, err)
			}
		}
		t.Cleanup(func() {
			for _, statement := range []string{disown, drop} {
				if _, err := conn.Exec(ctx, statement); err != nil {
					t.Errorf("taking the two-step grant away with %q: %v", statement, err)
				}
			}
		})

		// Read again rather than reused: the closure is what is under test here,
		// and the roles above were read before this membership existed.
		reached := scanStrings(t, conn, rolesReachableFromTheConnectionRoles,
			[]string{testsupport.AppRole, testsupport.ServiceAppRole})

		held := scanStrings(t, conn, authSchemaWalk, reached)
		if finding := probe + " holds SELECT on auth.users"; !slices.Contains(held, finding) {
			t.Fatalf("the walk did not report %q; it reached %v and reported %v", finding, reached, held)
		}
	})

	// And with every one of them gone the walk is empty again, so the green
	// above was the schema's state rather than a walk that never reports
	// anything.
	t.Run("and is empty again once they are taken away", func(t *testing.T) {
		// Read again rather than reused: the probe was granted through a
		// membership that did not exist when the roles above were read, so the
		// one planted grant that could outlive its subtest is the one this
		// would otherwise not be looking for.
		reached := scanStrings(t, conn, rolesReachableFromTheConnectionRoles,
			[]string{testsupport.AppRole, testsupport.ServiceAppRole})

		if held := scanStrings(t, conn, authSchemaWalk, reached); len(held) > 0 {
			t.Errorf("the planted grants outlived the tests that planted them: %v", held)
		}
	})
}

// The harness creates the schema with the right owner, so the assertion above
// reads back a fixture this file arranged. What decides it outside a test is the
// development stack's one-shot step, and nothing else in the repository names
// that role — the deployment inherits the same statement.
//
// Whole line rather than a substring, for the reason the image drift check
// learned by measurement: a commented-out line still contains the text.
func TestTheStackGivesTheAuthSchemaToTheIdentityProvider(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(testsupport.ModuleRoot(t), "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading the development stack: %v", err)
	}

	want := fmt.Sprintf("ALTER SCHEMA auth OWNER TO %s;", testsupport.GoTrueRole)

	given := slices.ContainsFunc(strings.Split(string(compose), "\n"), func(line string) bool {
		return strings.TrimSpace(line) == want
	})

	if !given {
		t.Fatalf("no line of docker-compose.yml is %q — the walk over the auth schema is written "+
			"against an owner the stack no longer sets", want)
	}
}

func scanStrings(t *testing.T, conn *pgx.Conn, query string, arg []string) []string {
	t.Helper()

	var args []any
	if arg != nil {
		args = append(args, arg)
	}

	rows, err := conn.Query(t.Context(), query, args...)
	if err != nil {
		t.Fatalf("querying the catalogue: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		found = append(found, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	return found
}

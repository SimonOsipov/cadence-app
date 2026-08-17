//go:build integration

package database_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The three registries.
//
// Each declares what the migrations are supposed to have produced and reconciles
// it against the catalogue, in both directions: a missing entry and a surplus one
// both fail. That is the shape the whole access model needs, because the failure
// mode here is not a wrong value — it is a privilege nobody meant to grant,
// sitting where nobody looks. `make gate` cannot see it; a reviewer reading a
// migration cannot see it either, because what is wrong is what the file does
// not say.

// grantRegistry is the table of the spec's technical detail, written out.
//
// Verbs are table-wide. cadence_owner holds everything on its own tables
// inherently rather than by grant, and that is declared rather than excluded:
// what governs the owner is FORCE, not privileges, and a reader should meet that
// fact here rather than wonder why a role is missing.
//
// The two LOGIN roles hold nothing at all. They never touch a table outside a
// seam, and the seam has already become one of the four roles below by the time
// a statement runs.
func grantRegistry() map[string][]string {
	everything := []string{"DELETE", "INSERT", "REFERENCES", "SELECT", "TRIGGER", "TRUNCATE", "UPDATE"}
	crud := []string{"DELETE", "INSERT", "SELECT", "UPDATE"}

	registry := map[string][]string{
		"profiles/cadence_patient":              {"SELECT"},
		"profiles/cadence_doctor":               {"SELECT"},
		"profiles/cadence_admin":                crud,
		"profiles/cadence_service":              {"INSERT", "SELECT", "UPDATE"},
		"profiles/cadence_owner":                everything,
		"patient_profiles/cadence_patient":      {"SELECT"},
		"patient_profiles/cadence_doctor":       {"SELECT"},
		"patient_profiles/cadence_admin":        crud,
		"patient_profiles/cadence_service":      {"INSERT", "SELECT"},
		"patient_profiles/cadence_owner":        everything,
		"provider_profiles/cadence_patient":     {"SELECT"},
		"provider_profiles/cadence_doctor":      {"SELECT"},
		"provider_profiles/cadence_admin":       crud,
		"provider_profiles/cadence_service":     {"INSERT", "SELECT"},
		"provider_profiles/cadence_owner":       everything,
		"care_team_assignments/cadence_patient": {"SELECT"},
		"care_team_assignments/cadence_doctor":  {"SELECT"},
		"care_team_assignments/cadence_admin":   crud,
		"care_team_assignments/cadence_service": {"DELETE", "INSERT", "SELECT"},
		"care_team_assignments/cadence_owner":   everything,
		"user_preferences/cadence_patient":      {"SELECT", "UPDATE"},
		"user_preferences/cadence_admin":        crud,
		"user_preferences/cadence_service":      {"INSERT", "SELECT"},
		"user_preferences/cadence_owner":        everything,
		"audit_log/cadence_admin":               {"SELECT"},
		"audit_log/cadence_service":             {"INSERT"},
		"audit_log/cadence_owner":               everything,
		// §04's row for invites: the doctor creates, the admin reads, the patient
		// holds nothing. The service path reads too, because the transaction that
		// writes the record runs there and has to recognise its own.
		"invites/cadence_doctor":  {"INSERT"},
		"invites/cadence_admin":   {"SELECT"},
		"invites/cadence_service": {"INSERT", "SELECT"},
		"invites/cadence_owner":   everything,
	}

	// Everybody else holds nothing, stated rather than left out: an absent key
	// and an empty one are the same to a map and very different to a reader.
	for _, table := range identityTables() {
		for _, role := range testsupport.ChainRoles() {
			key := table + "/" + role
			if _, declared := registry[key]; !declared {
				registry[key] = nil
			}
		}
	}

	return registry
}

func TestTheGrantsAreTheOnesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	ctx := t.Context()

	declared := grantRegistry()

	for _, table := range identityTables() {
		for _, role := range testsupport.ChainRoles() {
			var held []string
			if err := conn.QueryRow(ctx, `
				SELECT coalesce(array_agg(verb ORDER BY verb), '{}')
				FROM unnest(ARRAY[
					'SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER'
				]) AS verb
				WHERE has_table_privilege($1, $2, verb)
			`, role, "app."+table).Scan(&held); err != nil {
				t.Fatalf("reading %s's privileges on %s: %v", role, table, err)
			}

			key := table + "/" + role
			want := declared[key]
			if len(held) == 0 {
				held = nil
			}

			if !slices.Equal(held, want) {
				t.Errorf("%s holds %v on app.%s, want %v", role, held, table, want)
			}
		}
	}
}

// columnGrantRegistry is where "may see" and "may change" come apart, and it is
// the mechanism behind two acceptance criteria: a patient cannot change their
// own role, and cannot write the clinical fields of their own card.
//
// Only the columns a role may UPDATE without holding UPDATE on the table.
func columnGrantRegistry() map[string][]string {
	return map[string][]string{
		"profiles/cadence_patient":         {"full_name", "locale", "timezone"},
		"patient_profiles/cadence_patient": {"target_weight_kg"},
	}
}

func TestTheColumnGrantsAreTheOnesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	ctx := t.Context()

	rows, err := conn.Query(ctx, `
		SELECT c.relname, r.rolname, a.attname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		CROSS JOIN pg_roles r
		WHERE n.nspname = $1 AND c.relkind = 'r' AND r.rolname = ANY($2)
		  AND has_column_privilege(r.rolname, c.oid, a.attnum, 'UPDATE')
		  AND NOT has_table_privilege(r.rolname, c.oid, 'UPDATE')
		ORDER BY c.relname, r.rolname, a.attname
	`, testsupport.AppSchema, testsupport.ChainRoles())
	if err != nil {
		t.Fatalf("reading the column grants: %v", err)
	}
	defer rows.Close()

	found := map[string][]string{}
	for rows.Next() {
		var table, role, column string
		if err := rows.Scan(&table, &role, &column); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		found[table+"/"+role] = append(found[table+"/"+role], column)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	declared := columnGrantRegistry()
	for key, want := range declared {
		if !slices.Equal(found[key], want) {
			t.Errorf("%s: column UPDATE grants are %v, want %v", key, found[key], want)
		}
	}
	for key, held := range found {
		if _, ok := declared[key]; !ok {
			t.Errorf("%s holds column UPDATE grants nobody declared: %v", key, held)
		}
	}

	// The two criteria this registry exists for, asserted as themselves rather
	// than left to be inferred from the list above.
	for _, forbidden := range []struct{ table, column string }{
		{"profiles", "role"},
		{"profiles", "user_id"},
		{"patient_profiles", "dob"},
		{"patient_profiles", "sex"},
		{"patient_profiles", "height_cm"},
	} {
		var permitted bool
		if err := conn.QueryRow(
			ctx,
			`SELECT has_column_privilege($1, $2, $3, 'UPDATE')`,
			testsupport.PatientRole, "app."+forbidden.table, forbidden.column,
		).Scan(&permitted); err != nil {
			t.Fatalf("reading the patient's grant on %s.%s: %v", forbidden.table, forbidden.column, err)
		}
		if permitted {
			t.Errorf("the patient may write %s.%s", forbidden.table, forbidden.column)
		}
	}
}

// policyPredicates declares what each policy actually says, so that a policy
// widened to USING (true) fails here rather than two steps later.
//
// Six shapes, and the repetition is the point: a predicate that does not match
// one of them is either a seventh shape nobody decided on, or one of the six
// written wrong. Deparsed by Postgres, so the text is canonical rather than the
// text anybody typed — which is why it is dumped from the catalogue once and
// then read as a declaration.
//
// The behaviour these predicates produce is proven by the regression suite two
// steps from here. What this closes is the gap in between: until then, "the
// policy is in place" would rest on its name alone.
func policyPredicates() map[string]string {
	const (
		anything    = "true | -"
		anyWrite    = "- | true"
		anythingRW  = "true | true"
		ownRow      = "(user_id = app.jwt_subject()) | -"
		ownRowWrite = "(user_id = app.jwt_subject()) | (user_id = app.jwt_subject())"

		// The one shape in the schema whose body names a product role, and what
		// it names is the value in the row rather than the caller: the service
		// path brings a patient or a doctor into being and not an admin. Written
		// as IN in 000006 and deparsed by Postgres into = ANY.
		//
		// USING stays open on the UPDATE side. The service path may still read
		// and rewrite an admin's row — correcting the name of one is not the
		// escalation — it may only not produce the value.
		noAdmin = "(role = ANY (ARRAY['patient'::text, 'doctor'::text]))"
	)

	// The care team read in one direction or the other. The table being read
	// supplies the row; the subquery supplies the relation.
	through := func(table, mine, theirs string) string {
		return fmt.Sprintf(
			"(EXISTS ( SELECT FROM app.care_team_assignments WHERE "+
				"((care_team_assignments.%s = %s.user_id) AND "+
				"(care_team_assignments.%s = app.jwt_subject())))) | -",
			mine, table, theirs,
		)
	}

	return map[string]string{
		"audit_log_admin_read": anything,
		// The bootstrap command runs through no seam, so there is no published
		// actor to reconcile against: what is pinned is that the owner may sign as
		// a job and never in a person's name.
		"audit_log_bootstrap_insert": "- | (actor_job IS NOT NULL)",
		"audit_log_service_insert": "- | (((actor_id)::text = lower(NULLIF(current_setting(" +
			"'app.actor_id'::text, true), ''::text))) OR (actor_job = NULLIF(current_setting(" +
			"'app.actor_job'::text, true), ''::text)))",
		"care_team_assignments_admin":          anythingRW,
		"care_team_assignments_mine_patient":   "(patient_id = app.jwt_subject()) | -",
		"care_team_assignments_mine_provider":  "(provider_id = app.jwt_subject()) | -",
		"care_team_assignments_service_delete": anything,
		"care_team_assignments_service_insert": anyWrite,
		"care_team_assignments_service_read":   anything,
		"invites_admin_read":                   anything,
		"invites_doctor_insert":                "- | (invited_by = app.jwt_subject())",
		"invites_service_insert":               anyWrite,
		"invites_service_read":                 anything,
		"patient_profiles_admin":               anythingRW,
		"patient_profiles_of_my_patients":      through("patient_profiles", "patient_id", "provider_id"),
		"patient_profiles_own_select":          ownRow,
		"patient_profiles_own_update":          ownRowWrite,
		"patient_profiles_service_insert":      anyWrite,
		"patient_profiles_service_read":        anything,
		"profiles_admin":                       anythingRW,
		"profiles_bootstrap_insert":            "- | (role = 'admin'::text)",
		"profiles_hook_read":                   anything,
		"profiles_of_my_patients":              through("profiles", "patient_id", "provider_id"),
		"profiles_of_my_specialists":           through("profiles", "provider_id", "patient_id"),
		"profiles_own_select":                  ownRow,
		"profiles_own_update":                  ownRowWrite,
		"profiles_service_insert":              "- | " + noAdmin,
		"profiles_service_read":                anything,
		"profiles_service_update":              "true | " + noAdmin,
		"provider_profiles_admin":              anythingRW,
		"provider_profiles_of_my_specialists":  through("provider_profiles", "provider_id", "patient_id"),
		"provider_profiles_own_select":         ownRow,
		"provider_profiles_service_insert":     anyWrite,
		"provider_profiles_service_read":       anything,
		"user_preferences_admin":               anythingRW,
		"user_preferences_own_select":          ownRow,
		"user_preferences_own_update":          ownRowWrite,
		"user_preferences_service_insert":      anyWrite,
		"user_preferences_service_read":        anything,
	}
}

func TestThePolicyPredicatesAreTheOnesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		SELECT policyname,
		       regexp_replace(
		           coalesce(qual, '-') || ' | ' || coalesce(with_check, '-'), '\s+', ' ', 'g')
		FROM pg_policies
		WHERE schemaname = $1
		ORDER BY policyname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the policy predicates: %v", err)
	}
	defer rows.Close()

	declared := policyPredicates()
	seen := map[string]bool{}

	for rows.Next() {
		var name, predicate string
		if err := rows.Scan(&name, &predicate); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		seen[name] = true

		want, ok := declared[name]
		if !ok {
			t.Errorf("policy %s exists and its predicate is not declared here", name)

			continue
		}
		if predicate != want {
			t.Errorf("policy %s reads\n  %s\nwant\n  %s", name, predicate, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	for name := range declared {
		if !seen[name] {
			t.Errorf("policy %s is declared here and does not exist", name)
		}
	}
}

// Nothing is granted on sequences, and it is a decision rather than an omission:
// an identity column's sequence leaks the number of values ever handed out — an
// upper bound on inserts that no policy filters, because row level security
// cannot be enabled on a sequence at all.
func TestNothingIsGrantedOnSequences(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	ctx := t.Context()

	var sequences []string
	rows, err := conn.Query(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'S'
		ORDER BY c.relname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("listing the sequences: %v", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		sequences = append(sequences, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	// The control: audit_log's identity column owns one, so a green result below
	// is an empty grant list rather than an empty schema.
	if len(sequences) == 0 {
		t.Fatal("the schema holds no sequences, so this test measured nothing")
	}

	for _, sequence := range sequences {
		for _, role := range testsupport.ChainRoles() {
			if role == testsupport.OwnerRole {
				continue
			}

			for _, verb := range []string{"USAGE", "SELECT", "UPDATE"} {
				var granted bool
				if err := conn.QueryRow(
					ctx,
					`SELECT has_sequence_privilege($1, $2, $3)`, role, "app."+sequence, verb,
				).Scan(&granted); err != nil {
					t.Fatalf("reading %s's %s on %s: %v", role, verb, sequence, err)
				}
				if granted {
					t.Errorf("%s holds %s on sequence app.%s", role, verb, sequence)
				}
			}
		}
	}
}

// policyRegistry is the set of policies, their verbs and their TO — reconciled
// exactly, because a policy nobody declared is a door nobody opened on purpose.
func policyRegistry() []string {
	return []string{
		"audit_log audit_log_admin_read SELECT {cadence_admin}",
		"audit_log audit_log_bootstrap_insert INSERT {cadence_owner}",
		"audit_log audit_log_service_insert INSERT {cadence_service}",
		"care_team_assignments care_team_assignments_admin ALL {cadence_admin}",
		"care_team_assignments care_team_assignments_mine_patient SELECT {cadence_patient}",
		"care_team_assignments care_team_assignments_mine_provider SELECT {cadence_doctor}",
		"care_team_assignments care_team_assignments_service_delete DELETE {cadence_service}",
		"care_team_assignments care_team_assignments_service_insert INSERT {cadence_service}",
		"care_team_assignments care_team_assignments_service_read SELECT {cadence_service}",
		"invites invites_admin_read SELECT {cadence_admin}",
		"invites invites_doctor_insert INSERT {cadence_doctor}",
		"invites invites_service_insert INSERT {cadence_service}",
		"invites invites_service_read SELECT {cadence_service}",
		"patient_profiles patient_profiles_admin ALL {cadence_admin}",
		"patient_profiles patient_profiles_of_my_patients SELECT {cadence_doctor}",
		"patient_profiles patient_profiles_own_select SELECT {cadence_patient}",
		"patient_profiles patient_profiles_own_update UPDATE {cadence_patient}",
		"patient_profiles patient_profiles_service_insert INSERT {cadence_service}",
		"patient_profiles patient_profiles_service_read SELECT {cadence_service}",
		"profiles profiles_admin ALL {cadence_admin}",
		"profiles profiles_bootstrap_insert INSERT {cadence_owner}",
		"profiles profiles_hook_read SELECT {cadence_owner}",
		"profiles profiles_of_my_patients SELECT {cadence_doctor}",
		"profiles profiles_of_my_specialists SELECT {cadence_patient}",
		"profiles profiles_own_select SELECT {cadence_doctor,cadence_patient}",
		"profiles profiles_own_update UPDATE {cadence_patient}",
		"profiles profiles_service_insert INSERT {cadence_service}",
		"profiles profiles_service_read SELECT {cadence_service}",
		"profiles profiles_service_update UPDATE {cadence_service}",
		"provider_profiles provider_profiles_admin ALL {cadence_admin}",
		"provider_profiles provider_profiles_of_my_specialists SELECT {cadence_patient}",
		"provider_profiles provider_profiles_own_select SELECT {cadence_doctor}",
		"provider_profiles provider_profiles_service_insert INSERT {cadence_service}",
		"provider_profiles provider_profiles_service_read SELECT {cadence_service}",
		"user_preferences user_preferences_admin ALL {cadence_admin}",
		"user_preferences user_preferences_own_select SELECT {cadence_patient}",
		"user_preferences user_preferences_own_update UPDATE {cadence_patient}",
		"user_preferences user_preferences_service_insert INSERT {cadence_service}",
		"user_preferences user_preferences_service_read SELECT {cadence_service}",
	}
}

func TestThePoliciesAreTheOnesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		-- The roles are sorted here rather than taken as the catalogue returns
		-- them: pg_policies lists them in OID order, so a declaration written in
		-- the order the CREATE POLICY used would be right on one cluster and
		-- wrong on the next.
		SELECT tablename, policyname, cmd,
		       '{' || array_to_string(ARRAY(SELECT unnest(roles) ORDER BY 1), ',') || '}'
		FROM pg_policies
		WHERE schemaname = $1
		ORDER BY tablename, policyname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the policies: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var table, name, cmd, roles string
		if err := rows.Scan(&table, &name, &cmd, &roles); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		found = append(found, fmt.Sprintf("%s %s %s %s", table, name, cmd, roles))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	want := policyRegistry()
	if !slices.Equal(found, want) {
		t.Errorf("policies:\n got %v\nwant %v", found, want)
	}
}

// A policy with no explicit TO applies to PUBLIC, which includes the service
// path — and would hand it the request path's row predicate without anybody
// writing that down. polroles = '{0}' is how PUBLIC is spelled in the catalogue.
func TestEveryPolicyNamesItsRoles(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	var public []string
	rows, err := conn.Query(t.Context(), `
		SELECT p.polname
		FROM pg_policy p
		JOIN pg_class c ON c.oid = p.polrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND p.polroles = '{0}'
		ORDER BY p.polname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the policy roles: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		public = append(public, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	if len(public) > 0 {
		t.Errorf("these policies apply to PUBLIC rather than to named roles: %v", public)
	}
}

// The *caller's* role is decided by TO and never by comparing a value. A policy
// body that read a role from a claim is the shape this whole block was rewritten
// to remove: privileges in Postgres are bound to a role, so an escalation there
// is closeable neither by a policy nor by a column.
//
// The literals are not forbidden outright any more, because 000006 has one
// legitimate use for them — profiles' service-path WITH CHECK, which names the
// closed set of roles the service path may write into the row. That is a
// constraint on the value, not a decision about the caller, and the two are told
// apart by two rules rather than by an exemption: the policies allowed to name a
// role at all are declared here, and no policy may name one in the same body
// that reads a claim.
//
// The second rule is what an exemption alone would lose. Adding
// `AND current_setting('request.jwt.claims') ... = 'doctor'` to one of the
// declared policies is the original mistake written inside the one place the
// literals are permitted.
func TestNoPolicyBodyDecidesTheCallersRole(t *testing.T) {
	// A fourth name appearing here is a policy that started deciding something,
	// and it fails until somebody writes down which. profiles_bootstrap_insert
	// joined the list with 000009 and is the same shape as the other two: the
	// closed set of values the caller may write, not a claim about the caller.
	mayNameARole := []string{
		"profiles_bootstrap_insert", "profiles_service_insert", "profiles_service_update",
	}

	// How a policy reaches the caller's identity: the pinned helper, or the
	// connection settings it is built on. Either one beside a product-role
	// literal is a role compared against a claim.
	claimReaders := []string{"app.jwt_subject()", "current_setting("}

	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		SELECT policyname, coalesce(qual, '') || ' ' || coalesce(with_check, '')
		FROM pg_policies
		WHERE schemaname = $1
		ORDER BY policyname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the policy bodies: %v", err)
	}
	defer rows.Close()

	var naming []string
	for rows.Next() {
		var name, body string
		if err := rows.Scan(&name, &body); err != nil {
			t.Fatalf("scanning: %v", err)
		}

		named := false
		for _, literal := range []string{"'patient'", "'doctor'", "'admin'"} {
			if !strings.Contains(body, literal) {
				continue
			}
			named = true

			for _, reader := range claimReaders {
				if strings.Contains(body, reader) {
					t.Errorf("policy %s names %s in a body that reads %s; "+
						"the caller's role is decided by TO", name, literal, reader)
				}
			}
		}

		if named {
			naming = append(naming, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	if !slices.Equal(naming, mayNameARole) {
		t.Errorf("the policies naming a product role are %v, want exactly %v", naming, mayNameARole)
	}
}

// A SECURITY DEFINER function with an unpinned search_path is the classic way a
// schema hands its owner's rights to whoever can create a temporary object.
func TestTheFunctionsAreTheOnesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		SELECT p.proname,
		       pg_get_userbyid(p.proowner),
		       p.prosecdef,
		       coalesce(array_to_string(p.proconfig, ','), ''),
		       coalesce(array_to_string(ARRAY(
		           SELECT r.rolname FROM pg_roles r
		           WHERE has_function_privilege(r.rolname, p.oid, 'EXECUTE')
		             AND r.rolname LIKE 'cadence\_%'
		           ORDER BY r.rolname
		       ), ','), '')
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1
		ORDER BY p.proname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the functions: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var name, owner, config, executors string
		var securityDefiner bool
		if err := rows.Scan(&name, &owner, &securityDefiner, &config, &executors); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		found = append(found, fmt.Sprintf("%s owner=%s secdef=%v config=%q execute=%s",
			name, owner, securityDefiner, config, executors))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	// cadence_migrator appears because it is a member of cadence_owner, which is
	// how the chain creates objects at all.
	want := []string{
		`access_token_hook owner=cadence_owner secdef=true config="search_path=pg_catalog, pg_temp" ` +
			`execute=cadence_auth_hook,cadence_migrator,cadence_owner`,
		`jwt_subject owner=cadence_owner secdef=false config="search_path=pg_catalog, pg_temp" ` +
			`execute=cadence_admin,cadence_doctor,cadence_migrator,cadence_owner,cadence_patient,cadence_service`,
	}
	if !slices.Equal(found, want) {
		t.Errorf("functions:\n got %v\nwant %v", found, want)
	}
}

// The constraint that shapes every policy in this schema, and the one that is
// invisible until it fires: a policy's subquery executes under the policies of
// the table it reads, so a back-reference is infinite recursion.
//
// The point of planting one is that "there is no recursion" is otherwise green
// in a world where somebody forgot to write the policy at all. This proves the
// arrangement is recursion-free because it was measured, not because nothing
// happened.
func TestABackReferenceBetweenPoliciesIsRecursion(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE `+testsupport.OwnerRole); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}

	// The one the schema deliberately does not have: care_team_assignments
	// reading profiles, whose own policies read care_team_assignments.
	if _, err := owner.Exec(ctx, `
		CREATE POLICY care_team_assignments_recursion_probe ON app.care_team_assignments
			FOR SELECT TO cadence_patient
			USING (EXISTS (
				SELECT FROM app.profiles
				WHERE profiles.user_id = care_team_assignments.patient_id
			))
	`); err != nil {
		t.Fatalf("planting the back-reference: %v", err)
	}

	app := testsupport.Connect(t, db.AppURL)
	if _, err := app.Exec(ctx, `SET ROLE `+testsupport.PatientRole); err != nil {
		t.Fatalf("impersonating the patient: %v", err)
	}

	var rows int
	err := app.QueryRow(ctx, `SELECT count(*) FROM app.profiles`).Scan(&rows)
	if err == nil {
		t.Fatal("a policy reading a table whose policies read it back returned an answer")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42P17" {
		t.Fatalf("the recursive read failed with %v, want SQLSTATE 42P17 (invalid_object_definition)", err)
	}

	// And without the probe it does not: the same read on the same connection
	// after the policy is gone has to succeed, or the assertion above would hold
	// for a schema that was recursive to begin with.
	if _, err := owner.Exec(
		ctx, `DROP POLICY care_team_assignments_recursion_probe ON app.care_team_assignments`,
	); err != nil {
		t.Fatalf("removing the back-reference: %v", err)
	}

	if err := app.QueryRow(ctx, `SELECT count(*) FROM app.profiles`).Scan(&rows); err != nil {
		t.Errorf("the read still fails with the back-reference gone: %v", err)
	}
}

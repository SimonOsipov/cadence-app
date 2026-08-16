//go:build integration

// The harness's own test: the arrangement the onboarding cycle needs is real rather than merely
// configured — the chain applied on the database GoTrue is connected to, the token hook registered
// and reached, and a reset that empties both schemas.
//
// A cycle test written against a harness whose hook was configured but unreachable would come out
// green with every token missing its product role, and the failure would surface as an
// authorization bug in step 5 rather than as a harness fault here.
package testsupport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var (
	cluster *testsupport.Cluster
	cycle   *testsupport.Cycle
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	startedCluster, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)
		os.Exit(1)
	}
	cluster = startedCluster

	startedCycle, err := testsupport.StartCycle(ctx, cluster)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the cycle harness: %v\n", err)

		if terminateErr := cluster.Terminate(ctx); terminateErr != nil {
			fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", terminateErr)
		}

		os.Exit(1)
	}
	cycle = startedCycle

	code := m.Run()

	if err := cycle.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the cycle harness: %v\n", err)
	}

	if err := cluster.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
	}

	os.Exit(code)
}

// The hook is reached, and what it wrote is in the token.
//
// Three things at once, deliberately: the chain applied on GoTrue's own database or the hook's
// SELECT finds no app.profiles, the membership granted or the EXECUTE is refused, and the hook
// named in the configuration. Any one missing produces the same observable — a signed token with no
// cadence_role — which is why the negative below is a separate test rather than an assumption.
func TestTheHarnessIssuesTokensCarryingTheProductRole(t *testing.T) {
	cycle.Reset(t)

	const (
		email    = "hook-reached@example.test"
		password = "harness-password-not-a-secret"
	)

	userID := createUser(t, email, password)
	insertProfile(t, userID, "doctor")

	claims := signIn(t, email, password)

	if got := claims["cadence_role"]; got != "doctor" {
		t.Fatalf("the token carries cadence_role %v, want \"doctor\" — the hook did not run, "+
			"or it ran against a database with no app.profiles in it", got)
	}
}

// The control: without it the test above would pass on a harness that wrote "doctor" into every
// token regardless of who asked.
func TestATokenForAUserWithNoProfileCarriesNoProductRole(t *testing.T) {
	cycle.Reset(t)

	const (
		email    = "no-profile@example.test"
		password = "harness-password-not-a-secret"
	)

	createUser(t, email, password)

	claims := signIn(t, email, password)

	if got, ok := claims["cadence_role"]; ok {
		t.Fatalf("a user with no profile got cadence_role %v; the claim must be absent", got)
	}
}

// Asserted in both directions: rows exist, then they do not.
func TestResetEmptiesBothTheApplicationAndTheAuthSchema(t *testing.T) {
	cycle.Reset(t)

	userID := createUser(t, "reset@example.test", "harness-password-not-a-secret")
	insertProfile(t, userID, "patient")

	if profiles, users := counts(t); profiles == 0 || users == 0 {
		t.Fatalf("the fixture wrote %d profiles and %d users; Reset would be measuring nothing",
			profiles, users)
	}

	applied := appliedGoTrueMigrations(t)
	if applied == 0 {
		t.Fatal("auth.schema_migrations is empty before Reset, so the exclusion below " +
			"has nothing to protect")
	}

	cycle.Reset(t)

	if profiles, users := counts(t); profiles != 0 || users != 0 {
		t.Errorf("after Reset there are %d profiles and %d auth users, want none of either",
			profiles, users)
	}

	// The mutation Reset has to fail against: emptying the provider's own bookkeeping tells a
	// restarted container to migrate a schema that is already there.
	if survived := appliedGoTrueMigrations(t); survived != applied {
		t.Errorf("Reset left %d rows in auth.schema_migrations, want the %d it started with",
			survived, applied)
	}
}

func appliedGoTrueMigrations(t *testing.T) int {
	t.Helper()

	conn := testsupport.Connect(t, cycle.DB.SuperuserURL)

	var applied int
	if err := conn.QueryRow(
		t.Context(), `SELECT count(*) FROM auth.schema_migrations`,
	).Scan(&applied); err != nil {
		t.Fatalf("counting the identity provider's migrations: %v", err)
	}

	return applied
}

// A per-test database is not the one GoTrue is connected to, and the absent `auth` schema is what
// tells them apart from the outside — which is why dropping one WITH (FORCE) severs no connection
// GoTrue is holding open.
func TestPerTestDatabasesStaySeparateFromTheHarnessDatabase(t *testing.T) {
	db := cluster.NewDatabase(t)

	if db.Name == cycle.DB.Name {
		t.Fatalf("NewDatabase handed back the harness database %q", db.Name)
	}

	conn := testsupport.Connect(t, db.SuperuserURL)

	var hasAuthSchema bool
	if err := conn.QueryRow(
		t.Context(),
		`SELECT EXISTS (SELECT FROM pg_namespace WHERE nspname = 'auth')`,
	).Scan(&hasAuthSchema); err != nil {
		t.Fatalf("asking the per-test database for the auth schema: %v", err)
	}

	if hasAuthSchema {
		t.Error("a per-test database has an auth schema: the identity provider is no longer " +
			"confined to the harness database, and dropping one of these WITH (FORCE) now " +
			"severs a live connection")
	}
}

func counts(t *testing.T) (profiles, users int) {
	t.Helper()

	conn := testsupport.Connect(t, cycle.DB.SuperuserURL)

	if err := conn.QueryRow(
		t.Context(),
		`SELECT (SELECT count(*) FROM app.profiles), (SELECT count(*) FROM auth.users)`,
	).Scan(&profiles, &users); err != nil {
		t.Fatalf("counting the rows: %v", err)
	}

	return profiles, users
}

// A confirmed account with a password, created the way the product creates nothing — the cycle
// invites instead. It is the cheapest way to get a token out of GoTrue.
func createUser(t *testing.T, email, password string) string {
	t.Helper()

	body := call(t, http.MethodPost, cycle.GoTrue.URL+"/admin/users",
		cycle.AdminToken(t), map[string]any{
			"email":         email,
			"password":      password,
			"email_confirm": true,
		})

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("reading the created user: %v (%s)", err, body)
	}
	if created.ID == "" {
		t.Fatalf("the identity provider created no user: %s", body)
	}

	return created.ID
}

func insertProfile(t *testing.T, userID, role string) {
	t.Helper()

	conn := testsupport.Connect(t, cycle.DB.SuperuserURL)

	if _, err := conn.Exec(
		t.Context(),
		`INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, $2, $3)`,
		userID, role, "Харнесс Тестович",
	); err != nil {
		t.Fatalf("inserting the profile: %v", err)
	}
}

func signIn(t *testing.T, email, password string) jwt.MapClaims {
	t.Helper()

	body := call(t, http.MethodPost, cycle.GoTrue.URL+"/token?grant_type=password", "",
		map[string]any{"email": email, "password": password})

	var granted struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &granted); err != nil {
		t.Fatalf("reading the grant: %v (%s)", err, body)
	}
	if granted.AccessToken == "" {
		t.Fatalf("the identity provider issued no token: %s", body)
	}

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(granted.AccessToken, claims); err != nil {
		t.Fatalf("reading the token: %v", err)
	}

	return claims
}

func call(t *testing.T, method, url, token string, payload map[string]any) []byte {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encoding the request: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("calling %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer from %s: %v", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d: %s", url, resp.StatusCode, body)
	}

	return body
}

// Guards the harness against a lifetime it cannot outlive: the expiry test of step 2 measures a
// link against this value, and GoTrue's own default is a day. Read off the running container for
// the reason ConfiguredWith gives — an assertion on HarnessOTPExpiry alone stays green with the
// line that hands it to GoTrue deleted, and the loss surfaces in step 2 as a test that hangs.
func TestTheHarnessOverridesTheOneTimeTokenLifetime(t *testing.T) {
	if cycle.OTPExpiry <= 0 || cycle.OTPExpiry > time.Hour {
		t.Fatalf("the harness is configured for an OTP_EXP of %s; an expiry test "+
			"cannot wait that out", cycle.OTPExpiry)
	}

	configured := cycle.ConfiguredWith(t, testsupport.OTPExpiryVariable)

	if want := strconv.Itoa(int(cycle.OTPExpiry.Seconds())); configured != want {
		t.Errorf("the container runs with %s=%q, want %q — the override did not reach it, "+
			"and links here live for GoTrue's default of a day",
			testsupport.OTPExpiryVariable, configured, want)
	}
}

// The budget a package has to spend on email, read off the running container for the same reason
// as the lifetime above. Without this the pin has no witness: the quota bites only after a package
// has sent more than the provider allows in an hour, so deleting the line that sets it surfaces in
// another package as over_email_send_rate_limit on a call that had every right to succeed.
func TestTheHarnessGivesThePackageAnEmailBudget(t *testing.T) {
	configured := cycle.ConfiguredWith(t, testsupport.EmailsPerHourVariable)

	if configured != testsupport.HarnessEmailsPerHour {
		t.Errorf("the container runs with %s=%q, want %q — the packages sharing it spend "+
			"whatever the provider defaults to",
			testsupport.EmailsPerHourVariable, configured, testsupport.HarnessEmailsPerHour)
	}
}

// The hook the harness proves is the hook the deployment calls. Without this the tests above would
// go on passing while the deployment called a function at another address, and the harness would be
// evidence about nothing anyone runs. A whole line of compose is matched, because a commented-out
// one still contains the text.
func TestTheHarnessCallsTheHookTheDeploymentDoes(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(testsupport.ModuleRoot(t), "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading the development stack: %v", err)
	}

	want := fmt.Sprintf("%s: %s", testsupport.HookURIVariable,
		cycle.ConfiguredWith(t, testsupport.HookURIVariable))

	given := slices.ContainsFunc(strings.Split(string(compose), "\n"), func(line string) bool {
		return strings.TrimSpace(line) == want
	})

	if !given {
		t.Fatalf("no line of docker-compose.yml is %q — the harness proves a hook the "+
			"deployment does not call", want)
	}
}

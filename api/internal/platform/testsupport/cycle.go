//go:build integration

package testsupport

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// HarnessOTPExpiry is what the harness runs OTPExpiryVariable at, against a GoTrue default of a
// day an expiry test cannot wait out, and long enough for an invite and a handful of local
// statements. The production value is chosen at step 2 and is not this one.
const HarnessOTPExpiry = time.Minute

// HarnessMailerMaxFrequency is what the harness runs MailerMaxFrequencyVariable at, against a
// GoTrue default of a minute a test measuring the gap's end would have to wait out. The production
// value is chosen at step 2 and is not this one.
const HarnessMailerMaxFrequency = 2 * time.Second

// HarnessEmailsPerHour is a budget no package is going to spend: the container is shared by a whole
// binary, and a quota that runs out mid-run fails whichever test is next, for somebody else's reason.
const HarnessEmailsPerHour = "1000"

// HarnessRedirectAllowList is a surface nothing serves and no name resolves: the provider decides
// where a link lands without fetching the address. The deployment's own list is the environment's.
const HarnessRedirectAllowList = "http://cadence.test/**"

// HookURIVariable names the hook GoTrue calls while minting a token.
const HookURIVariable = "GOTRUE_HOOK_CUSTOM_ACCESS_TOKEN_URI"

// accessTokenHookURI is the same string docker-compose.yml sets, and a test reads both rather than
// trusting this sentence: a harness pointing elsewhere would prove the hook works where nobody runs it.
const accessTokenHookURI = "pg-functions://postgres/app/access_token_hook"

// Not optional: migration 000005 grants EXECUTE to AuthHookRole, and GoTrue calls the function over
// its own connection as GoTrueRole.
const grantTheHookMembership = `GRANT ` + AuthHookRole + ` TO ` + GoTrueRole

// emptyBothSchemas is built from the catalogue rather than from a list, so a table added by a later
// migration is emptied without this file being edited. Both migration bookkeeping tables are left
// alone: emptying GoTrue's would tell a restarted container to migrate a schema already there.
const emptyBothSchemas = `DO $$
DECLARE statement text;
BEGIN
    SELECT 'TRUNCATE TABLE '
           || pg_catalog.string_agg(pg_catalog.format('%I.%I', schemaname, tablename), ', ')
           || ' RESTART IDENTITY CASCADE'
      INTO statement
      FROM pg_catalog.pg_tables
     WHERE schemaname IN ('app', 'auth') AND tablename <> 'schema_migrations';

    IF statement IS NOT NULL THEN
        EXECUTE statement;
    END IF;
END $$`

// Cycle is one database with the migration chain applied and one GoTrue connected to that same
// database with the token issuance hook registered — the arrangement a deployment has.
//
// One per package, started from TestMain and shared by the whole binary: the container costs more
// than every test that uses it put together. The per-test databases of NewDatabase never meet
// GoTrue, which is what keeps dropping one WITH (FORCE) from severing a live connection.
//
// Hermeticity is incomplete, by decision rather than by oversight. The `auth` state is shared
// across the package: tests are separated by using addresses of their own, and a test that reuses
// another's address will see the other's user. IDN-17 asks for more — no address collisions and no
// schema residue between tests — and this harness does not satisfy it. Reset narrows the residue to
// nothing between tests that call it; it does not make two tests sharing one GoTrue independent.
//
// Nothing enforces the call, and that is the sharp end of it: a test written without one inherits
// whatever the previous test left, and under -shuffle=on «the previous test» is a different one each
// run. Addresses of one's own are what keeps that from mattering, so a new test picks an address no
// other test uses even when it does call Reset.
type Cycle struct {
	// DB is the harness database: the chain is applied and GoTrue lives in the `auth` schema beside it.
	DB *Database

	GoTrue *GoTrue

	// SigningKey is what GoTrue signs sessions with, so a verifier under test has to be given it.
	SigningKey *SigningKey

	// OTPExpiry is the lifetime the container was configured with, so a test measuring an expired
	// link waits against the value in force rather than a constant that has drifted from it.
	OTPExpiry time.Duration

	// MailerMaxFrequency is the gap the container was configured with, for the same reason.
	MailerMaxFrequency time.Duration

	container testcontainers.Container
}

// StartCycle brings the arrangement up. Call it from TestMain, and Terminate when m.Run returns.
func StartCycle(ctx context.Context, c *Cluster) (*Cycle, error) {
	name := fmt.Sprintf("cadence_cycle_%d", databaseCounter.Add(1))

	if err := c.createDatabase(ctx, name); err != nil {
		return nil, err
	}

	// From here on every failure has a database to drop, and later a container to reap: the suite
	// runs with Ryuk disabled, so nothing else would.
	cycle := &Cycle{OTPExpiry: HarnessOTPExpiry, MailerMaxFrequency: HarnessMailerMaxFrequency}
	abort := func(cause error) (*Cycle, error) {
		return nil, errors.Join(cause, cycle.terminate(ctx, c, name))
	}

	path, err := migrationsPath()
	if err != nil {
		return abort(err)
	}

	db, err := c.migrate(ctx, name, path)
	if err != nil {
		return abort(err)
	}
	cycle.DB = db

	// The chain first: the membership below is to a role migration 000005 creates.
	databaseURL, err := c.prepareForGoTrueContext(ctx, name)
	if err != nil {
		return abort(err)
	}

	if err := c.exec(ctx, db.SuperuserURL, grantTheHookMembership); err != nil {
		return abort(fmt.Errorf("granting the hook membership: %w", err))
	}

	key, err := newES256Key("cycle-session-key")
	if err != nil {
		return abort(err)
	}
	cycle.SigningKey = key

	keys, err := gotrueJWKS(JWKEntry{Key: key, Signing: true})
	if err != nil {
		return abort(err)
	}

	env := gotrueEnv(databaseURL, keys)
	env["GOTRUE_HOOK_CUSTOM_ACCESS_TOKEN_ENABLED"] = "true"
	env[HookURIVariable] = accessTokenHookURI
	env[OTPExpiryVariable] = strconv.Itoa(int(cycle.OTPExpiry.Seconds()))
	env[MailerMaxFrequencyVariable] = cycle.MailerMaxFrequency.String()
	env[EmailsPerHourVariable] = HarnessEmailsPerHour
	env[RedirectAllowListVariable] = HarnessRedirectAllowList

	container, err := runGoTrueContainer(ctx, env,
		wait.ForHTTP("/health").WithPort(gotruePort).WithStartupTimeout(gotrueStartupTimeout))
	cycle.container = container
	if err != nil {
		return abort(err)
	}

	url, err := endpoint(ctx, container)
	if err != nil {
		return abort(err)
	}
	cycle.GoTrue = &GoTrue{URL: url}

	return cycle, nil
}

// Terminate stops the container and drops the harness database.
func (cy *Cycle) Terminate(ctx context.Context) error {
	return cy.terminate(ctx, cy.DB.cluster, cy.DB.Name)
}

// The cluster and the name are parameters because a failure inside StartCycle has a database to
// drop before there is a Cycle holding either.
func (cy *Cycle) terminate(ctx context.Context, c *Cluster, name string) error {
	// The container first: the drop is refused while GoTrue holds a connection, and WITH (FORCE)
	// would sever one mid-statement for no reason.
	var problems error
	if cy.container != nil {
		if err := testcontainers.TerminateContainer(
			cy.container, testcontainers.StopContext(ctx),
		); err != nil {
			problems = fmt.Errorf("terminating the GoTrue container: %w", err)
		}
	}

	return errors.Join(problems, c.dropDatabase(ctx, name))
}

// Reset empties `app` and `auth`, which is what lets one database serve a whole package. It does
// not restart GoTrue: the container holds no state that survives its tables being emptied.
func (cy *Cycle) Reset(t *testing.T) {
	t.Helper()

	if err := cy.DB.cluster.exec(t.Context(), cy.DB.SuperuserURL, emptyBothSchemas); err != nil {
		t.Fatalf("emptying the harness database: %v", err)
	}
}

// ConfiguredWith reads a variable back off the running container: what Docker holds for the
// process, not what this package composed. A test asserting the harness configured something has to
// be able to fail when the line configuring it is deleted, and one against the constant that
// produced that line cannot.
func (cy *Cycle) ConfiguredWith(t *testing.T, variable string) string {
	t.Helper()

	inspected, err := cy.container.Inspect(t.Context())
	if err != nil {
		t.Fatalf("inspecting the GoTrue container: %v", err)
	}

	for _, entry := range inspected.Config.Env {
		if name, value, found := strings.Cut(entry, "="); found && name == variable {
			return value
		}
	}

	return ""
}

// AdminToken mints what GoTrue's admin routes admit: the service role, signed by a key the
// container is configured with. In production that key lives in cmd/provisioner and nowhere else;
// here it is the session key, because GoTrue admits an admin token signed by any of its configured
// keys — the measured behaviour that put the real key in a component of its own.
func (cy *Cycle) AdminToken(t *testing.T) string {
	t.Helper()

	return cy.SigningKey.Sign(t, jwt.MapClaims{
		"role": "service_role",
		"aud":  GoTrueAudience,
		"iss":  GoTrueIssuer,
		"exp":  time.Now().Add(time.Minute).Unix(),
	})
}

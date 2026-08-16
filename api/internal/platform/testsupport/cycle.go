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

// HarnessOTPExpiry is what the harness runs OTPExpiryVariable at, against a
// GoTrue default of a day: an expiry test cannot wait that out, and a link the
// harness itself needs must survive an invite and a handful of local
// statements. The production value is chosen at step 2 and is not this one.
const HarnessOTPExpiry = time.Minute

// HarnessMailerMaxFrequency is what the harness runs MailerMaxFrequencyVariable
// at, against a GoTrue default of a minute: a test measuring that the gap ends
// has to wait it out. The production value is chosen at step 2 and is not this
// one.
const HarnessMailerMaxFrequency = 2 * time.Second

// HarnessEmailsPerHour is a budget no package is going to spend. The container
// is shared by a whole binary, and what its tests may send between them belongs
// in this file rather than in whatever the provider defaults to — a quota that
// runs out mid-run fails the test that happens to be next, for somebody else's
// reason.
const HarnessEmailsPerHour = "1000"

// HarnessRedirectAllowList is a surface nothing serves and no name resolves:
// the provider decides where a link lands without fetching the address, so one
// that existed would only make the measurement slower. The deployment's own
// list is the environment's and is not this.
const HarnessRedirectAllowList = "http://cadence.test/**"

// HookURIVariable names the hook GoTrue calls while minting a token.
const HookURIVariable = "GOTRUE_HOOK_CUSTOM_ACCESS_TOKEN_URI"

// accessTokenHookURI is the same string docker-compose.yml sets, and a test
// reads both rather than trusting this sentence: a harness pointing at a
// function the deployment does not call would prove the hook works somewhere
// nobody runs.
const accessTokenHookURI = "pg-functions://postgres/app/access_token_hook"

// The membership is the deployment's last provisioning step, and it is not
// optional: migration 000005 grants EXECUTE to AuthHookRole, and GoTrue calls
// the function over its own connection as GoTrueRole.
const grantTheHookMembership = `GRANT ` + AuthHookRole + ` TO ` + GoTrueRole

// emptyBothSchemas returns the harness database to the state a package started
// in. It is one statement built from the catalogue rather than a list, so a
// table added by a later migration is emptied without this file being edited.
//
// Both migration bookkeeping tables are left alone: neither migrator re-reads
// its own after it has run, and emptying GoTrue's would tell a restarted
// container to migrate a schema that is already there.
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

// Cycle is one database with the migration chain applied and one GoTrue
// connected to that same database with the token issuance hook registered — the
// arrangement the onboarding tests need, and the one a deployment has.
//
// One per package, started from TestMain and shared by every test in the binary:
// the container costs more than every test that uses it put together, and there
// is nothing per-test about it once Reset can empty both schemas. The per-test
// databases of NewDatabase are untouched by this and never meet GoTrue, which is
// what keeps dropping one WITH (FORCE) from severing a live connection.
//
// Hermeticity is incomplete, by decision rather than by oversight. The `auth`
// state is shared across the package: tests are separated by using addresses of
// their own, and a test that reuses another's address will see the other's user.
// IDN-17 asks for more than that — no address collisions and no schema residue
// between tests — and this harness does not satisfy it. Reset narrows the
// residue to nothing between tests that call it; it does not make two tests
// running against one GoTrue independent, and nothing here should be read as
// claiming otherwise.
type Cycle struct {
	// DB is the harness database: the chain is applied and GoTrue lives in the
	// `auth` schema beside it.
	DB *Database

	// GoTrue is where the test process reaches the identity provider.
	GoTrue *GoTrue

	// SigningKey is the key GoTrue signs sessions with, and therefore the one a
	// verifier under test has to be configured with.
	SigningKey *SigningKey

	// OTPExpiry is the lifetime the container was configured with, so a test
	// measuring an expired link waits against the value in force rather than
	// against a constant that has drifted from it.
	OTPExpiry time.Duration

	// MailerMaxFrequency is the gap the container was configured with, for the
	// same reason.
	MailerMaxFrequency time.Duration

	container testcontainers.Container
}

// StartCycle brings the arrangement up. Call it from TestMain, and Terminate
// when m.Run returns.
func StartCycle(ctx context.Context, c *Cluster) (*Cycle, error) {
	name := fmt.Sprintf("cadence_cycle_%d", databaseCounter.Add(1))

	if err := c.createDatabase(ctx, name); err != nil {
		return nil, err
	}

	// From here on every failure has a database to drop, and after the container
	// starts it has a container to reap as well. The suite runs with Ryuk
	// disabled, so nothing else would.
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

	// The chain first, then the identity provider's own arrangement: the
	// membership below is to a role migration 000005 creates.
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

// The cluster and the name are parameters because a failure inside StartCycle
// has a database to drop before there is a Cycle holding either.
func (cy *Cycle) terminate(ctx context.Context, c *Cluster, name string) error {
	// The container first: dropping the database out from under a running GoTrue
	// is refused while it holds a connection, and WITH (FORCE) would sever one
	// mid-statement for no reason.
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

// Reset empties `app` and `auth`, which is what lets one database serve a whole
// package. It does not restart GoTrue: the container holds no state of its own
// that survives its tables being emptied.
func (cy *Cycle) Reset(t *testing.T) {
	t.Helper()

	if err := cy.DB.cluster.exec(t.Context(), cy.DB.SuperuserURL, emptyBothSchemas); err != nil {
		t.Fatalf("emptying the harness database: %v", err)
	}
}

// ConfiguredWith reads a variable back off the running container.
//
// The value it returns is what Docker holds for the process, not what this
// package composed: a test asserting the harness configured something has to be
// able to fail when the line that configures it is deleted, and an assertion
// against the constant that produced the line cannot.
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

// AdminToken mints what GoTrue's admin routes admit: the service role, signed by
// a key the container is configured with.
//
// In production that key lives in cmd/provisioner and nowhere else. Here it is
// the session key, because the harness is one process and GoTrue admits an admin
// token signed by any of its configured keys — which is the measured behaviour
// that put the real key in a component of its own.
func (cy *Cycle) AdminToken(t *testing.T) string {
	t.Helper()

	return cy.SigningKey.Sign(t, jwt.MapClaims{
		"role": "service_role",
		"aud":  GoTrueAudience,
		"iss":  GoTrueIssuer,
		"exp":  time.Now().Add(time.Minute).Unix(),
	})
}

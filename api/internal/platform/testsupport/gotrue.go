//go:build integration

package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// GoTrueImage is v2.194.0, pinned by digest so an upgrade runs into the contract tests rather
// than past them. docker-compose.yml pins the same digest, and a test asserts they agree.
const GoTrueImage = "supabase/gotrue@sha256:2b352c02adf11a2025cd5993c246ef85db73743553c39194d2b1862d1cc4d1fd"

const (
	// GoTrueAudience is what the harness sets GOTRUE_JWT_AUD to, so a test can send a wrong one.
	GoTrueAudience = "authenticated"

	// GoTrueIssuer is the address inside the container, not the one a test reaches: a claim, not a route.
	GoTrueIssuer = "http://localhost:9999"
)

const gotruePort = "9999/tcp"

// gotrueStartupTimeout covers migrating the auth schema from empty, the slowest thing GoTrue
// does before it serves.
const gotrueStartupTimeout = 90 * time.Second

// GoTrueRole is the database user GoTrue connects as: not the superuser, exactly as in the
// deployment, owning `auth` and nothing else — what the "zero access to auth" walk is written against.
const GoTrueRole = "cadence_gotrue"

// createGoTrueRole converges rather than creates: roles are cluster objects, so a second database
// in one test binary finds this one already there.
const createGoTrueRole = `DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_gotrue') THEN
        CREATE ROLE cadence_gotrue LOGIN PASSWORD 'cadence_gotrue'
            NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
END $$`

// createSupabaseRoles creates the roles GoTrue's migrations hand grants to: a plain Postgres has
// none of them and the migration dies on the first GRANT. They are NOLOGIN and none of them is ours.
const createSupabaseRoles = `DO $$
DECLARE role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY[
        'postgres', 'anon', 'authenticated', 'service_role',
        'supabase_auth_admin', 'supabase_admin', 'authenticator', 'dashboard_user'
    ] LOOP
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN', role_name);
        END IF;
    END LOOP;
END $$`

// createAuthSchema is the one thing GoTrue assumes and does not do: it migrates the schema but
// dies with "schema auth does not exist" if it is missing.
const createAuthSchema = `CREATE SCHEMA auth AUTHORIZATION cadence_gotrue`

type GoTrue struct {
	// URL is where the test process reaches it. Standalone GoTrue serves at the root — the
	// /auth/v1 prefix seen on Supabase comes from their gateway.
	URL string
}

// JWKEntry is one key in GOTRUE_JWT_KEYS; the marker sits on the entry because what the startup
// behaviours vary is how many entries carry it.
type JWKEntry struct {
	Key     *SigningKey
	Signing bool
}

// GoTrueJWKS renders entries as GOTRUE_JWT_KEYS takes them: a bare array of JWKs including their
// private halves.
func GoTrueJWKS(t *testing.T, entries ...JWKEntry) string {
	t.Helper()

	rendered, err := gotrueJWKS(entries...)
	if err != nil {
		t.Fatal(err)
	}

	return rendered
}

// gotrueJWKS is the same rendering reached from TestMain, where there is no *testing.T to fail on.
func gotrueJWKS(entries ...JWKEntry) (string, error) {
	keys := make([]jwkset.JWKMarshal, 0, len(entries))

	for _, entry := range entries {
		marshalled, err := privateJWKMarshalFor(entry.Key, entry.Signing)
		if err != nil {
			return "", err
		}

		keys = append(keys, marshalled)
	}

	raw, err := json.Marshal(keys)
	if err != nil {
		return "", fmt.Errorf("marshalling the key set: %w", err)
	}

	return string(raw), nil
}

// StartGoTrue runs the pinned image against a database of its own.
func StartGoTrue(t *testing.T, cluster *Cluster, keys string) *GoTrue {
	t.Helper()

	return serveGoTrue(t, cluster.newGoTrueDatabase(t), keys)
}

// StartGoTrueOn runs the pinned image against a database the migration chain already owns — one
// database, two schemas, two migrators, as a deployment has. It exists for the assertions needing
// both halves in one catalogue: with GoTrue on a database of its own, "no role of the chain reaches
// auth" is not a question Postgres can be asked.
func StartGoTrueOn(t *testing.T, db *Database, keys string) *GoTrue {
	t.Helper()

	return serveGoTrue(t, db.cluster.prepareForGoTrue(t, db.Name), keys)
}

// StartGoTrueWith runs the pinned image against a database of its own, with settings of the caller's
// on top of the defaults. It exists for the measurements a shared container cannot carry: a quota
// small enough to spend is a quota that fails whichever test runs next.
func StartGoTrueWith(t *testing.T, cluster *Cluster, keys string, settings map[string]string) *GoTrue {
	t.Helper()

	databaseURL := cluster.newGoTrueDatabase(t)

	env := gotrueEnv(databaseURL, keys)
	maps.Copy(env, settings)

	container, err := runGoTrueContainer(t.Context(), env,
		wait.ForHTTP("/health").WithPort(gotruePort).WithStartupTimeout(gotrueStartupTimeout))

	// Registered before the error is checked, for the reason runGoTrue states.
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if err := testcontainers.TerminateContainer(container, testcontainers.StopContext(ctx)); err != nil {
			t.Errorf("terminating the GoTrue container: %v", err)
		}
	})

	if err != nil {
		t.Fatalf("starting GoTrue: %v", err)
	}

	url, err := endpoint(t.Context(), container)
	if err != nil {
		t.Fatal(err)
	}

	return &GoTrue{URL: url}
}

func serveGoTrue(t *testing.T, databaseURL, keys string) *GoTrue {
	t.Helper()

	container := runGoTrue(t, databaseURL, keys,
		wait.ForHTTP("/health").WithPort(gotruePort).WithStartupTimeout(gotrueStartupTimeout))

	url, err := endpoint(t.Context(), container)
	if err != nil {
		t.Fatal(err)
	}

	return &GoTrue{URL: url}
}

// StartGoTrueExpectingRefusal expects the process to die rather than serve and returns what it said
// on the way down. It waits for the exit rather than for the message, which would make the caller's
// assertion tautological.
func StartGoTrueExpectingRefusal(t *testing.T, cluster *Cluster, keys string) string {
	t.Helper()

	container := runGoTrue(t, cluster.newGoTrueDatabase(t), keys,
		wait.ForExit().WithExitTimeout(gotrueStartupTimeout))

	logs, err := container.Logs(t.Context())
	if err != nil {
		t.Fatalf("reading the GoTrue log: %v", err)
	}
	defer func() { _ = logs.Close() }()

	said, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("reading the GoTrue log: %v", err)
	}

	return string(said)
}

func runGoTrue(t *testing.T, databaseURL, keys string, ready wait.Strategy) testcontainers.Container {
	t.Helper()

	container, err := runGoTrueContainer(t.Context(), gotrueEnv(databaseURL, keys), ready)

	// Registered before the error is checked: a wait strategy that times out leaves a started
	// container behind, and the suite runs with Ryuk disabled, so nothing else would reap it.
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if err := testcontainers.TerminateContainer(container, testcontainers.StopContext(ctx)); err != nil {
			t.Errorf("terminating the GoTrue container: %v", err)
		}
	})

	if err != nil {
		t.Fatal(err)
	}

	return container
}

// runGoTrueContainer returns the container alongside the error so the caller can reap one a failed
// wait strategy left behind.
func runGoTrueContainer(
	ctx context.Context, env map[string]string, ready wait.Strategy,
) (testcontainers.Container, error) {
	container, err := testcontainers.Run(
		ctx, GoTrueImage,
		testcontainers.WithEnv(env),
		testcontainers.WithExposedPorts(gotruePort),
		testcontainers.WithWaitStrategy(ready),
		// Docker Desktop provides this name; plain Docker on Linux does not, and a test that puts
		// a server on the host — the SMTP trap the invitation mail is measured through — then has
		// GoTrue answer 500 «Error sending invite email» rather than fail to resolve a name. Added
		// here so the difference between a developer's machine and CI is not one the tests carry.
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			hostConfig.ExtraHosts = append(hostConfig.ExtraHosts, "host.docker.internal:host-gateway")
		}),
	)
	if err != nil {
		return container, fmt.Errorf("starting GoTrue: %w", err)
	}

	return container, nil
}

func endpoint(ctx context.Context, container testcontainers.Container) (string, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("reading the GoTrue host: %w", err)
	}

	port, err := container.MappedPort(ctx, gotruePort)
	if err != nil {
		return "", fmt.Errorf("reading the GoTrue port: %w", err)
	}

	return fmt.Sprintf("http://%s:%s", host, port.Port()), nil
}

func gotrueEnv(databaseURL, keys string) map[string]string {
	return map[string]string{
		"GOTRUE_API_HOST": "0.0.0.0",
		"PORT":            "9999",

		"GOTRUE_DB_DRIVER": "postgres",
		// Both, and they do different jobs: DB_NAMESPACE tells the migrator where to create the
		// tables, search_path tells the running server where to find them — its queries are unqualified.
		"GOTRUE_DB_NAMESPACE": "auth",
		"DATABASE_URL":        databaseURL,

		"API_EXTERNAL_URL":  GoTrueIssuer,
		"GOTRUE_SITE_URL":   GoTrueIssuer,
		"GOTRUE_JWT_ISSUER": GoTrueIssuer,
		"GOTRUE_JWT_AUD":    GoTrueAudience,

		// GoTrue refuses to start without this even when JWT_KEYS is set; it signs nothing here.
		"GOTRUE_JWT_SECRET": "contract-tests-only-not-a-secret",
		"GOTRUE_JWT_KEYS":   keys,

		// Invite-only, as everywhere else: a harness that allowed signup is not the product's.
		DisableSignupVariable: SignupDisabled,
	}
}

func (c *Cluster) newGoTrueDatabase(t *testing.T) string {
	t.Helper()

	name := fmt.Sprintf("cadence_gotrue_%d", databaseCounter.Add(1))

	if err := c.createDatabase(t.Context(), name); err != nil {
		t.Fatal(err)
	}
	c.dropWhenTheTestEnds(t, name)

	return c.prepareForGoTrue(t, name)
}

// prepareForGoTrue gives an existing database the roles and the empty `auth` schema the provider's
// migrator assumes. The DSN names the Postgres container's own IP: the mapped port is on the test
// host, which a container has no name for.
func (c *Cluster) prepareForGoTrue(t *testing.T, name string) string {
	t.Helper()

	dsn, err := c.prepareForGoTrueContext(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}

	return dsn
}

func (c *Cluster) prepareForGoTrueContext(ctx context.Context, name string) (string, error) {
	adminDSN := c.DSN(superuserRole, superuserPass, name)
	for _, statement := range []string{createGoTrueRole, createSupabaseRoles, createAuthSchema} {
		if err := c.exec(ctx, adminDSN, statement); err != nil {
			return "", fmt.Errorf("preparing database %s for GoTrue: %w", name, err)
		}
	}

	ip, err := c.container.ContainerIP(ctx)
	if err != nil {
		return "", fmt.Errorf("reading the Postgres container address: %w", err)
	}

	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable&search_path=auth,public",
		GoTrueRole, GoTrueRole, ip, name), nil
}

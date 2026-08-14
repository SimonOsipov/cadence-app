//go:build integration

package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// GoTrueImage is the identity provider, pinned by digest rather than by tag.
// The trust boundary rests on behaviours GoTrue does not document, so an
// upgrade has to run into the contract tests rather than past them. The tag is
// v2.194.0; docker-compose.yml pins the same digest, and a contract test
// asserts the two have not drifted apart.
const GoTrueImage = "supabase/gotrue@sha256:2b352c02adf11a2025cd5993c246ef85db73743553c39194d2b1862d1cc4d1fd"

const (
	// GoTrueAudience is what the harness configures GOTRUE_JWT_AUD with, so a
	// test can hand GoTrue the audience it expects and then change it.
	GoTrueAudience = "authenticated"

	// GoTrueIssuer is the issuer the harness configures. It is the address
	// inside the container, which is not the one a test reaches — GoTrue serves
	// its API at the root, and the issuer is a claim rather than a route.
	GoTrueIssuer = "http://localhost:9999"
)

const gotruePort = "9999/tcp"

// gotrueStartupTimeout covers pulling nothing and migrating the auth schema
// from empty, which is the slowest thing GoTrue does before it serves.
const gotrueStartupTimeout = 90 * time.Second

// gotrueRole is the database user GoTrue connects as: not the cluster's
// superuser, exactly as in the deployment. It owns the `auth` schema and
// nothing else, which is the arrangement the "zero access to auth" walk is
// written against.
const gotrueRole = "cadence_gotrue"

// createGoTrueRole converges rather than creates: roles are cluster objects, so
// the second database in one test binary finds this one already there.
const createGoTrueRole = `DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_gotrue') THEN
        CREATE ROLE cadence_gotrue LOGIN PASSWORD 'cadence_gotrue'
            NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
END $$`

// createSupabaseRoles creates the roles GoTrue's own migrations hand grants to.
// A plain Postgres has none of them and the migration dies on the first GRANT.
// They are NOLOGIN and hold nothing — they exist so the grants have a target,
// and none of them is one of ours.
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

// createAuthSchema is the one thing GoTrue assumes and does not do: it migrates
// the schema but dies with "schema auth does not exist" if it is missing.
const createAuthSchema = `CREATE SCHEMA auth AUTHORIZATION cadence_gotrue`

// GoTrue is a running identity provider with a database of its own.
type GoTrue struct {
	// URL is where the test process reaches it. Standalone GoTrue serves at the
	// root — the /auth/v1 prefix seen on Supabase comes from their gateway.
	URL string
}

// JWKEntry is one key in GOTRUE_JWT_KEYS and whether it carries the signing
// marker.
//
// The marker is a property of the entry rather than of the key because the two
// startup behaviours are about how many entries carry it, with the keys
// themselves unremarkable.
type JWKEntry struct {
	Key     *SigningKey
	Signing bool
}

// GoTrueJWKS renders entries as the JSON array GOTRUE_JWT_KEYS takes: a bare
// array of JWKs including their private halves, which is how GoTrue is handed
// keys it has to sign with.
func GoTrueJWKS(t *testing.T, entries ...JWKEntry) string {
	t.Helper()

	keys := make([]jwkset.JWKMarshal, 0, len(entries))

	for _, entry := range entries {
		metadata := jwkset.JWKMetadataOptions{
			ALG: jwkset.ALG(entry.Key.Alg),
			KID: entry.Key.KID,
			USE: jwkset.UseSig,
		}
		if entry.Signing {
			metadata.KEYOPS = []jwkset.KEYOPS{jwkset.KeyOpsSign}
		}

		jwk, err := jwkset.NewJWKFromKey(entry.Key.private, jwkset.JWKOptions{
			Marshal:  jwkset.JWKMarshalOptions{Private: true},
			Metadata: metadata,
		})
		if err != nil {
			t.Fatalf("building the JWK for %q: %v", entry.Key.KID, err)
		}

		keys = append(keys, jwk.Marshal())
	}

	raw, err := json.Marshal(keys)
	if err != nil {
		t.Fatalf("marshalling the key set: %v", err)
	}

	return string(raw)
}

// StartGoTrue runs the pinned image against a database of its own and waits
// until it answers on /health.
func StartGoTrue(t *testing.T, cluster *Cluster, keys string) *GoTrue {
	t.Helper()

	container := runGoTrue(t, cluster, keys,
		wait.ForHTTP("/health").WithPort(gotruePort).WithStartupTimeout(gotrueStartupTimeout))

	ctx := t.Context()

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("reading the GoTrue host: %v", err)
	}

	port, err := container.MappedPort(ctx, gotruePort)
	if err != nil {
		t.Fatalf("reading the GoTrue port: %v", err)
	}

	return &GoTrue{URL: fmt.Sprintf("http://%s:%s", host, port.Port())}
}

// StartGoTrueExpectingRefusal runs the pinned image expecting the process to
// die rather than serve, and returns everything it said on the way down.
//
// A wait strategy that looked for the message would make the caller's assertion
// tautological, so this one waits for the exit and hands the log over unread.
func StartGoTrueExpectingRefusal(t *testing.T, cluster *Cluster, keys string) string {
	t.Helper()

	container := runGoTrue(t, cluster, keys,
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

func runGoTrue(t *testing.T, cluster *Cluster, keys string, ready wait.Strategy) testcontainers.Container {
	t.Helper()

	container, err := testcontainers.Run(
		t.Context(), GoTrueImage,
		testcontainers.WithEnv(gotrueEnv(cluster.newGoTrueDatabase(t), keys)),
		testcontainers.WithExposedPorts(gotruePort),
		testcontainers.WithWaitStrategy(ready),
	)

	// Registered before the error is checked: a wait strategy that times out —
	// the likeliest failure here — leaves a started container behind, and the
	// integration suite runs with Ryuk disabled, so nothing else would reap it.
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if err := testcontainers.TerminateContainer(container, testcontainers.StopContext(ctx)); err != nil {
			t.Errorf("terminating the GoTrue container: %v", err)
		}
	})

	if err != nil {
		t.Fatalf("starting GoTrue: %v", err)
	}

	return container
}

func gotrueEnv(databaseURL, keys string) map[string]string {
	return map[string]string{
		"GOTRUE_API_HOST": "0.0.0.0",
		"PORT":            "9999",

		"GOTRUE_DB_DRIVER": "postgres",
		// Both, and they do different jobs: DB_NAMESPACE tells the migrator
		// where to create tables, search_path tells the running server where to
		// find them — its runtime queries are unqualified.
		"GOTRUE_DB_NAMESPACE": "auth",
		"DATABASE_URL":        databaseURL,

		"API_EXTERNAL_URL":  GoTrueIssuer,
		"GOTRUE_SITE_URL":   GoTrueIssuer,
		"GOTRUE_JWT_ISSUER": GoTrueIssuer,
		"GOTRUE_JWT_AUD":    GoTrueAudience,

		// GoTrue refuses to start without this even when JWT_KEYS is set; it
		// signs nothing here.
		"GOTRUE_JWT_SECRET": "contract-tests-only-not-a-secret",
		"GOTRUE_JWT_KEYS":   keys,

		// Invite-only, as everywhere else. No test here creates an account, and
		// a harness that quietly allowed signup would be a harness the product
		// does not have.
		"GOTRUE_DISABLE_SIGNUP": "true",
	}
}

// newGoTrueDatabase creates a database for the identity provider to own, with
// the auth schema and the roles its migrations assume, and returns the
// connection string as seen from inside a sibling container.
//
// The address is the Postgres container's own IP rather than the mapped port:
// the mapped port is on the test host, which a container has no name for.
func (c *Cluster) newGoTrueDatabase(t *testing.T) string {
	t.Helper()

	ctx := t.Context()
	name := fmt.Sprintf("cadence_gotrue_%d", databaseCounter.Add(1))

	bootstrapDSN := c.DSN(bootstrapRole, bootstrapPass, "postgres")
	if err := c.exec(ctx, bootstrapDSN, fmt.Sprintf("CREATE DATABASE %s", pgIdent(name))); err != nil {
		t.Fatalf("creating database %s: %v", name, err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		adminDSN := c.DSN(superuserRole, superuserPass, "postgres")
		if err := c.exec(
			cleanupCtx, adminDSN,
			fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", pgIdent(name)),
		); err != nil {
			t.Errorf("dropping database %s: %v", name, err)
		}
	})

	adminDSN := c.DSN(superuserRole, superuserPass, name)
	for _, statement := range []string{createGoTrueRole, createSupabaseRoles, createAuthSchema} {
		if err := c.exec(ctx, adminDSN, statement); err != nil {
			t.Fatalf("preparing database %s for GoTrue: %v", name, err)
		}
	}

	ip, err := c.container.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("reading the Postgres container address: %v", err)
	}

	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable&search_path=auth,public",
		gotrueRole, gotrueRole, ip, name)
}

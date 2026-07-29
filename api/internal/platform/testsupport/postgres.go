//go:build integration

package testsupport

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// Role names and credentials of the harness. The three cadence_* roles are the
// ones the chain creates; bootstrapRole is what applies it.
const (
	superuserRole = "postgres"
	superuserPass = "postgres"

	// bootstrapRole is deliberately not a superuser. Managed Postgres does not
	// hand out superuser — not on Timeweb, not on Supabase — and the role that
	// applies the chain has CREATEROLE and CREATEDB and nothing beyond. A chain
	// that quietly needs superuser passes here and fails in the one environment
	// that matters.
	bootstrapRole = "cadence_migrator"
	bootstrapPass = "cadence_migrator"

	// AppRole is the request path: the role in DATABASE_URL.
	AppRole = "cadence_app"
	appPass = "cadence_app"

	// OwnerRole owns the application schema and every object in it.
	OwnerRole = "cadence_owner"

	// AuthenticatedRole is the target of impersonation inside a request.
	AuthenticatedRole = "cadence_authenticated"

	// AppSchema is the schema the migration chain owns.
	AppSchema = "app"
)

// Cluster is a Postgres server shared by every test in one test binary.
//
// One container per test binary, not one per run: Go runs each package's tests
// in its own process, so a package-scoped container is as far as sharing goes
// without cross-process container reuse. That is a real cost only once a second
// package needs a database.
type Cluster struct {
	container *postgres.PostgresContainer
	host      string
	port      string
}

// StartCluster starts Postgres and creates the non-superuser bootstrap role the
// migration chain is applied under. Call it from TestMain.
func StartCluster(ctx context.Context) (*Cluster, error) {
	container, err := postgres.Run(
		ctx, "postgres:17-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername(superuserRole),
		postgres.WithPassword(superuserPass),
		postgres.BasicWaitStrategies(),
	)

	// Declared before the error check, not after: postgres.Run returns a non-nil
	// container alongside an error, and a wait-strategy timeout — the likeliest
	// failure — is exactly the case that leaves a started but unhealthy container
	// behind. Ryuk normally reaps a leak, but a harness whose cleanup depends on
	// someone else's leaks wherever Ryuk is turned off, which some CI
	// configurations do. TerminateContainer is nil-safe.
	abort := func(cause error) (*Cluster, error) {
		return nil, errors.Join(cause, testcontainers.TerminateContainer(container))
	}

	if err != nil {
		return abort(fmt.Errorf("starting postgres container: %w", err))
	}

	host, err := container.Host(ctx)
	if err != nil {
		return abort(fmt.Errorf("reading container host: %w", err))
	}

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return abort(fmt.Errorf("reading container port: %w", err))
	}

	cluster := &Cluster{container: container, host: host, port: port.Port()}

	if err := cluster.exec(
		ctx, cluster.DSN(superuserRole, superuserPass, "postgres"),
		fmt.Sprintf(`CREATE ROLE %s LOGIN CREATEROLE CREATEDB NOSUPERUSER NOBYPASSRLS PASSWORD '%s'`,
			bootstrapRole, bootstrapPass),
	); err != nil {
		return abort(fmt.Errorf("creating bootstrap role: %w", err))
	}

	return cluster, nil
}

// Terminate stops the container.
func (c *Cluster) Terminate(ctx context.Context) error {
	if err := testcontainers.TerminateContainer(c.container, testcontainers.StopContext(ctx)); err != nil {
		return fmt.Errorf("terminating postgres container: %w", err)
	}

	return nil
}

// DSN composes a connection string for one role against one database.
func (c *Cluster) DSN(role, password, dbname string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(role), url.QueryEscape(password), c.host, c.port, dbname)
}

// Database is one throwaway database with the migration chain applied.
type Database struct {
	Name string

	// MigrationURL is the bootstrap role — CREATEROLE, CREATEDB, not superuser.
	MigrationURL string

	// AppURL is the request path: cadence_app. This is what DATABASE_URL holds
	// in a deployed environment, and what assertions about the request path's
	// privileges must be made against.
	AppURL string

	// SuperuserURL exists so a test can observe what the request path is not
	// allowed to observe. Nothing under test ever runs through it.
	SuperuserURL string

	cluster *Cluster
}

var databaseCounter atomic.Uint64

// NewDatabase creates a database, applies the whole migration chain under the
// non-superuser bootstrap role, gives cadence_app a password so the test can
// connect as the request path, and registers cleanup.
//
// Tests using it must not call t.Parallel: the chain's roles are cluster
// objects, so a test that rolls the chain back would pull them out from under a
// test running beside it.
func (c *Cluster) NewDatabase(t *testing.T) *Database {
	t.Helper()

	ctx := t.Context()
	name := fmt.Sprintf("cadence_test_%d", databaseCounter.Add(1))

	adminDSN := c.DSN(superuserRole, superuserPass, "postgres")
	bootstrapDSN := c.DSN(bootstrapRole, bootstrapPass, "postgres")

	if err := c.exec(ctx, bootstrapDSN, fmt.Sprintf("CREATE DATABASE %s", pgIdent(name))); err != nil {
		t.Fatalf("creating database %s: %v", name, err)
	}

	t.Cleanup(func() {
		// t.Context is already cancelled by the time cleanup runs.
		cleanupCtx := context.WithoutCancel(ctx)
		if err := c.exec(
			cleanupCtx, adminDSN,
			fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", pgIdent(name)),
		); err != nil {
			t.Errorf("dropping database %s: %v", name, err)
		}
	})

	db := &Database{
		Name:         name,
		MigrationURL: c.DSN(bootstrapRole, bootstrapPass, name),
		AppURL:       c.DSN(AppRole, appPass, name),
		SuperuserURL: c.DSN(superuserRole, superuserPass, name),
		cluster:      c,
	}

	if err := database.RunMigrations(db.MigrationURL, MigrationsPath(t)); err != nil {
		t.Fatalf("applying migration chain: %v", err)
	}

	// The chain creates cadence_app without a password on purpose — the
	// credential is provisioned outside the repository. The harness plays that
	// part so the test can connect as the request path.
	if err := c.exec(
		ctx, bootstrapDSN,
		fmt.Sprintf("ALTER ROLE %s PASSWORD '%s'", pgIdent(AppRole), appPass),
	); err != nil {
		t.Fatalf("setting the app role password: %v", err)
	}

	return db
}

// Connect opens a connection and closes it when the test ends.
func Connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close(context.WithoutCancel(t.Context()))
	})

	return conn
}

func (c *Cluster) exec(ctx context.Context, dsn, statement string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, statement); err != nil {
		return fmt.Errorf("executing %q: %w", statement, err)
	}

	return nil
}

// MigrationsPath locates api/migrations by walking up to the module root, so a
// test can be run from any package without a relative path that breaks when the
// package moves.
func MigrationsPath(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("reading working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "migrations")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}

// pgIdent quotes an identifier for the statements the harness builds. The
// values are the harness's own constants and counters, never test input, but a
// helper that returns a bare string invites the next caller to pass one.
func pgIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

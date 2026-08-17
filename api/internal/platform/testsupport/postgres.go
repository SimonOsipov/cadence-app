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

// Role names and credentials of the harness. The seven cadence_* roles are the
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

	// ServiceAppRole is the service path: the role in DATABASE_SERVICE_URL. It
	// connects on its own pool, and the boundary between the two paths runs
	// along session_user rather than along a single statement.
	ServiceAppRole = "cadence_service_app"
	serviceAppPass = "cadence_service_app"

	// OwnerRole owns the application schema and every object in it.
	OwnerRole = "cadence_owner"

	// The product roles are Postgres roles rather than a claim, because grants
	// are bound to a role: while the admin was a claim, a grant the admin needed
	// reached the patient too.
	PatientRole = "cadence_patient"
	DoctorRole  = "cadence_doctor"
	AdminRole   = "cadence_admin"

	// ServiceRole is the impersonation target of the service path. It is never
	// granted to AppRole, and that is asserted rather than assumed.
	ServiceRole = "cadence_service"

	// AuthHookRole is the intermediary the identity provider's own database role
	// is made a member of. It holds EXECUTE on the token issuance hook and USAGE
	// on the schema, and nothing else anywhere.
	AuthHookRole = "cadence_auth_hook"

	// AppSchema is the schema the migration chain owns.
	AppSchema = "app"
)

// ProductRoles are the three roles a request can be impersonated into.
func ProductRoles() []string {
	return []string{PatientRole, DoctorRole, AdminRole}
}

// ImpersonationRoles are every target of SET LOCAL ROLE: the product roles plus
// the service path. They are the roles that hold table privileges, so they are
// the ones USAGE on the application schema is granted to.
func ImpersonationRoles() []string {
	return []string{PatientRole, DoctorRole, AdminRole, ServiceRole}
}

// BaseRoles are the seven the first migration creates — the arrangement every
// environment stands on. A later migration may create a role of its own, and
// rolling that migration back must not take these with it.
func BaseRoles() []string {
	return []string{
		OwnerRole, AppRole, ServiceAppRole,
		PatientRole, DoctorRole, AdminRole, ServiceRole,
	}
}

// ChainRoles is every role the migration chain creates, in no particular order.
// A rollback has to leave none of them behind.
func ChainRoles() []string {
	return []string{
		OwnerRole, AppRole, ServiceAppRole,
		PatientRole, DoctorRole, AdminRole, ServiceRole, AuthHookRole,
	}
}

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

// MigrationRole is the role that applies the chain: CREATEROLE, CREATEDB, not a
// superuser. Named for the handful of tests that have to grant it something.
func MigrationRole() string {
	return bootstrapRole
}

// AdminDSN is the superuser against the maintenance database. It exists for the
// handful of tests that have to arrange cluster state — a role, say — before
// there is a database to arrange it from. Nothing under test runs through it.
func (c *Cluster) AdminDSN() string {
	return c.DSN(superuserRole, superuserPass, "postgres")
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

	// ServiceAppURL is the service path: cadence_service_app. This is what
	// DATABASE_SERVICE_URL holds, and it is a second connection rather than a
	// second statement — which is what makes SET ROLE between the paths a
	// refusal instead of an option.
	ServiceAppURL string

	// SuperuserURL exists so a test can observe what the request path is not
	// allowed to observe. Nothing under test ever runs through it.
	SuperuserURL string

	cluster *Cluster
}

var databaseCounter atomic.Uint64

// NewEmptyDatabase creates a database with no migrations applied and returns
// the bootstrap connection string.
//
// It exists for tests about the chain itself — what it does to a database it
// has never touched — as opposed to the far more common tests about what the
// chain has already produced, which take NewDatabase.
func (c *Cluster) NewEmptyDatabase(t *testing.T) string {
	t.Helper()

	name := fmt.Sprintf("cadence_empty_%d", databaseCounter.Add(1))

	if err := c.createDatabase(t.Context(), name); err != nil {
		t.Fatal(err)
	}
	c.dropWhenTheTestEnds(t, name)

	return c.DSN(bootstrapRole, bootstrapPass, name)
}

// createDatabase creates one under the bootstrap role, which is the role that
// owns everything the chain will put in it.
func (c *Cluster) createDatabase(ctx context.Context, name string) error {
	bootstrapDSN := c.DSN(bootstrapRole, bootstrapPass, "postgres")
	if err := c.exec(ctx, bootstrapDSN, fmt.Sprintf("CREATE DATABASE %s", pgIdent(name))); err != nil {
		return fmt.Errorf("creating database %s: %w", name, err)
	}

	return nil
}

// dropDatabase runs as the superuser rather than as the creator: WITH (FORCE)
// terminates other backends, which the bootstrap role may not do.
func (c *Cluster) dropDatabase(ctx context.Context, name string) error {
	adminDSN := c.DSN(superuserRole, superuserPass, "postgres")
	if err := c.exec(
		ctx, adminDSN, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", pgIdent(name)),
	); err != nil {
		return fmt.Errorf("dropping database %s: %w", name, err)
	}

	return nil
}

// dropWhenTheTestEnds is registered immediately after the create and before
// anything that can fail, so a database whose migrations died is still dropped.
func (c *Cluster) dropWhenTheTestEnds(t *testing.T, name string) {
	t.Helper()

	t.Cleanup(func() {
		// t.Context is already cancelled by the time cleanup runs.
		if err := c.dropDatabase(context.WithoutCancel(t.Context()), name); err != nil {
			t.Error(err)
		}
	})
}

// NewDatabase creates a database, applies the whole migration chain under the
// non-superuser bootstrap role, gives cadence_app a password so the test can
// connect as the request path, and registers cleanup.
//
// Tests using it must not call t.Parallel: the chain's roles are cluster
// objects, so a test that rolls the chain back would pull them out from under a
// test running beside it.
func (c *Cluster) NewDatabase(t *testing.T) *Database {
	t.Helper()

	name := fmt.Sprintf("cadence_test_%d", databaseCounter.Add(1))

	if err := c.createDatabase(t.Context(), name); err != nil {
		t.Fatal(err)
	}
	c.dropWhenTheTestEnds(t, name)

	db, err := c.migrate(t.Context(), name, MigrationsPath(t))
	if err != nil {
		t.Fatal(err)
	}

	return db
}

// migrate applies the whole chain to an existing database and gives both LOGIN
// roles a password.
//
// The migrations path is a parameter rather than looked up here: finding it
// walks up from the working directory, which needs a *testing.T, and this is
// also reached from TestMain.
func (c *Cluster) migrate(ctx context.Context, name, migrationsPath string) (*Database, error) {
	db := &Database{
		Name:          name,
		MigrationURL:  c.DSN(bootstrapRole, bootstrapPass, name),
		AppURL:        c.DSN(AppRole, appPass, name),
		ServiceAppURL: c.DSN(ServiceAppRole, serviceAppPass, name),
		SuperuserURL:  c.DSN(superuserRole, superuserPass, name),
		cluster:       c,
	}

	if err := database.RunMigrations(db.MigrationURL, migrationsPath); err != nil {
		return nil, fmt.Errorf("applying migration chain: %w", err)
	}

	// The chain creates both LOGIN roles without a password on purpose — the
	// credential is provisioned outside the repository. The harness plays that
	// part so a test can connect as either path.
	bootstrapDSN := c.DSN(bootstrapRole, bootstrapPass, "postgres")
	for role, password := range map[string]string{AppRole: appPass, ServiceAppRole: serviceAppPass} {
		if err := c.exec(
			ctx, bootstrapDSN,
			fmt.Sprintf("ALTER ROLE %s PASSWORD %s", pgIdent(role), pgLiteral(password)),
		); err != nil {
			return nil, fmt.Errorf("setting the %s password: %w", role, err)
		}
	}

	return db, nil
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

	return filepath.Join(ModuleRoot(t), "migrations")
}

// migrationsPath is the same location reached from TestMain, where there is no
// *testing.T to fail on.
func migrationsPath() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "migrations"), nil
}

// ModuleRoot is the directory holding go.mod, found by walking up from the
// package under test. Same reason as MigrationsPath.
func ModuleRoot(t *testing.T) string {
	t.Helper()

	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("reading working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found above the working directory")
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

// pgLiteral quotes a string for the same statements, and exists because ALTER ROLE … PASSWORD takes no bound
// parameter: without it the password is interpolated raw, which is the one form this project's authorship gate refuses
// everywhere else. With standard_conforming_strings on — the server default — doubling the quote is the whole of the
// escaping.
func pgLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}

// Package database owns the Postgres connection pools and the migration
// runner. It knows nothing about the domain: the bounded contexts receive a
// pool and write their own queries.
package database

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// insufficientPrivilege is the only SQLSTATE that means "the database refused
// this on privileges". Every other error means the probe did not find out.
const insufficientPrivilege = "42501"

// The time limits the service path runs under. Both are exported because the
// number is load-bearing outside this package: an outward call made between two
// statements of a service transaction is idle-in-transaction time, so whatever
// bounds that call has to be compared against the limit here rather than against
// a copy of it — and a copy is how two numbers stop being one.
//
// They are a default rather than a barrier: both settings are USERSET, so a
// session may raise its own. Forbidding that needs a role without the SET
// privilege and is probably impossible without a superuser — an open question in
// the spec, written down so the ceiling is not mistaken for a wall.
const (
	// ServiceStatementTimeout bounds one statement on the service path.
	ServiceStatementTimeout = 5 * time.Second

	// ServiceIdleInTransactionTimeout bounds a service transaction that is
	// holding its locks and doing nothing. Ordered against the one above by
	// TestTheServicePathLimitsAreOrdered.
	ServiceIdleInTransactionTimeout = 15 * time.Second
)

// serviceMaxConns is how many connections the service path may hold at once.
//
// Explicit because pgx's default is max(4, NumCPU) — a number nobody chose,
// which grows with the host the process happens to land on. This path makes the
// writes no request may make and its policies carry no row predicate; how many
// of those run at once is a decision, and Postgres's connection budget is shared
// with the request path.
const serviceMaxConns = 4

// serviceRuntimeParams is what the service pool sends in the startup packet:
// the same two limits migration 000007 sets on the role, in milliseconds, which
// is the unit both settings take a bare integer in.
//
// Duplicated on purpose, and neither copy replaces the other. The role setting
// covers every session that logs in as cadence_service_app — a psql session, a
// job somebody ran by hand. The startup packet covers this pool on a database
// the migration never reached, a restored dump among them.
// TestTheMigrationSetsTheNumbersTheServicePoolConnectsUnder is what keeps the
// two spellings one number.
func serviceRuntimeParams() map[string]string {
	return map[string]string{
		"statement_timeout":                   milliseconds(ServiceStatementTimeout),
		"idle_in_transaction_session_timeout": milliseconds(ServiceIdleInTransactionTimeout),
	}
}

func milliseconds(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10)
}

// NewPool opens a connection pool and verifies it can reach the database, so
// that an unreachable Postgres fails the process at startup rather than on the
// first request.
//
// Two pools are opened by the composition root, never one: the request path
// connects as a role that can assume the three product roles and nothing else,
// the service path as a role that can assume cadence_service and nothing else.
// The separation runs along session_user, so a bug on one path cannot reach the
// other's grants — see VerifyPools, which asserts that at startup rather than
// trusting the connection strings.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	return open(ctx, cfg)
}

// NewServicePool opens the service path's pool.
//
// Its own constructor rather than NewPool with another URL, because the two
// paths differ in more than a connection string: this one connects under both
// time limits and holds a number of connections somebody chose. A constructor is
// where that stops depending on whoever writes the composition root next.
func NewServicePool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := serviceConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	return open(ctx, cfg)
}

func serviceConfig(databaseURL string) (*pgxpool.Config, error) {
	cfg, err := poolConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = serviceMaxConns

	// In the startup packet rather than as a SET on every transaction: a
	// statement that publishes the limit is a statement somebody's closure can
	// run without, and the connection is where a limit belongs to the connection.
	maps.Copy(cfg.ConnConfig.RuntimeParams, serviceRuntimeParams())

	return cfg, nil
}

func poolConfig(databaseURL string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	// Pinned rather than left to the default, because the seams depend on it and
	// a default is a thing that changes. Under QueryExecModeSimpleProtocol pgx
	// interpolates arguments into the statement on the client, which would turn
	// "the claims are a bound parameter" — the property the whole impersonation
	// seam rests on — into a string concatenation performed somewhere nobody is
	// looking, and one the authorship gate cannot see either.
	//
	// CacheStatement is what pgx picks by itself today; the value is written
	// down so that the pinning is a decision rather than a coincidence. It is
	// also the one to revisit if a connection pooler in transaction mode ever
	// sits in front of this, which is where prepared statements stop working.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement

	return cfg, nil
}

// open is the half both constructors share: create, then prove the connection
// before handing it back.
func open(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// HealthCheck reports whether the pool can still serve a trivial query. It is
// the probe behind /healthz.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("health check query: %w", err)
	}

	return nil
}

// poolRole is what a connection turned out to be, read back from the server
// rather than parsed out of a connection string. A URL says what somebody
// intended; this says what the database agreed to.
type poolRole struct {
	Name      string
	Superuser bool
	BypassRLS bool
}

// VerifyPools asserts at startup what the whole access model assumes about the
// two connections, and refuses to start when it does not hold.
//
// Three properties, and each one has been the shape of a real failure. A pool
// connected as the wrong role runs every request with somebody else's grants. A
// superuser or a rolbypassrls role makes every policy advisory while every test
// about policies stays green. And a request-path connection able to SET ROLE
// cadence_service holds the service path's grants, which is the one privilege
// boundary in this system that no policy defends — the service-path policies
// carry no row predicate at all.
//
// Checked here rather than left to the deployment: DATABASE_URL and
// DATABASE_SERVICE_URL are two strings in a variable store, and swapping them is
// a plausible mistake with no visible symptom.
func VerifyPools(ctx context.Context, request, service *pgxpool.Pool) error {
	requestRole, err := connectionRole(ctx, request)
	if err != nil {
		return fmt.Errorf("reading the request-path connection: %w", err)
	}

	serviceRoleOf, err := connectionRole(ctx, service)
	if err != nil {
		return fmt.Errorf("reading the service-path connection: %w", err)
	}

	// Ordered, because a map would name whichever of two offending pools came
	// out first, and the message an operator reads at three in the morning
	// should not depend on that.
	for _, checked := range []struct {
		path string
		role poolRole
	}{
		{"request path", requestRole},
		{"service path", serviceRoleOf},
	} {
		path, role := checked.path, checked.role

		if role.Superuser {
			return fmt.Errorf("the %s connects as %s, which is a superuser", path, role.Name)
		}
		if role.BypassRLS {
			return fmt.Errorf("the %s connects as %s, which bypasses row level security", path, role.Name)
		}
	}

	// Pinned against the names the migration chain creates. "They differ" would
	// pass for any two low-privilege roles, including a DATABASE_URL pointing at
	// something nobody wrote a grant for — which fails loudly on the first
	// request rather than at startup, and by then the deploy has gone out.
	for _, expected := range []struct {
		path string
		got  string
		want string
	}{
		{"request path", requestRole.Name, appRole},
		{"service path", serviceRoleOf.Name, serviceAppRole},
	} {
		if expected.got != expected.want {
			return fmt.Errorf("the %s connects as %s, want %s",
				expected.path, expected.got, expected.want)
		}
	}

	// The barrier itself, asserted by trying it, in both directions. Reading
	// pg_auth_members would prove something about a catalogue; this proves what
	// the server does.
	//
	// Both directions, because the separation is claimed to be symmetric: a
	// service path that could assume a product role would read every patient's
	// rows through policies written for somebody else.
	for _, barrier := range []struct {
		from *pgxpool.Pool
		name string
		role string
	}{
		{request, requestRole.Name, serviceRole},
		{service, serviceRoleOf.Name, patientRole},
		{service, serviceRoleOf.Name, doctorRole},
		{service, serviceRoleOf.Name, adminRole},
	} {
		refused, err := roleChangeIsRefused(ctx, barrier.from, barrier.role)
		if err != nil {
			return fmt.Errorf("probing whether %s can assume %s: %w", barrier.name, barrier.role, err)
		}
		if !refused {
			return fmt.Errorf("%s can assume %s, so the two paths are one privilege domain",
				barrier.name, barrier.role)
		}
	}

	return nil
}

// roleChangeIsRefused reports whether the pool's connection role is forbidden to
// assume the named role.
//
// The distinction it draws is the whole value of the check. Any error at all
// would read as "refused" — including the role not existing, which is exactly
// the state of a cluster where the migration chain has not been applied, and
// which would let a process start having measured nothing at all. So only 42501
// counts as a refusal; anything else is reported as a failure to find out.
func roleChangeIsRefused(ctx context.Context, pool *pgxpool.Pool, role string) (bool, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acquiring a connection: %w", err)
	}
	defer conn.Release()

	// Inside a transaction that is always rolled back: set_config with is_local
	// does nothing outside one, and a probe must not be able to leave a role
	// behind on a connection the pool will hand to somebody else.
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("beginning the probe transaction: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	_, err = tx.Exec(ctx, "SELECT set_config('role', $1, true)", role)
	if err == nil {
		return false, nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false, fmt.Errorf("assuming %s: %w", role, err)
	}
	if pgErr.Code != insufficientPrivilege {
		return false, fmt.Errorf(
			"assuming %s gave SQLSTATE %s, which is neither a refusal nor an acceptance: %w",
			role, pgErr.Code, err,
		)
	}

	return true, nil
}

func connectionRole(ctx context.Context, pool *pgxpool.Pool) (poolRole, error) {
	var role poolRole
	if err := pool.QueryRow(ctx, `
		SELECT rolname, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = CURRENT_USER
	`).Scan(&role.Name, &role.Superuser, &role.BypassRLS); err != nil {
		return poolRole{}, fmt.Errorf("reading the current role: %w", err)
	}

	return role, nil
}

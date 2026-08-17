//go:build integration

package database_test

import (
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// GoTrue shares this database and brings its own migrator.
//
// They share a database because a deployment has one, and because the token
// issuance hook resolves app.profiles over GoTrue's own connection — which is
// enough to make the collision below happen. That migrator writes its
// bookkeeping to `public.schema_migrations` with a single `version` column,
// which is the name golang-migrate also claims and a shape it cannot read.
//
// Whoever runs second loses, and the failure is not subtle:
//
//	pq: column "dirty" does not exist in line 0:
//	SELECT version, dirty FROM "public"."schema_migrations"
//
// Found by bringing GoTrue up locally before applying the chain. It would have
// been found again on the deployment, where the order of two containers
// starting decides whether the API can migrate at all.
func TestChainCoexistsWithAnotherMigratorsBookkeeping(t *testing.T) {
	dsn := cluster.NewEmptyDatabase(t)
	conn := testsupport.Connect(t, dsn)

	// Exactly what GoTrue's migrator leaves behind, created before ours runs.
	if _, err := conn.Exec(t.Context(), `
		CREATE TABLE public.schema_migrations (
			version character varying(14) NOT NULL,
			CONSTRAINT schema_migrations_pkey PRIMARY KEY (version)
		)
	`); err != nil {
		t.Fatalf("planting the foreign bookkeeping table: %v", err)
	}

	if err := database.RunMigrations(dsn, testsupport.MigrationsPath(t)); err != nil {
		t.Fatalf("the chain refused to run alongside another migrator: %v", err)
	}

	// The foreign table is still the foreign table: ours went somewhere else
	// rather than adopting, extending or replacing it.
	var columns int
	if err := conn.QueryRow(t.Context(), `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'schema_migrations'
	`).Scan(&columns); err != nil {
		t.Fatalf("inspecting the foreign table: %v", err)
	}
	if columns != 1 {
		t.Errorf("public.schema_migrations now has %d columns, want 1 — the chain "+
			"wrote into another migrator's table instead of keeping its own", columns)
	}

	// And ours records a version, so the isolation did not come from the chain
	// quietly not recording anything.
	var applied int
	if err := conn.QueryRow(
		t.Context(),
		`SELECT count(*) FROM public.cadence_schema_migrations`,
	).Scan(&applied); err != nil {
		t.Fatalf("reading the chain's own bookkeeping: %v", err)
	}
	if applied == 0 {
		t.Error("the chain applied but recorded no version of its own")
	}
}

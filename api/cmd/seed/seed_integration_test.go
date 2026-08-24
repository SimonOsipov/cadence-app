//go:build integration

// The command against a real chain: what it writes is four tables and an audit row per person, all
// of it through policies, and none of that is observable against a fake database.
//
// The provider is a fake here and deliberately: what crossing the trust boundary does is measured in
// cmd/provisioner's own suite and in the client's, and a second implementation of that component
// would be the thing this file ended up measuring.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	started, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)

		return 1
	}
	cluster = started

	defer func() {
		if err := cluster.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
		}
	}()

	return m.Run()
}

// fakeAccounts is the provider as this command uses it: an address gets an identifier the first time
// it is invited and the same one every time after, which is what makes a repeated run meet the
// clinic it already created.
type fakeAccounts struct {
	mu        sync.Mutex
	byAddress map[string]identity.Account
	passwords map[string]string

	// refuse: what the provider answers instead of inviting. A component that is
	// down, or an address it will not take.
	refuse error
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{byAddress: map[string]identity.Account{}, passwords: map[string]string{}}
}

func (a *fakeAccounts) Invite(_ context.Context, email string) (identity.Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.refuse != nil {
		return identity.Account{}, a.refuse
	}

	if found, ok := a.byAddress[email]; ok {
		return found, nil
	}

	account := identity.Account{ID: uuid.NewString()}
	a.byAddress[email] = account

	return account, nil
}

func (a *fakeAccounts) Lookup(_ context.Context, email string) (*identity.Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	found, ok := a.byAddress[email]
	if !ok {
		return nil, nil
	}

	return &found, nil
}

func (a *fakeAccounts) LookupBatch(context.Context, []string) ([]identity.Account, error) {
	return nil, errors.New("the seed does not read the roster")
}

func (a *fakeAccounts) Delete(context.Context, identity.Deletion) error {
	return errors.New("the seed deletes nobody")
}

func (a *fakeAccounts) SetPassword(_ context.Context, id, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.passwords[id] = password

	return nil
}

// seedStand is a clinic with its first administrator and nobody else, and the two pools the creation
// path writes through.
func seedStand(t *testing.T) (deps, *testsupport.Database) {
	t.Helper()

	db := cluster.NewDatabase(t)

	requests, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(requests.Close)

	writes, err := database.NewServicePool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(writes.Close)

	return deps{
		requests:    requests,
		writes:      writes,
		provisioner: newFakeAccounts(),
		password:    "a-seeded-password-nobody-uses",
		// A calendar and not whatever day it is: the course is counted back from
		// this, and a suite reading the clock asserts a different course each day.
		today: theSeededDay,
	}, db
}

// theSeededDay is a Wednesday, chosen so that the course's Sunday alignment is
// something the fixture exercises rather than something it happens to satisfy.
var theSeededDay = civil.NewDate(2026, time.May, 27)

// theFirstAdministrator writes the row bootstrap-admin writes, under the role that command runs as.
// The two statements are not this command's and are arranged rather than exercised here.
func theFirstAdministrator(t *testing.T, db *testsupport.Database) string {
	t.Helper()

	userID := uuid.NewString()

	conn := testsupport.Connect(t, db.MigrationURL)

	if _, err := conn.Exec(t.Context(), `SELECT set_config('role', $1, false)`, "cadence_owner"); err != nil {
		t.Fatalf("assuming the owner role: %v", err)
	}
	if _, err := conn.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'admin', 'Пётр Аверин')
	`, userID); err != nil {
		t.Fatalf("writing the first administrator: %v", err)
	}

	return userID
}

func countOf(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) int {
	t.Helper()

	var count int

	if err := database.WithServiceJob(t.Context(), pool, seedJob, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, statement, args...).Scan(&count)
	}); err != nil {
		t.Fatalf("counting with %q: %v", statement, err)
	}

	return count
}

// Everybody the design was drawn around, in one command: the staff exist as doctors, the patients
// exist with the care teams that make them visible, and each one carries the four rows a patient is.
func TestTheSeedCreatesTheWholeClinic(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	clinic := theClinic()

	if got := countOf(t, on.writes, `SELECT count(*) FROM app.profiles WHERE role = 'doctor'`); got != len(clinic.staff) {
		t.Errorf("the clinic has %d doctors, want %d", got, len(clinic.staff))
	}
	if got := countOf(t, on.writes, `SELECT count(*) FROM app.profiles WHERE role = 'patient'`); got != len(clinic.patients) {
		t.Errorf("the clinic has %d patients, want %d", got, len(clinic.patients))
	}

	// A patient is four rows and an audit record. A profile with no clinical card is a patient whose
	// screens have nothing to draw, and one with no care team is invisible to every doctor.
	for _, table := range []string{"app.patient_profiles", "app.user_preferences"} {
		if got := countOf(t, on.writes, `SELECT count(*) FROM `+table); got != len(clinic.patients) {
			t.Errorf("%s carries %d rows, want one per patient (%d)", table, got, len(clinic.patients))
		}
	}

	if got := countOf(t, on.writes, `SELECT count(*) FROM app.invites`); got != len(clinic.staff)+len(clinic.patients) {
		t.Errorf("app.invites carries %d rows, want one per person invited", got)
	}

	// The persona's care team is the one the chat screen draws, and exactly one of them is primary.
	marina := countOf(t, on.writes, `
		SELECT count(*) FROM app.care_team_assignments a
		JOIN app.profiles p ON p.user_id = a.patient_id
		WHERE p.full_name = 'Марина Волкова'
	`)
	if marina != 3 {
		t.Errorf("Марина Волкова has %d specialists, want 3", marina)
	}
}

// A seed is run against a stand somebody is already using, and a run interrupted halfway is finished
// by running it again. Neither may create a second clinic.
func TestRunningTheSeedTwiceCreatesOneClinic(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("the first run: %v", err)
	}

	before := countOf(t, on.writes, `SELECT count(*) FROM app.profiles`)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("the second run: %v", err)
	}

	if after := countOf(t, on.writes, `SELECT count(*) FROM app.profiles`); after != before {
		t.Errorf("a second run took the clinic from %d people to %d", before, after)
	}
}

// Staff are created by an administrator, and the first administrator is bootstrap-admin's to write.
// Without one this command has nobody to create people as, and says so rather than failing at the
// first invitation with a refusal from the database.
func TestTheSeedRefusesAClinicWithNoAdministrator(t *testing.T) {
	on, _ := seedStand(t)

	err := seed(t.Context(), theClinic(), on)
	if !errors.Is(err, errNoAdministrator) {
		t.Errorf("seeding a clinic with no administrator answered %v, want errNoAdministrator", err)
	}
}

// The password is what a seeded person signs in with, and only those the roster is meant to show as
// accepted are given one: setting it confirms the address, and a confirmed account is one somebody
// has been inside.
func TestOnlyThePeopleWhoArrivedAreGivenAPassword(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	clinic := theClinic()

	want := len(clinic.staff)
	for _, person := range clinic.patients {
		if person.signsIn {
			want++
		}
	}

	provider, _ := on.provisioner.(*fakeAccounts)
	if got := len(provider.passwords); got != want {
		t.Errorf("%d people were given a password, want %d", got, want)
	}
	if want == len(clinic.staff) {
		t.Error("no patient is seeded as having arrived, so the registry shows one state only")
	}
}

// The one refusal a re-run is expected to meet is an address already onboarded here. Every other one
// stops the command: a seed that reads them all as "already here" reports a clinic it did not create.
func TestARefusedInvitationStopsTheSeed(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	provider, _ := on.provisioner.(*fakeAccounts)
	provider.refuse = errors.New("the provider is not answering")

	// identity.ErrProvisionerUnavailable and not the provider's own error: the creation path wraps a
	// refusal it may not read into one sentence, deliberately, and what this measures is that the
	// seed stops rather than reading the refusal as somebody who is already here.
	err := seed(t.Context(), theClinic(), on)
	if !errors.Is(err, identity.ErrProvisionerUnavailable) {
		t.Errorf("a refused invitation answered %v, want ErrProvisionerUnavailable", err)
	}
}

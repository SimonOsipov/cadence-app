// Command seed fills a development clinic with the people the screens were
// designed around: the care team of three, the mobile persona, and the roster
// the dashboard's fixture draws.
//
// One command and not a migration. The identifiers exist only after the identity
// provider has been asked for them, so the accounts and the rows that name them
// cannot be written by a chain applied ahead of time — and a seed skipped in
// production would tear a chain that is strictly sequential.
//
// It creates people the way the API does, through identity.Onboarding: a second
// implementation of onboarding here would be the one thing this command is meant
// to exercise, written twice.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/config"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/provisioning"
)

// errNotInProduction is the refusal the acceptance criterion asks for, and it is
// the outer of two barriers rather than the only one: setting a password is
// mounted outside production only, so a production provisioner answers this
// command's second step 404 whatever SEED_ENVIRONMENT says.
var errNotInProduction = errors.New("this command does not run against production")

// errNoAdministrator: staff are created by an administrator, and the first one
// comes into being through bootstrap-admin under the migration role. Seeding one
// here would be that command's two statements written a second time.
var errNoAdministrator = errors.New("the clinic has no administrator to create staff as")

// errNotWhoWeMeant: an address the provider already holds resolved to a profile
// that is not the person this seed is about, so prescribing would write a course
// onto somebody else's record.
var errNotWhoWeMeant = errors.New("the address is held by somebody else")

// seedJob is who this read is on behalf of: a command has no human to attribute
// it to, and attributing it to the administrator it is about to act as would put
// a person's name on a row nobody asked for.
const seedJob = "seed"

// The addresses are the slug at one domain nobody receives mail at: on a
// development stand no invitation is delivered anywhere, and the seeded people
// sign in with the password this sets rather than with the link.
const addressDomain = "@clinic.example"

// seededZone is where every seeded person lives, and therefore the zone the
// course is counted in. The host's own day is the wrong one: the reads resolve a
// patient's day in their profile's zone, so a seed run at 23:30 UTC on a Saturday
// would count back from that Saturday while Moscow is already Sunday — and the
// stand would open on week five at 0,5 мг instead of week four at 0,25.
const seededZone = "Europe/Moscow"

// seededToday answers the day the course is counted back from: the seeded
// people's own, not the host's.
//
// A function rather than the expression at the call site so that the zone it
// names is something a test can reach — the argument at a call site is the seam
// an extracted function leaves untested.
func seededToday(at time.Time) civil.Date {
	return todayIn(seededZone, at)
}

// todayIn answers the civil day at that instant in the named zone. A zone the
// host cannot load falls back to the instant's own day rather than refusing: the
// seed is a development command, and a stand that will not start over a missing
// tzdata is worse than one whose course is a day out.
func todayIn(zone string, at time.Time) civil.Date {
	if loaded, err := time.LoadLocation(zone); err == nil {
		at = at.In(loaded)
	}

	return civil.NewDate(at.Year(), at.Month(), at.Day())
}

// seededPlace is the same zone as a place, which is what stamping a reading with a local hour
// needs. It carries todayIn's tolerance and lands it in the same fallback: the instant that
// function is given is time.Now(), whose own location is the host's.
func seededPlace() *time.Location {
	loaded, err := time.LoadLocation(seededZone)
	if err != nil {
		return time.Local
	}

	return loaded
}

// seeder is the seam the environment refusal is tested through: everything below
// it needs a database and a provisioner.
type seeder func(ctx context.Context, cfg *config.SeedConfig) error

func main() {
	if err := run(context.Background(), seedTheClinic); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, seed seeder) error {
	cfg, err := config.LoadSeed()
	if err != nil {
		return err
	}

	if cfg.Environment == config.Production {
		return fmt.Errorf("%w: SEED_ENVIRONMENT is %q", errNotInProduction, cfg.Environment)
	}

	return seed(ctx, cfg)
}

func seedTheClinic(ctx context.Context, cfg *config.SeedConfig) error {
	requests, err := database.NewPool(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("opening the request pool: %w", err)
	}
	defer requests.Close()

	writes, err := database.NewServicePool(ctx, cfg.Database.ServiceURL)
	if err != nil {
		return fmt.Errorf("opening the service pool: %w", err)
	}
	defer writes.Close()

	provisioner, err := provisioning.New(provisioning.Config{
		BaseURL: cfg.Provisioner.BaseURL,
		Secret:  cfg.Provisioner.Secret,
	})
	if err != nil {
		return fmt.Errorf("reaching the provisioner: %w", err)
	}

	return seed(ctx, theClinic(), deps{
		requests:    requests,
		writes:      writes,
		provisioner: provisioner,
		password:    cfg.Password,
		today:       seededToday(time.Now()),
	})
}

// accounts is what this command needs of the provisioner: the operations the
// creation path uses, and the one that gives a seeded person a password.
type accounts interface {
	identity.Provisioner

	SetPassword(ctx context.Context, id, password string) error
}

type deps struct {
	requests    *pgxpool.Pool
	writes      *pgxpool.Pool
	provisioner accounts
	password    string

	// today is what the seeded course is counted back from. A parameter and not
	// time.Now inside, so the suite can seed a calendar rather than whatever day
	// it happens to be.
	today civil.Date
}

// seed creates everybody, staff first: a patient names their care team, so the
// team has to exist and be doctors before the first patient is invited.
//
// Re-running is ordinary rather than exceptional. A person the clinic already
// holds is left alone and their identifier is read back from the provider, so a
// run interrupted halfway is finished by running it again — which is what a seed
// against a stand somebody is using has to be.
func seed(ctx context.Context, of clinic, on deps) error {
	admin, err := theAdministrator(ctx, on.writes)
	if err != nil {
		return err
	}

	onboarding := identity.NewOnboarding(on.requests, on.writes, on.provisioner)

	staff := make(map[string]string, len(of.staff))
	for _, member := range of.staff {
		userID, err := createStaff(auth.WithPrincipal(ctx, admin), onboarding, on, member)
		if err != nil {
			return fmt.Errorf("taking on %s: %w", member.fullName, err)
		}

		staff[member.slug] = userID
	}

	courses, cabinets, histories := 0, 0, 0
	for _, person := range of.patients {
		// As the patient's own primary specialist, which is who creates a patient
		// in the product: a doctor may put themselves on a care team and nobody else.
		asDoctor := auth.WithPrincipal(ctx, auth.Principal{Subject: staff[person.careTeam[0]], Role: "doctor"})

		userID, err := createPatient(asDoctor, onboarding, on, of, staff, person)
		if err != nil {
			return fmt.Errorf("creating %s: %w", person.fullName, err)
		}

		if !person.prescribed {
			continue
		}

		// Who the provider handed back is checked against who this is meant to be.
		// On a re-run the identifier comes from the provider by email address, and
		// requireCaresFor cannot discriminate here: every seeded patient names the
		// same primary specialist, so a re-used address would put the persona's
		// twelve-week course onto somebody else's record and pass.
		if err := isWhoWeMeant(ctx, on, userID, person.fullName); err != nil {
			return err
		}

		// By the creating doctor, because Create refuses somebody else's patient.
		written, err := prescribe(asDoctor, on.writes, civil.UserID(userID), on.today)
		if err != nil {
			return fmt.Errorf("prescribing for %s: %w", person.fullName, err)
		}
		if written {
			courses++
		}

		// After the course, because the doses are attributed to its own weekly
		// injection and the vials name the compounds it entered in the directory.
		filled, err := fillTheCabinet(ctx, on.writes, civil.UserID(userID), on.today)
		if err != nil {
			return fmt.Errorf("filling the cabinet of %s: %w", person.fullName, err)
		}
		if filled {
			cabinets++
		}

		// Behind the course for the same reason as the cabinet, and a different one: the
		// cycle window is that course's own geometry, so a patient carrying readings and no
		// course has one of the four windows answering nothing at all.
		measured, err := recordTheReadings(ctx, on.writes, civil.UserID(userID), on.today)
		if err != nil {
			return fmt.Errorf("recording the history of %s: %w", person.fullName, err)
		}
		if measured {
			histories++
		}
	}

	fmt.Printf("seed: %d members of staff, %d patients, %d courses, %d cabinets, %d histories\n",
		len(of.staff), len(of.patients), courses, cabinets, histories)

	return nil
}

// theAdministrator reads the account staff are created as.
func theAdministrator(ctx context.Context, writes *pgxpool.Pool) (auth.Principal, error) {
	var userID string

	err := database.WithServiceJob(ctx, writes, seedJob, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT user_id::text FROM app.profiles WHERE role = 'admin' ORDER BY created_at LIMIT 1
		`).Scan(&userID)
	})

	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, fmt.Errorf("%w: run bootstrap-admin first", errNoAdministrator)
	}

	if err != nil {
		return auth.Principal{}, fmt.Errorf("looking for the administrator: %w", err)
	}

	return auth.Principal{Subject: userID, Role: "admin"}, nil
}

func createStaff(ctx context.Context, onboarding *identity.Onboarding, on deps, member staffMember) (string, error) {
	address := member.slug + addressDomain
	title := member.title

	userID, err := onboarding.InviteProvider(ctx, address, identity.NewProvider{
		Role:     "doctor",
		FullName: member.fullName,
		TitleRU:  &title,
	})
	if err != nil {
		return alreadyHere(ctx, on, address, err)
	}

	return userID, on.provisioner.SetPassword(ctx, userID, on.password)
}

func createPatient(
	ctx context.Context,
	onboarding *identity.Onboarding,
	on deps,
	of clinic,
	staff map[string]string,
	person seededPatient,
) (string, error) {
	address := person.slug + addressDomain

	specialists := make([]identity.Assignment, 0, len(person.careTeam))
	for i, slug := range person.careTeam {
		specialists = append(specialists, identity.Assignment{
			ProviderID: staff[slug],
			CareRole:   careRoleOf(of, slug),
			Primary:    i == 0,
		})
	}

	dob := time.Now().AddDate(-person.age, 0, -1).Format(time.DateOnly)

	newPatient := identity.NewPatient{
		FullName:    person.fullName,
		Timezone:    seededZone,
		Locale:      "ru",
		DateOfBirth: &dob,
		Specialists: specialists,
	}
	if person.body != nil {
		newPatient.Sex = &person.body.sex
		newPatient.HeightCM = &person.body.heightCM
		newPatient.TargetWeightKG = &person.body.targetWeightKG
	}

	userID, err := onboarding.InvitePatient(ctx, address, newPatient)
	if err != nil {
		// Answered rather than discarded: a re-run has to be able to prescribe for
		// somebody it did not create this time round.
		return alreadyHere(ctx, on, address, err)
	}

	// Only for those meant to have arrived: setting a password confirms the
	// address — it has to, or the grant refuses it — and a confirmed account is
	// one the registry draws as accepted. The rest are what pending means.
	if !person.signsIn {
		return userID, nil
	}

	return userID, on.provisioner.SetPassword(ctx, userID, on.password)
}

// alreadyHere turns the one refusal a re-run is expected to meet into the
// identifier of the person who is already here. Every other refusal is returned:
// a seed that swallows them reports a clinic it did not create.
func alreadyHere(ctx context.Context, on deps, address string, refusal error) (string, error) {
	if !errors.Is(refusal, identity.ErrAlreadyOnboarded) {
		return "", refusal
	}

	found, err := on.provisioner.Lookup(ctx, address)
	if err != nil {
		return "", fmt.Errorf("looking up the account already here: %w", err)
	}
	if found == nil {
		return "", fmt.Errorf("%s is onboarded here and unknown to the provider", address)
	}

	return found.ID, nil
}

func careRoleOf(of clinic, slug string) string {
	for _, member := range of.staff {
		if member.slug == slug {
			return member.careRole
		}
	}

	return ""
}

// isWhoWeMeant refuses to prescribe for an account whose profile is not the person
// the seed is holding.
func isWhoWeMeant(ctx context.Context, on deps, userID, fullName string) error {
	var found string

	err := database.WithServiceJob(ctx, on.writes, seedJob, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT full_name FROM app.profiles WHERE user_id = $1`, userID).Scan(&found)
	})
	if err != nil {
		return fmt.Errorf("reading back who %s is: %w", fullName, err)
	}
	if found != fullName {
		return fmt.Errorf("%w: %q resolves to an account belonging to %q", errNotWhoWeMeant, fullName, found)
	}

	return nil
}

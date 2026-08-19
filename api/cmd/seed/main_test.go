package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/config"
)

func seedEnvironment(t *testing.T, environment string) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://app@db/cadence")
	t.Setenv("DATABASE_SERVICE_URL", "postgres://service@db/cadence")
	t.Setenv("PROVISIONER_URL", "http://provisioner:8081")
	t.Setenv("PROVISIONER_SHARED_SECRET", strings.Repeat("s", 32))
	t.Setenv("SEED_PASSWORD", "a-seeded-password-nobody-uses")
	t.Setenv("SEED_ENVIRONMENT", environment)
}

// The refusal is the whole reason this command reads an environment at all. It
// comes before anything is created, because half a seeded clinic in production
// is worse than none.
func TestSeedRefusesToRunAgainstProduction(t *testing.T) {
	seedEnvironment(t, "production")

	asked := false

	err := run(context.Background(), func(context.Context, *config.SeedConfig) error {
		asked = true

		return nil
	})

	if !errors.Is(err, errNotInProduction) {
		t.Errorf("seeding production answered %v, want errNotInProduction", err)
	}
	if asked {
		t.Error("the clinic was seeded anyway")
	}
}

func TestSeedRunsAgainstADevelopmentClinic(t *testing.T) {
	seedEnvironment(t, "development")

	asked := false

	if err := run(context.Background(), func(context.Context, *config.SeedConfig) error {
		asked = true

		return nil
	}); err != nil {
		t.Fatalf("seeding a development clinic: %v", err)
	}

	if !asked {
		t.Error("a development clinic was not seeded")
	}
}

// A literal table of twenty-nine people is a table with a typo in it. What the
// schema and the creation path refuse — a care role outside the closed set, two
// primary specialists, a name nobody has — they refuse one address into the run,
// with the invitations of everybody before it already sent.
func TestTheClinicItSeedsIsCoherent(t *testing.T) {
	clinic := theClinic()

	careRoles := []string{"endo", "dietitian", "nurse"}

	staff := make(map[string]staffMember, len(clinic.staff))
	for _, member := range clinic.staff {
		if !slices.Contains(careRoles, member.careRole) {
			t.Errorf("%s does %q for a patient, which is not one of %v", member.slug, member.careRole, careRoles)
		}
		if member.fullName == "" || member.title == "" {
			t.Errorf("%s is missing a name or a title: %+v", member.slug, member)
		}
		if _, twice := staff[member.slug]; twice {
			t.Errorf("%q names two members of staff", member.slug)
		}

		staff[member.slug] = member
	}

	seen := map[string]bool{}
	for _, person := range clinic.patients {
		if seen[person.slug] {
			t.Errorf("%q names two patients", person.slug)
		}
		seen[person.slug] = true

		if person.fullName == "" || person.age <= 0 {
			t.Errorf("%s is missing a name or an age: %+v", person.slug, person)
		}
		if len(person.careTeam) == 0 {
			t.Errorf("%s has no care team, and the creation path refuses a patient with none", person.slug)
		}

		for _, slug := range person.careTeam {
			if _, ok := staff[slug]; !ok {
				t.Errorf("%s is on %s's care team and is nobody this seeds", slug, person.slug)
			}
		}

		if slices.Contains(person.careTeam[1:], person.careTeam[0]) {
			t.Errorf("%s carries %q twice, which the creation path refuses", person.slug, person.careTeam[0])
		}
	}

	if len(seen) != len(clinic.patients) {
		t.Errorf("the roster carries %d patients under %d slugs", len(clinic.patients), len(seen))
	}
}

// The persona the mobile screens were drawn around, and the one patient whose
// card is filled in: a demographics block half-written is a screen half-drawn.
func TestTheMobilePersonaCarriesEverythingHerScreensRead(t *testing.T) {
	marina := patientNamed(t, "Марина Волкова")

	if marina.body == nil {
		t.Fatal("Марина Волкова has no demographics, and the profile screen reads all four")
	}
	if marina.body.sex == "" || marina.body.heightCM == 0 || marina.body.targetWeightKG == 0 {
		t.Errorf("her card is half-filled: %+v", *marina.body)
	}
	if len(marina.careTeam) != 3 {
		t.Errorf("her care team is %v, want the three the chat screen draws", marina.careTeam)
	}
}

func patientNamed(t *testing.T, fullName string) seededPatient {
	t.Helper()

	for _, person := range theClinic().patients {
		if person.fullName == fullName {
			return person
		}
	}

	t.Fatalf("%s is not among the people this seeds", fullName)

	return seededPatient{}
}

package storage_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

const patient = "5a5f0b0e-3f5a-4c1e-9c0a-2b7d4e8f1a33"

// The rule the two tables state, read from the migrations rather than copied
// here.
//
// Copying it would give this suite a regular expression that agrees with itself
// while the database rejects every key the API mints — which is exactly the
// failure it exists to catch, and it would arrive as a 23514 on a patient's
// first photograph.
func constraintFrom(t *testing.T, migration, constraint string) *regexp.Regexp {
	t.Helper()

	source, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", migration))
	if err != nil {
		t.Fatalf("reading %s: %v", migration, err)
	}

	// The CHECK reads `<column> ~ ('^' || patient_id::text || '<tail>')`, so the
	// tail is what varies and the head is the patient's own id.
	found := regexp.MustCompile(
		`CONSTRAINT ` + constraint + `[\s\S]*?patient_id::text \|\| '([^']*)'`,
	).FindSubmatch(source)
	if found == nil {
		t.Fatalf("%s does not state %s the way this test reads it", migration, constraint)
	}

	return regexp.MustCompile("^" + patient + string(found[1]))
}

// Every key the API mints has to satisfy the constraint the row will be written
// under, for every type it will mint one for.
func TestAMintedKeySatisfiesTheDatabase(t *testing.T) {
	for _, where := range []struct{ migration, constraint string }{
		{"000015_inventory_tables.up.sql", "vials_photo_key_is_under_its_own_prefix"},
		{"000019_dose_events_tables.up.sql", "dose_events_photo_key_is_under_its_own_prefix"},
	} {
		t.Run(where.constraint, func(t *testing.T) {
			allowed := constraintFrom(t, where.migration, where.constraint)

			for _, contentType := range []string{"image/jpeg", "image/png", "image/heic"} {
				key, err := storage.NewKey(patient, contentType)
				if err != nil {
					t.Fatalf("minting a key for %s: %v", contentType, err)
				}
				if !allowed.MatchString(key) {
					t.Errorf("the database would refuse %q, minted for %s", key, contentType)
				}
				if !strings.HasPrefix(key, patient+"/") {
					t.Errorf("%q is not under the patient's own prefix", key)
				}
			}
		})
	}
}

// Two keys minted for one patient are two objects. Without this a second
// photograph would overwrite the first, and the row pointing at it would be
// right about the key and wrong about the picture.
func TestTwoKeysAreTwoObjects(t *testing.T) {
	first, err := storage.NewKey(patient, "image/jpeg")
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	second, err := storage.NewKey(patient, "image/jpeg")
	if err != nil {
		t.Fatalf("minting: %v", err)
	}

	if first == second {
		t.Errorf("both photographs are %q", first)
	}
}

// A type outside the set is refused rather than stored under some default
// extension: the read side decides what to serve from the extension alone, so
// an unknown type stored as .jpg would be served as a photograph.
func TestATypeOutsideTheSetIsRefused(t *testing.T) {
	for _, contentType := range []string{"text/html", "application/pdf", "image/svg+xml", ""} {
		if _, err := storage.NewKey(patient, contentType); err == nil {
			t.Errorf("NewKey minted a key for %q", contentType)
		}
	}
}

// The upload path signs a PUT without opening a transaction, so this refusal is
// the only place the subject's shape is examined at all — database.WithCaller's
// own check never runs there. A prefix carrying a separator is the case that
// matters: normalised by any client, it addresses another patient's prefix.
func TestAPrefixThatIsNotOneIdentifierIsRefused(t *testing.T) {
	for _, prefix := range []string{
		"",
		"not-a-uuid",
		patient + "/x/..",
		"../" + patient,
		patient + "/nested",
		patient + "/",
		"/" + patient,
		patient + "\n" + patient,
	} {
		if key, err := storage.NewKey(prefix, "image/jpeg"); err == nil {
			t.Errorf("NewKey minted %q for the prefix %q", key, prefix)
		}
	}
}

// The round trip that makes the read side's pin correct: what a key was minted
// for is what it is served as.
func TestAKeyIsServedAsWhatItWasMintedFor(t *testing.T) {
	for _, contentType := range []string{"image/jpeg", "image/png", "image/heic"} {
		key, err := storage.NewKey(patient, contentType)
		if err != nil {
			t.Fatalf("minting: %v", err)
		}
		if got := storage.ContentTypeFor(key); got != contentType {
			t.Errorf("a key minted for %s is served as %s", contentType, got)
		}
	}
}

// A key this API did not mint still has to be readable — a seed writes one, and
// so did every row created before this set existed. It is served as something no
// browser renders, which is the safe end of the range rather than a guess.
func TestAnUnknownExtensionIsServedAsBytes(t *testing.T) {
	for _, key := range []string{
		patient + "/label.webp",
		patient + "/label",
		patient + "/label.html",
		patient + "/label.jpg.html",
	} {
		if got := storage.ContentTypeFor(key); got != "application/octet-stream" {
			t.Errorf("%q is served as %s", key, got)
		}
	}
}

// Storage keys come back from a database that does not promise a case, and the
// extension is the whole of what the read side has to go on.
func TestTheExtensionIsReadWithoutRegardToCase(t *testing.T) {
	if got := storage.ContentTypeFor(patient + "/LABEL.JPG"); got != "image/jpeg" {
		t.Errorf("an upper-case extension is served as %s", got)
	}
}

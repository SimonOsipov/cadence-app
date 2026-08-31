package measurements

import (
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The codes are written in three places and nothing reconciled them: here, the CHECK 000025
// carries, and the KMP enum the phone parses with. The schema half alone is not enough — a
// server agreeing with its own table and answering `hips` still loses two metrics of eight,
// because `Metric.fromCode` answers null and the overview stays well-formed.
func TestTheCodesAreTheOnesTheKMPEnumsDeclare(t *testing.T) {
	source := read(t, filepath.Join(
		"..", "..", "..", "kmp", "shared", "src", "commonMain", "kotlin",
		"app", "cadence", "shared", "domain", "Measurements.kt",
	))

	metrics := kotlinEnumCodes(t, source, "Metric")
	if want := codes(Metrics()); !slices.Equal(metrics, want) {
		t.Errorf("Kotlin declares %v, Go %v", metrics, want)
	}

	sources := kotlinEnumCodes(t, source, "MeasurementSource")
	if want := codes(Sources()); !slices.Equal(sources, want) {
		t.Errorf("Kotlin declares %v, Go %v", sources, want)
	}
}

// The two closed sets against the CHECKs that hold them. Read out of the migration rather than
// repeated here, the way inventory reads its own bounds out of 000015.
func TestTheSetsTheSchemaNamesAreTheOnesGoDeclares(t *testing.T) {
	migration := read(t, filepath.Join("..", "..", "migrations", "000025_measurements_tables.up.sql"))

	for _, set := range []struct {
		name    string
		marker  string
		declare []string
	}{
		{"metric", "CHECK (metric IN (", codes(Metrics())},
		{"source", "CHECK (source IN (", codes(Sources())},
	} {
		t.Run(set.name, func(t *testing.T) {
			// Sorted on both sides: the enumeration order is Go's own and means nothing
			// inside a CHECK, so comparing it would pin a thing the schema does not hold.
			schema := slices.Sorted(slices.Values(quoted(t, migration, set.marker)))
			declared := slices.Sorted(slices.Values(set.declare))
			if !slices.Equal(schema, declared) {
				t.Errorf("the schema admits %v, Go declares %v", schema, declared)
			}
		})
	}
}

// The unit is a function of the metric and the schema constrains it as a pair; the constants
// module is the second place that says so, and it is the one the wire reads from. Apart they
// fail the way inventory's lengths did: a server stamping `cm` on a weight writes a row the
// CHECK refuses, as a 23514 naming a constraint rather than the field.
func TestTheUnitsAreTheOnesTheSchemaPairsWithEachMetric(t *testing.T) {
	migration := read(t, filepath.Join("..", "..", "migrations", "000025_measurements_tables.up.sql"))

	constraint := after(t, migration, "CONSTRAINT measurements_unit_belongs_to_its_metric")
	pairs := map[string]string{}
	for _, found := range regexp.MustCompile(`\('([a-z_]+)', '([^']+)'\)`).FindAllStringSubmatch(constraint, -1) {
		pairs[found[1]] = found[2]
	}

	// The pairs the schema names, not their number: a constraint narrowed to one pair and a
	// metric set narrowed with it would agree with each other and with nothing else.
	if got := slices.Sorted(maps.Keys(pairs)); !slices.Equal(got, slices.Sorted(slices.Values(codes(Metrics())))) {
		t.Fatalf("the constraint pairs a unit with %v", got)
	}

	for _, metric := range Metrics() {
		meta, ok := Meta(metric)
		if !ok {
			t.Fatalf("Meta(%q) has no row", metric)
		}
		if meta.Unit != pairs[string(metric)] {
			t.Errorf("%q is measured in %q here and in %q by the schema", metric, meta.Unit, pairs[string(metric)])
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(source)
}

// The entries of one Kotlin enum, in declaration order. Scoped to the enum's own body: the
// file holds two of them, and a regexp over the whole of it would read one set as both.
func kotlinEnumCodes(t *testing.T, source, name string) []string {
	t.Helper()

	body := after(t, source, "enum class "+name+"(")
	// The enum's own closing brace: the companion object's is indented, so the first one in
	// the first column ends the declaration.
	if end := strings.Index(body, "\n}"); end >= 0 {
		body = body[:end]
	}

	var codes []string
	for _, found := range regexp.MustCompile(`(?m)^\s+[A-Z][A-Z_]*\("([^"]+)"\),`).FindAllStringSubmatch(body, -1) {
		codes = append(codes, found[1])
	}
	if len(codes) == 0 {
		t.Fatalf("%s declares no entry this test can read", name)
	}

	return codes
}

// The quoted values of a SQL list, from its opening marker to the parenthesis that closes it.
func quoted(t *testing.T, source, marker string) []string {
	t.Helper()

	list := after(t, source, marker)
	if end := strings.Index(list, ")"); end >= 0 {
		list = list[:end]
	}

	var values []string
	for _, found := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(list, -1) {
		values = append(values, found[1])
	}
	if len(values) == 0 {
		t.Fatalf("%q names no value this test can read", marker)
	}

	return values
}

func after(t *testing.T, source, marker string) string {
	t.Helper()

	at := strings.Index(source, marker)
	if at < 0 {
		t.Fatalf("%q is not in the source this test reads", marker)
	}

	return source[at+len(marker):]
}

func codes[T ~string](set []T) []string {
	out := make([]string, 0, len(set))
	for _, value := range set {
		out = append(out, string(value))
	}

	return out
}

// The note's published bounds against the CHECK that holds them, read out of the migration
// rather than repeated here. Apart they fail in both directions: a contract shorter than the
// column refuses a note the server would take, and one longer sends the patient's own sentence
// to a 23514 they cannot see. The two count alike — huma measures a string in runes
// (validate.go:531, v2.39.0) and pg_catalog.length in characters — which is why the integration
// suite writes a note of two thousand Cyrillic ones.
func TestTheNoteIsPublishedWithTheBoundsTheColumnHolds(t *testing.T) {
	migration := read(t, filepath.Join("..", "..", "migrations", "000025_measurements_tables.up.sql"))

	found := regexp.MustCompile(`pg_catalog\.length\(note\) BETWEEN (\d+) AND (\d+)`).
		FindStringSubmatch(migration)
	if found == nil {
		t.Fatal("000025 no longer bounds the note the way this test reads it")
	}
	low, high := found[1], found[2]

	api := registered(t)
	note := propertyOf(t, requestBody(t, api, operation(t, api, http.MethodPost, recordPath)), "note")
	if note.MinLength == nil || strconv.Itoa(*note.MinLength) != low {
		t.Errorf("the note is published from %v, and the column from %s", note.MinLength, low)
	}
	if note.MaxLength == nil || strconv.Itoa(*note.MaxLength) != high {
		t.Errorf("the note is published to %v, and the column to %s", note.MaxLength, high)
	}
}

//go:build integration

package measurements_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// One request through the registered handler, the way the app would send it. The refusals and
// the rendering are measured without a database in routes_test.go; what needs one is the
// answer built over real rows, and another context shipped two operations that could answer
// nothing but 422 because no test crossed this layer.
func get(t *testing.T, c clinic, subject, path string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: "patient"},
			)))
		})
	})
	measurements.NewService(
		func() time.Time { return theMorning }, measurements.Deps{RequestPool: c.request},
	).Register(httpserver.NewAPI(mux))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec.Code, rec.Body.String()
}

// The wire as a client reads it, and not the domain types the handler rendered from: a field
// renamed in the tag is invisible to an assertion made against the struct.
type wireSpan struct {
	Window string `json:"window"`
	Range  *struct {
		From    string `json:"from"`
		Through string `json:"through"`
	} `json:"range"`
	Timezone string `json:"timezone"`
}

type wireMetric struct {
	Metric string `json:"metric"`
	Meta   struct {
		Unit      string `json:"unit"`
		Direction string `json:"direction"`
		Threshold *struct {
			Value float64 `json:"value"`
		} `json:"threshold"`
	} `json:"meta"`
	Points []struct {
		ID         string  `json:"id"`
		Value      float64 `json:"value"`
		MeasuredAt string  `json:"measured_at"`
	} `json:"points"`
}

type wireOverview struct {
	wireSpan
	Metrics []wireMetric `json:"metrics"`
}

type wireDetail struct {
	wireSpan
	Metric wireMetric `json:"metric"`
	Bands  []struct {
		Dose struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"dose"`
		Range struct {
			From    string `json:"from"`
			Through string `json:"through"`
		} `json:"range"`
	} `json:"bands"`
	Marks []struct {
		Kind string `json:"kind"`
		Date string `json:"date"`
		From *struct {
			Value float64 `json:"value"`
		} `json:"from"`
		To struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"to"`
	} `json:"marks"`
}

func read[T any](t *testing.T, body string) T {
	t.Helper()

	var answered T
	if err := json.Unmarshal([]byte(body), &answered); err != nil {
		t.Fatalf("reading the reply: %v\n%s", err, body)
	}

	return answered
}

func TestTheAppReadsTheOverviewThroughTheTransport(t *testing.T) {
	c := newClinic(t)

	status, body := get(t, c, patientA, "/v1/me/trends?window=7d")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}
	answered := read[wireOverview](t, body)

	if answered.Window != "7d" || answered.Timezone != zoneOfThePatients {
		t.Errorf("the answer is dated %q in %q", answered.Window, answered.Timezone)
	}
	// Counted back from the patient's own day, which is the fifth of August in Yekaterinburg
	// while it is still the fourth in UTC for five hours a day.
	if answered.Range == nil || answered.Range.From != "2026-07-30" || answered.Range.Through != "2026-08-05" {
		t.Errorf("the range is %+v", answered.Range)
	}

	var codes []string
	for _, metric := range answered.Metrics {
		codes = append(codes, metric.Metric)
	}
	// The set, in order, and not its length: an answer that dropped the five unmeasured
	// metrics is exactly as well-formed as one that did not.
	if got := strings.Join(codes, ","); got != "weight,hrv,rhr,sleep,bodyfat,waist,hip,chest" {
		t.Errorf("the overview carries %s", got)
	}

	weight := answered.Metrics[0]
	if weight.Meta.Unit != "kg" || weight.Meta.Direction != "down" {
		t.Errorf("the weight reads %+v", weight.Meta)
	}
	// Absent, and not a zero every reading would clear.
	if weight.Meta.Threshold != nil {
		t.Errorf("the weight publishes a threshold of %v", weight.Meta.Threshold.Value)
	}
	if len(weight.Points) != 1 || weight.Points[0].Value != 82.4 || weight.Points[0].ID != c.manual[patientA] {
		t.Errorf("the weight series is %+v", weight.Points)
	}
	if weight.Points[0].MeasuredAt == "" {
		t.Error("a point travels without the instant it was measured at")
	}

	hrv := answered.Metrics[1]
	if hrv.Meta.Threshold == nil || hrv.Meta.Threshold.Value != 58 {
		t.Errorf("the HRV threshold is %+v", hrv.Meta.Threshold)
	}
	// The positive control for the four assertions above: without it they all hold over
	// eight empty series, which is what a read filtered by nobody's patient would answer.
	if got := len(answered.Metrics[4].Points); got != 0 {
		t.Errorf("the unmeasured bodyfat carries %d points", got)
	}
}

// The bytes and not the parsed shape: a derived value would be a second definition of an
// arithmetic the client already owns, and it would arrive as a field nothing here binds.
func TestNoDerivedValueTravels(t *testing.T) {
	c := newClinic(t)

	for _, path := range []string{"/v1/me/trends?window=7d", "/v1/me/trends/weight?window=7d"} {
		status, body := get(t, c, patientA, path)
		if status != http.StatusOK {
			t.Fatalf("%s answered %d: %s", path, status, body)
		}
		for _, derived := range []string{
			`"base"`, `"latest"`, `"delta"`, `"average"`, `"minimum"`, `"maximum"`, `"movement"`,
		} {
			if strings.Contains(body, derived) {
				t.Errorf("%s answers with %s", path, derived)
			}
		}
	}
}

// The published default, measured rather than read off the contract: huma fills an absent
// parameter from the schema, and the handler refuses an empty window with a 422 — so this is
// the one assertion that says which of the two is happening.
func TestAnAbsentWindowIsAnsweredAsThreeMonths(t *testing.T) {
	c := newClinic(t)

	status, body := get(t, c, patientA, "/v1/me/trends")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}
	answered := read[wireOverview](t, body)
	// Eighty-four days ending on the patient's own day, both edges counted.
	if answered.Window != "3m" || answered.Range == nil ||
		answered.Range.From != "2026-05-14" || answered.Range.Through != "2026-08-05" {
		t.Errorf("an absent window answered %q over %+v", answered.Window, answered.Range)
	}
}

func TestTheAppReadsOneMetricWithItsOverlayThroughTheTransport(t *testing.T) {
	c := newClinic(t)
	prescribed := c.prescribe(t, patientA, "active")

	status, body := get(t, c, patientA, "/v1/me/trends/weight?window=cycle")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}
	answered := read[wireDetail](t, body)

	if answered.Window != "cycle" || answered.Timezone != zoneOfThePatients {
		t.Errorf("the answer is dated %q in %q", answered.Window, answered.Timezone)
	}
	// The course's own geometry, closed at today because the course outlives it.
	if answered.Range == nil || answered.Range.From != theDayTheCourseBegan || answered.Range.Through != "2026-08-05" {
		t.Errorf("the cycle range is %+v", answered.Range)
	}
	if answered.Metric.Metric != "weight" || len(answered.Metric.Points) != 1 ||
		answered.Metric.Points[0].Value != 82.4 {
		t.Errorf("the metric answered %+v", answered.Metric)
	}

	// The strip follows the titrating position, whose two phases are 0,25 then 0,5.
	if len(answered.Bands) != 2 {
		t.Fatalf("the overlay draws %d bands: %s", len(answered.Bands), body)
	}
	if answered.Bands[0].Dose.Value != 0.25 || answered.Bands[0].Dose.Unit != "мг" {
		t.Errorf("the first band is %+v", answered.Bands[0].Dose)
	}
	if answered.Bands[0].Range.From != theDayTheCourseBegan {
		t.Errorf("the first band opens on %q", answered.Bands[0].Range.From)
	}
	if answered.Bands[1].Dose.Value != 0.5 {
		t.Errorf("the second band is %+v", answered.Bands[1].Dose)
	}

	if len(answered.Marks) != 2 {
		t.Fatalf("the overlay draws %d marks: %s", len(answered.Marks), body)
	}
	// Null on the start, because there is no dose to have come up from.
	if answered.Marks[0].Kind != "start" || answered.Marks[0].Date != prescribed.firstMark.String() ||
		answered.Marks[0].From != nil || answered.Marks[0].To.Value != 0.25 {
		t.Errorf("the first mark is %+v", answered.Marks[0])
	}
	if answered.Marks[1].Kind != "titration" || answered.Marks[1].Date != prescribed.secondMark.String() ||
		answered.Marks[1].From == nil || answered.Marks[1].From.Value != 0.25 ||
		answered.Marks[1].To.Value != 0.5 {
		t.Errorf("the second mark is %+v", answered.Marks[1])
	}
}

// The metric in the path and not the first of the set: a detail that answered about weight
// whatever was asked reads exactly like a correct one on the only screen that opens on weight.
func TestTheDetailAnswersAboutTheMetricThatWasAsked(t *testing.T) {
	c := newClinic(t)

	status, body := get(t, c, patientA, "/v1/me/trends/hrv?window=7d")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}
	answered := read[wireDetail](t, body)

	if answered.Metric.Metric != "hrv" || answered.Metric.Meta.Unit != "ms" ||
		answered.Metric.Meta.Direction != "up" {
		t.Errorf("asking for the HRV answered %+v", answered.Metric)
	}
	if answered.Metric.Meta.Threshold == nil || answered.Metric.Meta.Threshold.Value != 58 {
		t.Errorf("the HRV threshold is %+v", answered.Metric.Meta.Threshold)
	}
	// The points are this metric's and not the window's: the weight sits in the same days.
	if len(answered.Metric.Points) != 1 || answered.Metric.Points[0].Value != 61 ||
		answered.Metric.Points[0].ID != c.imported[patientA] {
		t.Errorf("the HRV series is %+v", answered.Metric.Points)
	}
}

// The subject arrives from a token, and a uuid has no case. What it pins is the property and
// not the line above it: measured, dropping the handler's strings.ToLower leaves this green,
// because app.jwt_subject() returns a uuid and every predicate binds the subject against a
// uuid column, so Postgres normalises the case before any policy is read.
func TestTheSubjectIsReadWhateverItsCase(t *testing.T) {
	c := newClinic(t)

	status, body := get(t, c, strings.ToUpper(patientA), "/v1/me/trends?window=7d")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}
	answered := read[wireOverview](t, body)

	if len(answered.Metrics[0].Points) != 1 || answered.Metrics[0].Points[0].ID != c.manual[patientA] {
		t.Errorf("an upper-cased subject is answered %+v", answered.Metrics[0].Points)
	}
}

// A patient between courses, which is the state every screen opens in before the first
// prescription: the range is absent rather than a day-long one, and the metric still answers.
func TestACycleWithNoCourseAnswersANullRangeAndStillAnswers(t *testing.T) {
	c := newClinic(t)

	status, body := get(t, c, patientA, "/v1/me/trends/weight?window=cycle")
	if status != http.StatusOK {
		t.Fatalf("the detail answered %d: %s", status, body)
	}
	detail := read[wireDetail](t, body)
	if detail.Range != nil {
		t.Errorf("a patient with no course is given the range %+v", detail.Range)
	}
	if detail.Metric.Metric != "weight" || detail.Metric.Meta.Unit != "kg" {
		t.Errorf("the metric answered %+v", detail.Metric)
	}
	if len(detail.Bands) != 0 || len(detail.Marks) != 0 {
		t.Errorf("a patient with no course is given %d bands and %d marks", len(detail.Bands), len(detail.Marks))
	}
	// Empty lists and not nulls: two list fields of one answer disagreeing about emptiness
	// is a client branch per field.
	if !strings.Contains(body, `"bands":[]`) || !strings.Contains(body, `"marks":[]`) {
		t.Errorf("the empty overlay is not two empty lists: %s", body)
	}

	status, body = get(t, c, patientA, "/v1/me/trends?window=cycle")
	if status != http.StatusOK {
		t.Fatalf("the overview answered %d: %s", status, body)
	}
	overview := read[wireOverview](t, body)
	if overview.Range != nil {
		t.Errorf("a patient with no course is given the range %+v", overview.Range)
	}
	if len(overview.Metrics) != 8 {
		t.Errorf("an empty window carries %d metrics", len(overview.Metrics))
	}
}

// The boundary the whole surface rests on, asked through the transport rather than of the
// policy: the subject the read filters by comes off the token, and one patient asking for the
// window another patient's readings fall in must not see them.
func TestOnePatientDoesNotReadAnothersThroughTheTransport(t *testing.T) {
	c := newClinic(t)

	status, body := get(t, c, patientB, "/v1/me/trends?window=7d")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}
	answered := read[wireOverview](t, body)

	if len(answered.Metrics[0].Points) != 1 {
		t.Fatalf("the second patient's weight series is %+v", answered.Metrics[0].Points)
	}
	if id := answered.Metrics[0].Points[0].ID; id != c.manual[patientB] {
		t.Errorf("the second patient is answered the reading %s", id)
	}
	if strings.Contains(body, c.manual[patientA]) || strings.Contains(body, c.imported[patientA]) {
		t.Errorf("one patient's answer carries another's reading: %s", body)
	}
}

// A patient whose profile carries no zone: every window edge is a midnight in a day that
// cannot be resolved, so the read refuses rather than cutting the axis in the server's zone.
func TestAPatientWithNoZoneIsARefusalAndNotAnAxisInTheServersZone(t *testing.T) {
	c := newClinic(t)

	if _, err := c.superuser.Exec(t.Context(),
		`UPDATE app.profiles SET timezone = NULL WHERE user_id = $1`, patientA); err != nil {
		t.Fatalf("clearing the zone: %v", err)
	}

	status, body := get(t, c, patientA, "/v1/me/trends?window=7d")
	if status != http.StatusInternalServerError {
		t.Errorf("answered %d, want 500: %s", status, body)
	}
	// The message names the fault and not the patient: the reply reaches a phone.
	if strings.Contains(body, patientA) {
		t.Errorf("the reply carries the patient's identifier: %s", body)
	}
}

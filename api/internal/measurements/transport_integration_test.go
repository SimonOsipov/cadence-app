//go:build integration

package measurements_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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

	return send(t, c, http.MethodGet, subject, path, "")
}

func send(t *testing.T, c clinic, method, subject, path, body string) (int, string) {
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

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

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

// The reading as the add sheet sends it, and the instant is a quarter of an hour before the
// clock the handler was built with: the row's own measured_at is read back below, and a draft
// measured at `now` could not tell a write that stored the recording instant from a right one.
const aTypedReading = `{"metric":"waist","value":93,"measured_at":"2026-08-05T03:45:00Z","note":"утром"}`

type wireRecorded struct {
	ID         string  `json:"id"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	MeasuredAt string  `json:"measured_at"`
	Source     string  `json:"source"`
}

// The happy path, and not only the refusals: an operation nothing sends a valid body to can
// answer 422 to everything and look exactly like a correct one.
func TestThePatientRecordsAReadingThroughTheTransport(t *testing.T) {
	c := newClinic(t)

	status, body := send(t, c, http.MethodPost, patientA, "/v1/me/measurements", aTypedReading)
	if status != http.StatusCreated {
		t.Fatalf("answered %d, want 201: %s", status, body)
	}
	answered := read[wireRecorded](t, body)

	// The unit is the server's: the request carries no field for one, so a reply that
	// echoed the request could not have said `cm` at all.
	if answered.Metric != "waist" || answered.Value != 93 || answered.Unit != "cm" {
		t.Errorf("the reply is %+v", answered)
	}
	// Read back off the row rather than asserted, and the column is one the patient holds
	// no grant on — a hand-typed reading cannot claim to have come off a watch.
	if answered.Source != "manual" {
		t.Errorf("the reading was written as %q", answered.Source)
	}
	if answered.ID == "" {
		t.Fatal("the reply carries no identifier, so the patient cannot ask for it back")
	}
	// The reply's own instant and not the row's: it is where the client draws the point it
	// has just added, and s.now() in its place leaves every assertion below green.
	if answered.MeasuredAt != "2026-08-05T03:45:00Z" {
		t.Errorf("the reply dates the reading %q", answered.MeasuredAt)
	}

	// The row as the privileged role reads it: the reply is what the handler rendered, and
	// a write that landed on nobody's patient renders exactly the same bytes.
	stored := c.row(t, measurements.ReadingID(answered.ID))
	if stored.patient != patientA || stored.metric != "waist" || stored.value != 93 ||
		stored.unit != "cm" || stored.source != "manual" {
		t.Errorf("the row reads %+v", stored)
	}
	if !stored.measuredAt.Equal(time.Date(2026, time.August, 5, 3, 45, 0, 0, time.UTC)) {
		t.Errorf("the row is measured at %v", stored.measuredAt)
	}
	if stored.note == nil || *stored.note != "утром" {
		t.Errorf("the row's note is %v", stored.note)
	}

	// And the point is on the axis the patient will look at, which is the whole reason the
	// write exists: the two halves of this context meet at the row and nowhere else.
	status, body = get(t, c, patientA, "/v1/me/trends/waist?window=7d")
	if status != http.StatusOK {
		t.Fatalf("the detail answered %d: %s", status, body)
	}
	drawn := read[wireDetail](t, body)
	if len(drawn.Metric.Points) != 1 || drawn.Metric.Points[0].ID != answered.ID ||
		drawn.Metric.Points[0].Value != 93 {
		t.Errorf("the waist axis carries %+v", drawn.Metric.Points)
	}
}

// The two bounds count alike: the schema measures the note in runes and the column in
// characters, so a note of two thousand Cyrillic ones — four thousand bytes — is admitted by
// both. A schema counting bytes would refuse it here, and a column counting them would refuse
// it one layer down as a 23514 the patient cannot read.
func TestANoteOfTwoThousandCyrillicCharactersIsAdmitted(t *testing.T) {
	c := newClinic(t)

	note := strings.Repeat("я", 2000)
	status, body := send(t, c, http.MethodPost, patientA, "/v1/me/measurements",
		`{"metric":"weight","value":81.2,"measured_at":"2026-08-05T03:00:00Z","note":"`+note+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("answered %d, want 201: %s", status, body)
	}

	stored := c.row(t, measurements.ReadingID(read[wireRecorded](t, body).ID))
	if stored.note == nil || *stored.note != note {
		t.Errorf("the row kept a note of %d characters", len([]rune(*stored.note)))
	}
}

// The refusals the server makes for itself, through the transport rather than at the service
// level: the schema can hold none of the three — two are comparisons against values it does not
// carry, and the third is the column's own CHECK.
//
// What is NOT asserted here is that nothing was written. WithCaller commits only when the
// closure returns nil, so a count taken after a refused request reads the seeded two however
// late in Record the check sat, and the assertion could not fail. The first two rows have
// their witness in write_integration_test.go, which takes the count INSIDE the transaction;
// the third is the column's own 23514, and the abort it causes is its witness, for the reason
// TestANoteOfNothingIsRefusedByTheTableAndSaidInWords records beside it.
func TestAReadingTheServerCannotBelieveIsRefusedThroughTheTransport(t *testing.T) {
	for _, refused := range []struct{ name, body string }{
		{"measured after it was recorded", `{"metric":"weight","value":81.2,"measured_at":"2026-08-05T04:00:01Z"}`},
		{"a weight with a slipped decimal point", `{"metric":"weight","value":812,"measured_at":"2026-08-05T03:00:00Z"}`},
		{"a note of nothing but whitespace", `{"metric":"weight","value":81.2,"measured_at":"2026-08-05T03:00:00Z","note":"   "}`},
	} {
		t.Run(refused.name, func(t *testing.T) {
			c := newClinic(t)

			status, body := send(t, c, http.MethodPost, patientA, "/v1/me/measurements", refused.body)
			if status != http.StatusUnprocessableEntity {
				t.Errorf("answered %d, want 422: %s", status, body)
			}
		})
	}
}

// The reading a patient typed by mistake, removed the way the screen removes it. A 204 and a
// re-read as the privileged role: a DELETE the policy filtered away answers success too.
func TestThePatientRemovesTheirOwnReadingThroughTheTransport(t *testing.T) {
	c := newClinic(t)

	mine := c.manual[patientA]
	status, body := send(t, c, http.MethodDelete, patientA, "/v1/me/measurements/"+mine, "")
	if status != http.StatusNoContent {
		t.Fatalf("answered %d, want 204: %s", status, body)
	}
	if body != "" {
		t.Errorf("a 204 carries %q", body)
	}
	if c.survives(t, mine) {
		t.Error("the row is still there")
	}
	// The neighbour's row of the same shape, so the delete was bounded and not merely
	// successful.
	if !c.survives(t, c.manual[patientB]) {
		t.Error("the other patient's row went with it")
	}
}

// The two absences answer alike, which is the whole of the design: an answer that told them
// apart would report another patient's history one reading at a time. Compared to each other
// rather than each to a constant — a status and a sentence that both drifted together would
// satisfy two separate assertions and still be two distinguishable answers.
//
// The whole reply less its `instance`, which is the request line and carries whatever
// identifier the caller themselves sent.
func TestAReadingTheyDoNotHoldAndOneNobodyHoldsAnswerAlike(t *testing.T) {
	c := newClinic(t)

	said := map[string]string{}
	for _, absent := range []struct{ name, id string }{
		{"another patient's reading", c.manual[patientB]},
		// The row whose kind the 409 branch would report if the source were read before
		// the ownership: invisible, so it is an absence like any other.
		{"another patient's imported reading", c.imported[patientB]},
		{"an identifier nobody holds", "7c4d1a90-0000-4000-8000-00000000dead"},
	} {
		t.Run(absent.name, func(t *testing.T) {
			status, body := send(t, c, http.MethodDelete, patientA, "/v1/me/measurements/"+absent.id, "")
			if status != http.StatusNotFound {
				t.Errorf("answered %d, want 404: %s", status, body)
			}

			problem := read[map[string]any](t, body)
			delete(problem, "instance")
			if detail, _ := problem["detail"].(string); strings.Contains(detail, absent.id) {
				t.Errorf("the refusal names the reading: %s", detail)
			}
			said[absent.name] = fmt.Sprintf("%d %v", status, problem)
		})
	}

	var answers []string
	for _, answer := range said {
		answers = append(answers, answer)
	}
	if len(answers) != 3 || answers[0] != answers[1] || answers[1] != answers[2] {
		t.Errorf("the three absences answer %v", said)
	}
	if !c.survives(t, c.manual[patientB]) || !c.survives(t, c.imported[patientB]) {
		t.Error("the other patient's rows were deleted")
	}
}

// Their own imported reading is the one refusal that is not an absence: the row IS on their
// screen, so 404 would be a lie, and the sample returns on the next sync anyway. Told apart
// from the case above by a read before the delete — the statement answers zero rows and nil
// for both.
func TestTheirOwnImportedReadingAnswersAConflictAndSaysWhy(t *testing.T) {
	c := newClinic(t)

	imported := c.imported[patientA]
	status, body := send(t, c, http.MethodDelete, patientA, "/v1/me/measurements/"+imported, "")
	if status != http.StatusConflict {
		t.Fatalf("answered %d, want 409: %s", status, body)
	}
	if !strings.Contains(body, "imported") {
		t.Errorf("the refusal does not say why: %s", body)
	}
	if !c.survives(t, imported) {
		t.Error("the imported row was deleted")
	}
}

// No unique key is reachable from a hand-typed reading, and this is what says so: besides the
// primary key, whose value the row is given, the table's only unique index is partial on
// `external_id`, which this path never sets and the patient holds no grant on. Were it reachable it would answer before RLS — the shape 000019
// records on the dose stream, and the reason the index leads with the patient rather than
// relying on the order.
func TestTwoIdenticalReadingsBothLandAndCrossNoTenant(t *testing.T) {
	c := newClinic(t)

	var written []string
	for _, patient := range []string{patientA, patientA, patientB} {
		status, body := send(t, c, http.MethodPost, patient, "/v1/me/measurements", aTypedReading)
		if status != http.StatusCreated {
			t.Fatalf("%s answered %d, want 201: %s", patient, status, body)
		}
		written = append(written, read[wireRecorded](t, body).ID)
	}

	// Every pair, and not the neighbouring ones: a check that never compared the first
	// against the third is satisfied by two writes that answered the same row.
	distinct := slices.Compact(slices.Sorted(slices.Values(written)))
	if len(written) != 3 || len(distinct) != 3 {
		t.Errorf("three writes produced the identifiers %v", written)
	}
	if held := c.held(t, patientA); held != seededReadings+2 {
		t.Errorf("the patient holds %d readings, not the %d they wrote", held, seededReadings+2)
	}
}

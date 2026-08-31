package measurements

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

const (
	overviewPath = "/v1/me/trends"
	detailPath   = "/v1/me/trends/{metric}"
	recordPath   = "/v1/me/measurements"
	removePath   = "/v1/me/measurements/{id}"
)

// One reading as the add sheet sends it, valid in every field, so that a subtest changing one
// of them changes only that one.
const aReading = `{"metric":"weight","value":81.2,"measured_at":"2026-08-05T03:00:00Z"}`

// registered is this context's operations alone on a document of their own, so that what the
// assertions below walk is what this package published and not what eleven contexts did.
func registered(t *testing.T) huma.API {
	t.Helper()

	api := httpserver.NewAPI(chi.NewRouter())
	NewService(func() time.Time { return time.Unix(0, 0).UTC() }, Deps{}).Register(api)

	return api
}

func operation(t *testing.T, api huma.API, method, path string) *huma.Operation {
	t.Helper()

	item, ok := api.OpenAPI().Paths[path]
	if !ok || item == nil {
		t.Fatalf("%s is not served at all", path)
	}

	var op *huma.Operation
	switch method {
	case http.MethodGet:
		op = item.Get
	case http.MethodPost:
		op = item.Post
	case http.MethodDelete:
		op = item.Delete
	}
	if op == nil {
		t.Fatalf("%s serves no %s", path, method)
	}

	return op
}

// The body schema as the document carries it, followed through the $ref huma registers it
// behind: the generated clients read the component and not the inline reference.
func requestBody(t *testing.T, api huma.API, op *huma.Operation) *huma.Schema {
	t.Helper()

	if op.RequestBody == nil {
		t.Fatalf("%s takes no body", op.OperationID)
	}
	media, ok := op.RequestBody.Content["application/json"]
	if !ok || media.Schema == nil {
		t.Fatalf("%s takes no JSON body", op.OperationID)
	}
	schema := media.Schema
	if schema.Ref != "" {
		schema = api.OpenAPI().Components.Schemas.SchemaFromRef(schema.Ref)
	}
	if schema == nil {
		t.Fatalf("%s's body resolves to no schema", op.OperationID)
	}

	return schema
}

func propertyOf(t *testing.T, schema *huma.Schema, name string) *huma.Schema {
	t.Helper()

	found, ok := schema.Properties[name]
	if !ok {
		t.Fatalf("the schema has no %s", name)
	}

	return found
}

func parameter(t *testing.T, op *huma.Operation, name string) *huma.Param {
	t.Helper()

	for _, param := range op.Parameters {
		if param.Name == name {
			return param
		}
	}
	t.Fatalf("%s takes no %s", op.OperationID, name)

	return nil
}

func published(t *testing.T, schema *huma.Schema) []string {
	t.Helper()

	if schema == nil {
		t.Fatal("the parameter carries no schema")
	}

	var offered []string
	for _, value := range schema.Enum {
		code, ok := value.(string)
		if !ok {
			t.Fatalf("the schema offers %v, which is not a string", value)
		}
		offered = append(offered, code)
	}

	return offered
}

// The published set against the parsed one, in both directions. A struct tag cannot call
// Windows(), so the codes are written out twice — and a code the schema offers that the
// parser refuses is a 422 the client cannot avoid, while one the parser knows and the schema
// does not is a window nobody can ask for.
func TestTheSchemaOffersTheWindowsTheServerParses(t *testing.T) {
	api := registered(t)

	var want []string
	for _, window := range Windows() {
		want = append(want, string(window))
	}

	for _, path := range []string{overviewPath, detailPath} {
		offered := published(t, parameter(t, operation(t, api, http.MethodGet, path), "window").Schema)
		if !slices.Equal(offered, want) {
			t.Errorf("%s offers the windows %v, and the server parses %v", path, offered, want)
		}
	}
}

// Both reads take every metric, the unwritable one included: sleep is scored from imported
// sessions and cannot be typed in, and a screen still has to be able to draw it.
func TestTheSchemaOffersTheMetricsTheServerParses(t *testing.T) {
	api := registered(t)

	var want []string
	for _, metric := range Metrics() {
		want = append(want, string(metric))
	}

	offered := published(t, parameter(t, operation(t, api, http.MethodGet, detailPath), "metric").Schema)
	if !slices.Equal(offered, want) {
		t.Errorf("the path offers the metrics %v, and the server parses %v", offered, want)
	}
}

// Published rather than applied in the handler, so a client reading the contract is told what
// an absent window means — and it is three months, which is where the patient app opens.
func TestAnAbsentWindowIsTheOneThePatientAppOpensOn(t *testing.T) {
	api := registered(t)

	for _, path := range []string{overviewPath, detailPath} {
		schema := parameter(t, operation(t, api, http.MethodGet, path), "window").Schema
		if schema == nil || schema.Default != string(WindowThreeMonths) {
			t.Errorf("%s defaults the window to %v, want %q", path, schema.Default, WindowThreeMonths)
		}
	}
}

// A $ref that may be null is a shape huma cannot spell, and without the rewrite a generator
// types the property as the object — so a client throws on the first patient with no course,
// or on the first of the five metrics that carry no bound.
func TestEveryOptionalObjectAdmitsNull(t *testing.T) {
	schemas := registered(t).OpenAPI().Components.Schemas

	for _, nullable := range []struct{ schema, property string }{
		{"TrendsBody", "range"},
		{"TrendBody", "range"},
		{"MetricMetaBody", "threshold"},
		{"ProtocolMarkBody", "from"},
	} {
		target := schemas.SchemaFromRef("#/components/schemas/" + nullable.schema)
		if target == nil {
			t.Errorf("no schema named %s", nullable.schema)

			continue
		}
		property, ok := target.Properties[nullable.property]
		if !ok {
			t.Errorf("%s has no %s", nullable.schema, nullable.property)

			continue
		}
		if len(property.OneOf) != 2 || property.OneOf[0].Ref == "" || property.OneOf[1].Type != "null" {
			t.Errorf("%s.%s is %+v, not «the object or null»", nullable.schema, nullable.property, property)
		}
	}
}

// The client computes these from the points, and a second definition of them here would be a
// second truth free to disagree with the first. Asserted against the published document
// rather than against the Go types, because it is the document two client surfaces are
// generated from.
func TestNoDerivedSeriesValueIsPublished(t *testing.T) {
	derived := []string{"base", "latest", "delta", "average", "minimum", "maximum", "movement"}

	schemas := registered(t).OpenAPI().Components.Schemas.Map()
	if len(schemas) == 0 {
		t.Fatal("the document carries no schemas, so this walked nothing")
	}

	for name, schema := range schemas {
		for property := range schema.Properties {
			if slices.Contains(derived, property) {
				t.Errorf("%s publishes %s, which the client derives", name, property)
			}
		}
	}
}

// Every refusal a read can produce, and the status it becomes. Named one by one rather than
// left to a default: a 500 for a refusal the caller could act on hides it, and a 422 for a
// fault of this process tells a patient their request is wrong about a bug here.
func TestEachRefusalBecomesItsOwnStatus(t *testing.T) {
	for _, mapped := range []struct {
		err  error
		want int
	}{
		{ErrNoTimezone, http.StatusInternalServerError},
		// The one the operation publishes and the only arm that tells a client to
		// retry; without it the fall-through answers 500 and the retry never happens.
		{fmt.Errorf("querying: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
		{fmt.Errorf("wrapped: %w", ErrNoTimezone), http.StatusInternalServerError},
		// A read reaching it means Meta and ParseMetric disagree about the eight, which
		// is this process being wrong and not the request.
		{ErrUnknownMetric, http.StatusInternalServerError},
		{errors.New("something nobody named"), http.StatusInternalServerError},
	} {
		var status huma.StatusError
		if !errors.As(answer("under test", mapped.err), &status) {
			t.Errorf("%v did not become a status", mapped.err)

			continue
		}
		if status.GetStatus() != mapped.want {
			t.Errorf("%v became %d, want %d", mapped.err, status.GetStatus(), mapped.want)
		}
	}
}

// The refusals that happen before a connection is asked for, through the transport that is
// actually registered. A pool is handed over and never dialled: pgxpool connects on demand,
// and every request below is refused above the database.
func asked(t *testing.T, path, subject, role string) (int, string) {
	t.Helper()

	return sent(t, http.MethodGet, path, "", subject, role)
}

func sent(t *testing.T, method, path, body, subject, role string) (int, string) {
	t.Helper()

	pool, err := pgxpool.New(t.Context(), "postgres://nobody:nothing@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("building a pool: %v", err)
	}
	t.Cleanup(pool.Close)

	mux := chi.NewRouter()
	if role != "" {
		mux.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
					r.Context(), auth.Principal{Subject: subject, Role: role},
				)))
			})
		})
	}
	NewService(func() time.Time { return time.Unix(0, 0).UTC() },
		Deps{RequestPool: pool}).Register(httpserver.NewAPI(mux))

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec.Code, rec.Body.String()
}

func TestOnlyAPatientReadsTheirOwnTrends(t *testing.T) {
	for _, refused := range []struct {
		name   string
		path   string
		role   string
		status int
	}{
		{"the overview with no principal", overviewPath, "", http.StatusUnauthorized},
		{"the detail with no principal", "/v1/me/trends/weight", "", http.StatusUnauthorized},
		{"the overview as a doctor", overviewPath, "doctor", http.StatusForbidden},
		{"the detail as a doctor", "/v1/me/trends/weight", "doctor", http.StatusForbidden},
		// An admin's policies on this table are USING (true), so the subject the read
		// filters by would be the only boundary left — and it comes off the token.
		{"the overview as an admin", overviewPath, "admin", http.StatusForbidden},
		{"the detail as an admin", "/v1/me/trends/weight", "admin", http.StatusForbidden},
	} {
		t.Run(refused.name, func(t *testing.T) {
			status, body := asked(t, refused.path, "8a1f3b7c-0000-4000-8000-000000000001", refused.role)
			if status != refused.status {
				t.Errorf("answered %d, want %d: %s", status, refused.status, body)
			}
		})
	}
}

// The schema refuses before the handler does, which is what makes the parser below the second
// guard rather than the first — and both are measured, because a keyword dropped from a field
// leaves the parser holding the set on its own.
func TestAWindowAndAMetricOffTheSetAreRefusedByTheSchema(t *testing.T) {
	for _, refused := range []struct{ name, path string }{
		{"a window nobody offers", overviewPath + "?window=1y"},
		{"a metric the prototype has and §03 does not", "/v1/me/trends/thigh"},
		{"the plural spelling the wire does not carry", "/v1/me/trends/hips"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			status, body := asked(t, refused.path, "8a1f3b7c-0000-4000-8000-000000000001", "patient")
			if status != http.StatusUnprocessableEntity {
				t.Errorf("answered %d, want 422: %s", status, body)
			}
		})
	}
}

func TestTheParserRefusesWhatTheSchemaWouldHaveCaught(t *testing.T) {
	for _, off := range []string{"", "1y", "7 d", string(WindowWeek) + " "} {
		var status huma.StatusError
		if _, err := askedWindow(off); !errors.As(err, &status) ||
			status.GetStatus() != http.StatusUnprocessableEntity {
			t.Errorf("the window %q was not refused with a 422: %v", off, err)
		}
	}
	for _, window := range Windows() {
		if parsed, err := askedWindow(string(window)); err != nil || parsed != window {
			t.Errorf("the window %q was refused: %v", window, err)
		}
	}
}

// The write's parser, below a keyword the transport cannot be asked to drop. It holds the
// eight and not the seven: the sleep score is refused one layer down, by Record, and a copy of
// that set here would make whichever ran second unmeasurable.
func TestTheMetricParserRefusesWhatTheSchemaWouldHaveCaught(t *testing.T) {
	for _, off := range []string{"", "WEIGHT", "body_fat", "hips", "thigh", "weight "} {
		var status huma.StatusError
		if _, err := askedMetric(off); !errors.As(err, &status) ||
			status.GetStatus() != http.StatusUnprocessableEntity {
			t.Errorf("the metric %q was not refused with a 422: %v", off, err)
		}
	}
	for _, metric := range Metrics() {
		if parsed, err := askedMetric(string(metric)); err != nil || parsed != metric {
			t.Errorf("the metric %q was refused: %v", metric, err)
		}
	}
}

// The parser at each place it is called, which is a seam of its own: the test above measures
// the function, and a handler that stopped calling it would leave that test green. Reached by
// calling the handlers rather than by sending requests, because both published enums are
// subsets of what ParseMetric admits — over HTTP the keyword refuses first and nothing below it
// can be observed. The refusal is a 422 before a connection is asked for; without the guard the
// string reaches the query and the pool that goes nowhere answers 503.
func TestBothHandlersRefuseAMetricOffTheSetBeforeAskingForAConnection(t *testing.T) {
	off := "thigh"

	written := &RecordInput{}
	written.Body.Metric = off
	written.Body.Value = 81.2
	written.Body.MeasuredAt = time.Unix(0, 0).UTC()

	for name, ask := range map[string]func(context.Context, *Service) error{
		"the write": func(ctx context.Context, s *Service) error {
			_, err := s.record(ctx, written)

			return err
		},
		"the detail": func(ctx context.Context, s *Service) error {
			_, err := s.detail(ctx, &TrendInput{Metric: off, Window: string(WindowWeek)})

			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			pool, err := pgxpool.New(t.Context(), "postgres://nobody:nothing@127.0.0.1:1/none")
			if err != nil {
				t.Fatalf("building a pool: %v", err)
			}
			t.Cleanup(pool.Close)

			ctx := auth.WithPrincipal(t.Context(), auth.Principal{Subject: aPatient, Role: "patient"})
			refusal := ask(ctx, NewService(
				func() time.Time { return time.Unix(0, 0).UTC() }, Deps{RequestPool: pool},
			))

			var status huma.StatusError
			if !errors.As(refusal, &status) || status.GetStatus() != http.StatusUnprocessableEntity {
				t.Errorf("a metric off the set answered %v", refusal)
			}
		})
	}
}

// The rendering, on the ordinary gate. Everything below is pure — it takes domain values and
// answers wire ones — and until it was written the only thing measuring it was the tagged
// suite, so on a host with no Docker daemon the zone, the range, the threshold and the dose
// could all have been dropped behind a green gate.
func TestTheSpanIsRenderedAsDatesWithTheZoneBesideThem(t *testing.T) {
	from, through := civil.NewDate(2026, time.July, 30), civil.NewDate(2026, time.August, 5)
	r, ok := civil.NewRange(from, through)
	if !ok {
		t.Fatal("the fixture's own range runs backwards")
	}

	covered := renderSpan(Span{Window: WindowWeek, Range: &r, Timezone: "Asia/Yekaterinburg"})
	if covered.Window != "7d" || covered.Timezone != "Asia/Yekaterinburg" {
		t.Errorf("the span is dated %q in %q", covered.Window, covered.Timezone)
	}
	if covered.Range == nil || covered.Range.From != "2026-07-30" || covered.Range.Through != "2026-08-05" {
		t.Errorf("the range renders as %+v", covered.Range)
	}

	// Absent and not a range of nothing: civil.Range{} would render as 0000-01-01, which a
	// client would draw as an axis rather than as «this patient has no course».
	empty := renderSpan(Span{Window: WindowCycle, Timezone: "Europe/Moscow"})
	if empty.Window != "cycle" || empty.Timezone != "Europe/Moscow" || empty.Range != nil {
		t.Errorf("an uncovered window renders as %+v", empty)
	}
}

// The unit and the direction come off the metric and never off the point, and the bound is
// absent for five of the eight rather than zero. Every metric, so a row added to Meta without
// one here is a failure and not a metric nobody rendered.
func TestEveryMetricRendersItsOwnMetaAndItsOwnPointsOnly(t *testing.T) {
	readings := []Reading{
		{ID: "b", Metric: MetricWeight, Value: 82.4, MeasuredAt: time.Unix(200, 0).UTC()},
		{ID: "a", Metric: MetricWeight, Value: 83.1, MeasuredAt: time.Unix(100, 0).UTC()},
		{ID: "c", Metric: MetricHRV, Value: 61, MeasuredAt: time.Unix(300, 0).UTC()},
	}

	withABound := map[Metric]float64{MetricHRV: 58, MetricRHR: 60, MetricSleep: 75}

	for _, metric := range Metrics() {
		body, err := renderSeries(SeriesOf(metric, readings))
		if err != nil {
			t.Fatalf("rendering the %s: %v", metric, err)
		}
		if body.Metric != string(metric) {
			t.Errorf("the %s renders as %q", metric, body.Metric)
		}

		meta, _ := Meta(metric)
		if body.Meta.Unit != meta.Unit || body.Meta.Direction != string(meta.Direction) {
			t.Errorf("the %s renders as %+v", metric, body.Meta)
		}

		want, bounded := withABound[metric]
		switch {
		case bounded && (body.Meta.Threshold == nil || body.Meta.Threshold.Value != want):
			t.Errorf("the %s renders the threshold %+v, want %v", metric, body.Meta.Threshold, want)
		case !bounded && body.Meta.Threshold != nil:
			t.Errorf("the %s renders a threshold of %v", metric, body.Meta.Threshold.Value)
		}
	}

	// The points are this metric's, oldest first, and an empty list rather than a null.
	weight, err := renderSeries(SeriesOf(MetricWeight, readings))
	if err != nil {
		t.Fatalf("rendering the weight: %v", err)
	}
	if len(weight.Points) != 2 || weight.Points[0].ID != "a" || weight.Points[0].Value != 83.1 ||
		weight.Points[1].ID != "b" {
		t.Errorf("the weight renders %+v", weight.Points)
	}
	if chest, _ := renderSeries(SeriesOf(MetricChest, readings)); chest.Points == nil {
		t.Error("an unmeasured metric renders a null list of points")
	}
}

// A ninth metric would come off Meta's switch with no unit at all, and an axis published
// without one is worse than a refusal: the client draws numbers against nothing.
func TestAMetricWithNoMetaIsARefusalAndNotAUnitlessAxis(t *testing.T) {
	if _, err := renderSeries(Series{Metric: Metric("thigh")}); !errors.Is(err, ErrUnknownMetric) {
		t.Errorf("a metric off the set rendered with %v", err)
	}
}

// A dose is a value and a unit all the way to the wire: «0,25 мг» is the client's sentence.
func TestADoseKeepsItsUnitOntoTheWire(t *testing.T) {
	rendered := renderDose(protocol.Dose{Value: 0.25, Unit: protocol.MG})
	if rendered.Value != 0.25 || rendered.Unit != "мг" {
		t.Errorf("the dose renders as %+v", rendered)
	}
}

// The write's own set, and it is seven where both reads offer eight: the sleep score is
// computed from imported sessions, so there is no number for a patient to type. Reconciled
// against WritableMetrics() in both directions for the reason the windows are — a code the
// schema offers and the server refuses is a 422 the client cannot avoid, and one the server
// would take and the schema hides is a reading nobody can send.
func TestTheWriteSchemaOffersOnlyTheMetricsAPatientTypes(t *testing.T) {
	api := registered(t)

	var want []string
	for _, metric := range WritableMetrics() {
		want = append(want, string(metric))
	}

	body := requestBody(t, api, operation(t, api, http.MethodPost, recordPath))
	offered := published(t, propertyOf(t, body, "metric"))
	if !slices.Equal(offered, want) {
		t.Errorf("the write offers the metrics %v, and the server writes %v", offered, want)
	}
	if slices.Contains(offered, string(MetricSleep)) {
		t.Error("the write offers the sleep score")
	}
}

// The source is read back off the row rather than assumed, so the reply publishes the set the
// column holds and not the one value this operation can produce today.
func TestTheReplyPublishesTheSourcesTheServerParses(t *testing.T) {
	api := registered(t)

	var want []string
	for _, source := range Sources() {
		want = append(want, string(source))
	}

	recorded := api.OpenAPI().Components.Schemas.SchemaFromRef("#/components/schemas/RecordedBody")
	if recorded == nil {
		t.Fatal("no schema named RecordedBody")
	}
	if offered := published(t, propertyOf(t, recorded, "source")); !slices.Equal(offered, want) {
		t.Errorf("the reply publishes the sources %v, and the server parses %v", offered, want)
	}
}

// The path that bypasses the generated client: a hand-written request carrying the one metric
// no patient types. The contract's enum is what refuses it here, and Record refuses it again
// below the transport — write_test.go measures that half, over a transaction that counts.
func TestTheSleepScoreIsRefusedToAClientThatIgnoresTheContract(t *testing.T) {
	status, body := sent(t, http.MethodPost, recordPath,
		`{"metric":"sleep","value":82,"measured_at":"2026-08-05T03:00:00Z"}`,
		aPatient, "patient")
	if status != http.StatusUnprocessableEntity {
		t.Errorf("a sleep score answered %d, want 422: %s", status, body)
	}

	// The control, and the reason the assertion above is not satisfied by a body that is
	// malformed in some other way: the same request with a metric a patient does type is
	// refused by nothing above the database, and dies at the pool that goes nowhere.
	status, body = sent(t, http.MethodPost, recordPath, aReading, aPatient, "patient")
	if status != http.StatusServiceUnavailable {
		t.Errorf("a weight answered %d, want the 503 of a pool that goes nowhere: %s", status, body)
	}
}

// The other closed sets of the write, refused before a connection is asked for.
func TestABodyTheContractDoesNotAdmitIsRefused(t *testing.T) {
	for _, refused := range []struct{ name, body string }{
		{"a metric §03 does not have", `{"metric":"thigh","value":81.2,"measured_at":"2026-08-05T03:00:00Z"}`},
		{"the plural spelling the wire does not carry", `{"metric":"hips","value":81.2,"measured_at":"2026-08-05T03:00:00Z"}`},
		{"no metric at all", `{"value":81.2,"measured_at":"2026-08-05T03:00:00Z"}`},
		{"no instant at all", `{"metric":"weight","value":81.2}`},
		{"an instant that is a date", `{"metric":"weight","value":81.2,"measured_at":"2026-08-05"}`},
		{"a note of two thousand and one characters", `{"metric":"weight","value":81.2,` +
			`"measured_at":"2026-08-05T03:00:00Z","note":"` + strings.Repeat("я", 2001) + `"}`},
		// The unit and the source are the server's, and refusing them is not the same as
		// having no field for them: a body that offers one has to be refused rather than
		// quietly ignored, or a hand-typed row could claim to have come off a watch.
		{"a source the request chose", `{"metric":"weight","value":81.2,` +
			`"measured_at":"2026-08-05T03:00:00Z","source":"healthkit"}`},
		{"a unit the request chose", `{"metric":"weight","value":81.2,` +
			`"measured_at":"2026-08-05T03:00:00Z","unit":"lb"}`},
	} {
		t.Run(refused.name, func(t *testing.T) {
			status, body := sent(t, http.MethodPost, recordPath, refused.body, aPatient, "patient")
			if status != http.StatusUnprocessableEntity {
				t.Errorf("answered %d, want 422: %s", status, body)
			}
		})
	}
}

// An identifier that is not one never reaches a statement: `WHERE id = $1` on it raises 22P02,
// which is a 500 for what is not a row. The schema refuses it here and Delete refuses it again
// for a caller with no document — that half is measured in the tagged suite.
func TestAnIdentifierThatIsNotOneIsRefusedByTheSchema(t *testing.T) {
	status, body := sent(t, http.MethodDelete, "/v1/me/measurements/the-one-i-deleted", "", aPatient, "patient")
	if status != http.StatusUnprocessableEntity {
		t.Errorf("answered %d, want 422: %s", status, body)
	}
}

const (
	aPatient = "8a1f3b7c-0000-4000-8000-000000000001"
	// A reading and not the patient: one uuid spelling two kinds of identity is a fixture
	// shape that has hidden a transition in this repository before.
	aReadingID = "8a1f3b7c-0000-4000-8000-0000000000e1"
)

// The same refusal both reads make, on the two operations that change something: for an
// admin every policy on this table is USING (true), so the subject these statements are
// bounded by would be the only boundary left — and it comes off the token.
func TestOnlyAPatientRecordsAndRemovesTheirOwnReadings(t *testing.T) {
	for _, refused := range []struct {
		name, method, path, body, role string
		status                         int
	}{
		{"recording with no principal", http.MethodPost, recordPath, aReading, "", http.StatusUnauthorized},
		{"recording as a doctor", http.MethodPost, recordPath, aReading, "doctor", http.StatusForbidden},
		{"recording as an admin", http.MethodPost, recordPath, aReading, "admin", http.StatusForbidden},
		{
			"removing with no principal", http.MethodDelete,
			"/v1/me/measurements/" + aReadingID, "", "", http.StatusUnauthorized,
		},
		{
			"removing as a doctor", http.MethodDelete,
			"/v1/me/measurements/" + aReadingID, "", "doctor", http.StatusForbidden,
		},
		{
			"removing as an admin", http.MethodDelete,
			"/v1/me/measurements/" + aReadingID, "", "admin", http.StatusForbidden,
		},
	} {
		t.Run(refused.name, func(t *testing.T) {
			status, body := sent(t, refused.method, refused.path, refused.body, aPatient, refused.role)
			if status != refused.status {
				t.Errorf("answered %d, want %d: %s", status, refused.status, body)
			}
		})
	}
}

// Every refusal the write and the delete can produce. The two mappers are separate from the
// reads' for one reason: a refusal a patient can act on has to be a 4xx here and a fault of
// this process a 500, and ErrUnknownMetric is the second on both paths — it can only mean
// Meta and ParseMetric disagree about the eight.
func TestEachRefusalOfTheWriteBecomesItsOwnStatus(t *testing.T) {
	for _, mapped := range []struct {
		name    string
		mapping func(error) error
		err     error
		want    int
	}{
		{"the sleep score", answerWrite, ErrMetricNotWritable, http.StatusUnprocessableEntity},
		{"a reading from the future", answerWrite, ErrMeasuredInTheFuture, http.StatusUnprocessableEntity},
		{"a slipped decimal point", answerWrite, ErrValueImplausible, http.StatusUnprocessableEntity},
		{"a note of whitespace", answerWrite, ErrNoteSaysNothing, http.StatusUnprocessableEntity},
		{"a wrapped refusal", answerWrite, fmt.Errorf("recording: %w", ErrValueImplausible), http.StatusUnprocessableEntity},
		{"a metric Meta has no row for", answerWrite, ErrUnknownMetric, http.StatusInternalServerError},
		{"a database that cannot answer", answerWrite, fmt.Errorf("q: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
		{"anything nobody named", answerWrite, errors.New("unnamed"), http.StatusInternalServerError},

		{"a reading the patient does not hold", answerDelete, ErrNoSuchReading, http.StatusNotFound},
		{"their own imported reading", answerDelete, ErrReadingWasImported, http.StatusConflict},
		{"a wrapped absence", answerDelete, fmt.Errorf("deleting: %w", ErrNoSuchReading), http.StatusNotFound},
		{"a database that cannot answer", answerDelete, fmt.Errorf("q: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
		{"anything nobody named", answerDelete, errors.New("unnamed"), http.StatusInternalServerError},
	} {
		t.Run(mapped.name, func(t *testing.T) {
			var status huma.StatusError
			if !errors.As(mapped.mapping(mapped.err), &status) {
				t.Fatalf("%v did not become a status", mapped.err)
			}
			if status.GetStatus() != mapped.want {
				t.Errorf("%v became %d, want %d", mapped.err, status.GetStatus(), mapped.want)
			}
		})
	}
}

// Every way a reading can be absent answers one sentence, and the status is only half of an
// answer: a message wrapping the identifier would make «not yours» and «nobody's» two replies
// that differ, which is the distinction the 404 exists to refuse to make.
func TestTheRefusedDeleteAnswersOneSentenceWhateverWasAsked(t *testing.T) {
	absences := []error{
		answerDelete(fmt.Errorf("%s: %w", "7c4d1a90-0000-4000-8000-0000000000b1", ErrNoSuchReading)),
		answerDelete(fmt.Errorf("%s: %w", "7c4d1a90-0000-4000-8000-00000000dead", ErrNoSuchReading)),
	}
	if absences[0].Error() != absences[1].Error() {
		t.Errorf("two absences answer %q and %q", absences[0], absences[1])
	}
	// Which sentence it is, and not only that the two agree on it: agreeing on the
	// conflict's sentence would satisfy the line above and tell a patient deleting a
	// reading nobody holds that theirs was imported.
	if absences[0].Error() != ErrNoSuchReading.Error() {
		t.Errorf("an absence answers %q", absences[0])
	}

	// The conflict's own wrap carries the row's source, which is the patient's own fact; it
	// is dropped for the same reason, so that the one refusal that IS about their row says
	// the one thing it is allowed to say.
	conflict := answerDelete(fmt.Errorf(
		"%s came from healthkit: %w", "7c4d1a90-0000-4000-8000-0000000000a1", ErrReadingWasImported,
	))
	if conflict.Error() != ErrReadingWasImported.Error() {
		t.Errorf("the conflict answers %q", conflict)
	}
}

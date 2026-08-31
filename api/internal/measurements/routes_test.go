package measurements

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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
)

// registered is this context's operations alone on a document of their own, so that what the
// assertions below walk is what this package published and not what eleven contexts did.
func registered(t *testing.T) huma.API {
	t.Helper()

	api := httpserver.NewAPI(chi.NewRouter())
	NewService(func() time.Time { return time.Unix(0, 0).UTC() }, Deps{}).Register(api)

	return api
}

func operation(t *testing.T, api huma.API, path string) *huma.Operation {
	t.Helper()

	item, ok := api.OpenAPI().Paths[path]
	if !ok || item == nil || item.Get == nil {
		t.Fatalf("%s serves no GET", path)
	}

	return item.Get
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
		offered := published(t, parameter(t, operation(t, api, path), "window").Schema)
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

	offered := published(t, parameter(t, operation(t, api, detailPath), "metric").Schema)
	if !slices.Equal(offered, want) {
		t.Errorf("the path offers the metrics %v, and the server parses %v", offered, want)
	}
}

// Published rather than applied in the handler, so a client reading the contract is told what
// an absent window means — and it is three months, which is where the patient app opens.
func TestAnAbsentWindowIsTheOneThePatientAppOpensOn(t *testing.T) {
	api := registered(t)

	for _, path := range []string{overviewPath, detailPath} {
		schema := parameter(t, operation(t, api, path), "window").Schema
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

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

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
		{"a metric spelled as the component note spells it", "/v1/me/trends/hips"},
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

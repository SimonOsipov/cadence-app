package measurements

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The request pool alone: every operation here is the patient acting on their own rows, so
// they run under the patient's identity and RLS is the boundary. Nothing reaches the service
// seam.
type Service struct {
	requests *pgxpool.Pool

	// Injected: see NewService.
	now func() time.Time
}

// Deps is what this context needs from outside itself.
type Deps struct {
	RequestPool *pgxpool.Pool
}

// NewService takes the clock positionally, for the reason protocol.NewService records.
func NewService(now func() time.Time, deps Deps) *Service {
	return &Service{requests: deps.RequestPool, now: now}
}

// Register mounts this context's operations on the API.
func (s *Service) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-trends",
		Method:      http.MethodGet,
		Path:        "/v1/me/trends",
		Summary:     "Every metric over one window",
		Description: "The trends list: one window, all eight metrics in the order they are " +
			"enumerated, the ones never measured included — a patient has to be able to " +
			"find out that a metric is unmeasured, and a dropped entry says nothing. Each " +
			"carries the unit it is read in, the way it has to move, and the clinical bound " +
			"where there is one; three of the eight have one and the rest carry null rather " +
			"than a zero that would read as «any reading clears it». The answer is dated in " +
			"the patient's own zone, and carries it: without it a client bins the points " +
			"into its own days and a reading admitted at the window's edge is drawn beyond " +
			"it. `cycle` is the geometry of the patient's last course — its start through " +
			"today or its last prescribed day, whichever is earlier — and its range is null " +
			"for a patient with no course and for one whose course has not started. " +
			"Nothing derived travels: the base, the latest, the delta, the average, the " +
			"extremes and the movement are arithmetic over the points, and a second " +
			"definition of them here is a second truth free to disagree.",
		Tags: []string{"measurements"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden,
			http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, s.overview)

	huma.Register(api, huma.Operation{
		OperationID: "get-metric-trend",
		Method:      http.MethodGet,
		Path:        "/v1/me/trends/{metric}",
		Summary:     "One metric with the prescription under it",
		Description: "The detail screen: one metric's points over the window, and the strip " +
			"of prescription drawn beneath them — the dose bands of the patient's last " +
			"course and the days it started or was titrated, both clipped to the window. " +
			"The strip follows the titrating position of the course, falling back to the " +
			"first injectable one, whose flat band is the prescribed dose; a course with " +
			"no injections draws no strip, and neither does a cancelled one, whose days " +
			"still make the window. A metric that was never measured answers an empty " +
			"list of points rather than a 404 — that it is unmeasured is the answer.",
		Tags: []string{"measurements"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden,
			http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, s.detail)

	s.registerWrites(api)

	httpserver.AdmitNull(api, "TrendsBody", "range")
	httpserver.AdmitNull(api, "TrendBody", "range")
	httpserver.AdmitNull(api, "MetricMetaBody", "threshold")
	httpserver.AdmitNull(api, "ProtocolMarkBody", "from")
}

// TrendsInput and TrendInput carry the window the same way, and the default is published
// rather than applied here. It is the window the patient app opens on in both its generations
// — AppState.tsx in the frozen prototype and TrendsUiState in the ported shell. The dashboard
// draws a sparkline off a fixture and asks for no window, so it has no opinion here.
//
// Both closed sets are written out, because a struct tag cannot call Windows() or Metrics().
// The two spellings are reconciled by the tests that read them back off the registered
// document: a code one side knows and the other does not is either a 422 the client cannot
// avoid or a value nobody can ask for.
type TrendsInput struct {
	Window string `query:"window" enum:"7d,4w,3m,cycle" default:"3m" doc:"The span to draw. The cycle window is the last course's geometry, so its width depends on the course rather than on the calendar."`
}

type TrendInput struct {
	Metric string `path:"metric" enum:"weight,hrv,rhr,sleep,bodyfat,waist,hip,chest"`
	Window string `query:"window" enum:"7d,4w,3m,cycle" default:"3m" doc:"The span to draw. The cycle window is the last course's geometry, so its width depends on the course rather than on the calendar."`
}

type TrendsOutput struct {
	Body TrendsBody
}

type TrendsBody struct {
	SpanBody

	Metrics []MetricTrendBody `json:"metrics" nullable:"false" doc:"All eight, in enumeration order, unmeasured ones included."`
}

type TrendOutput struct {
	Body TrendBody
}

type TrendBody struct {
	SpanBody

	Metric MetricTrendBody    `json:"metric"`
	Bands  []DoseBandBody     `json:"bands" nullable:"false" doc:"The prescribed dose over the window. Empty when the course draws no strip."`
	Marks  []ProtocolMarkBody `json:"marks" nullable:"false" doc:"The day the strip opened and each day it changed, clipped to the same window."`
}

// SpanBody is what an answer is dated in, and it is one value in both bodies rather than three
// fields written out twice: the two answers cannot then disagree about the shape, and nothing
// can transpose the window and the zone, which are both strings.
type SpanBody struct {
	Window   string          `json:"window" enum:"7d,4w,3m,cycle"`
	Range    *TrendRangeBody `json:"range" doc:"Null when the window covers no days at all, which only the cycle window does: the patient has no course, or the one they have has not started."`
	Timezone string          `json:"timezone" doc:"The IANA zone the range's days are the patient's own in."`
}

// TrendRangeBody is dates and not instants: a window is a set of the patient's own days, and
// the zone beside it is what those days are counted in.
type TrendRangeBody struct {
	From    string `json:"from" format:"date"`
	Through string `json:"through" format:"date" doc:"Inclusive: a window is a set of days, not a subtraction."`
}

type MetricTrendBody struct {
	Metric string           `json:"metric" enum:"weight,hrv,rhr,sleep,bodyfat,waist,hip,chest"`
	Meta   MetricMetaBody   `json:"meta"`
	Points []TrendPointBody `json:"points" nullable:"false" doc:"Oldest first, ties broken by identifier, and empty for a metric this patient has not measured in this window."`
}

// MetricMetaBody is the clinically significant half of the constants module. The label, the
// decimal places and the accent are rendering and stay on the surface that draws them.
type MetricMetaBody struct {
	Unit      string         `json:"unit" doc:"The unit every point of this metric is in; a point carries none of its own."`
	Direction string         `json:"direction" enum:"up,down" doc:"Which way the metric has to move for the patient to be getting better, and the side of the threshold that is well."`
	Threshold *ThresholdBody `json:"threshold" doc:"Null for the five metrics the clinic sets no bound on — absent, and not a zero every reading would clear."`
}

type ThresholdBody struct {
	Value float64 `json:"value"`
}

// TrendPointBody carries the row's own identifier because the reading a patient asks to have
// deleted is the point they are looking at.
type TrendPointBody struct {
	ID         string    `json:"id" format:"uuid"`
	Value      float64   `json:"value"`
	MeasuredAt time.Time `json:"measured_at"`
}

// DoseBandBody is a prescription and not a history: no dose the patient recorded reaches it.
// A band vanishing because a week was missed would tell a patient their protocol changed.
type DoseBandBody struct {
	Dose  protocol.DoseBody `json:"dose"`
	Range TrendRangeBody    `json:"range"`
}

type ProtocolMarkBody struct {
	Kind string             `json:"kind" enum:"start,titration"`
	Date string             `json:"date" format:"date"`
	From *protocol.DoseBody `json:"from" doc:"Null on a start mark: there is no dose to have come up from."`
	To   protocol.DoseBody  `json:"to"`
}

func (s *Service) overview(ctx context.Context, in *TrendsInput) (*TrendsOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}
	window, err := askedWindow(in.Window)
	if err != nil {
		return nil, err
	}

	var trends Trends
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		trends, err = TrendsFor(ctx, tx, patient, s.now(), window)

		return err
	}); err != nil {
		return nil, answer("reading the trends", err)
	}

	body := TrendsBody{Metrics: make([]MetricTrendBody, 0, len(trends.Series))}
	body.SpanBody = renderSpan(trends.Span)
	for _, series := range trends.Series {
		rendered, err := renderSeries(series)
		if err != nil {
			return nil, answer("reading the trends", err)
		}
		body.Metrics = append(body.Metrics, rendered)
	}

	return &TrendsOutput{Body: body}, nil
}

func (s *Service) detail(ctx context.Context, in *TrendInput) (*TrendOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}
	window, err := askedWindow(in.Window)
	if err != nil {
		return nil, err
	}
	metric, err := askedMetric(in.Metric)
	if err != nil {
		return nil, err
	}

	var trend Trend
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		trend, err = TrendFor(ctx, tx, patient, s.now(), window, metric)

		return err
	}); err != nil {
		return nil, answer("reading the metric", err)
	}

	body := TrendBody{
		Bands: make([]DoseBandBody, 0, len(trend.Overlay.Bands)),
		Marks: make([]ProtocolMarkBody, 0, len(trend.Overlay.Marks)),
	}
	body.SpanBody = renderSpan(trend.Span)
	if body.Metric, err = renderSeries(trend.Series); err != nil {
		return nil, answer("reading the metric", err)
	}
	for _, band := range trend.Overlay.Bands {
		body.Bands = append(body.Bands, DoseBandBody{
			Dose:  renderDose(band.Dose),
			Range: renderRange(band.Range),
		})
	}
	for _, mark := range trend.Overlay.Marks {
		rendered := ProtocolMarkBody{
			Kind: string(mark.Kind),
			Date: mark.Date.String(),
			To:   renderDose(mark.To),
		}
		if mark.From != nil {
			from := renderDose(*mark.From)
			rendered.From = &from
		}
		body.Marks = append(body.Marks, rendered)
	}

	return &TrendOutput{Body: body}, nil
}

// askedWindow is the guard below the schema's: over HTTP the enum keyword refuses an unknown
// code first and the default fills an absent one. This answers for the day either keyword is
// dropped from the field, and for a caller that reaches this package without a schema at all.
func askedWindow(asked string) (Window, error) {
	window, ok := ParseWindow(asked)
	if !ok {
		return "", huma.Error422UnprocessableEntity("window is not one this API knows: " + asked)
	}

	return window, nil
}

// askedMetric is askedWindow's twin, below the enum keyword of a path and of a body alike, and
// it holds all eight: whether a metric can be typed is Record's answer, and a second copy of
// that set here would leave one of the two unmeasurable.
func askedMetric(asked string) (Metric, error) {
	metric, ok := ParseMetric(asked)
	if !ok {
		return "", huma.Error422UnprocessableEntity("metric is not one this API knows: " + asked)
	}

	return metric, nil
}

// patient is the caller, refused unless they are one. Every neighbouring /v1/me surface
// refuses the same way: measurements_admin is FOR ALL USING (true), so for an admin the
// subject these reads filter by would be the only boundary left.
func (s *Service) patient(ctx context.Context) (civil.UserID, database.Caller, error) {
	if s.requests == nil || s.now == nil {
		return "", database.Caller{},
			huma.Error500InternalServerError("this API was assembled without its reads")
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return "", database.Caller{},
			huma.Error401Unauthorized("no verified principal on the request context")
	}
	if principal.Role != "patient" {
		return "", database.Caller{}, huma.Error403Forbidden("these are a patient's own measurements")
	}

	subject := strings.ToLower(principal.Subject)

	return civil.UserID(subject), database.Caller{Subject: subject, Role: principal.Role}, nil
}

func renderSpan(span Span) SpanBody {
	body := SpanBody{Window: string(span.Window), Timezone: span.Timezone}
	if span.Range != nil {
		rendered := renderRange(*span.Range)
		body.Range = &rendered
	}

	return body
}

func renderRange(r civil.Range) TrendRangeBody {
	return TrendRangeBody{From: r.From.String(), Through: r.Through.String()}
}

// The meta is asked for rather than carried on the series, and a metric with none is an
// error and not an empty row: Meta's switch has no default, so a ninth metric reaching the
// wire would otherwise publish a unitless axis instead of failing.
func renderSeries(series Series) (MetricTrendBody, error) {
	meta, ok := Meta(series.Metric)
	if !ok {
		return MetricTrendBody{}, ErrUnknownMetric
	}

	body := MetricTrendBody{
		Metric: string(series.Metric),
		Meta:   MetricMetaBody{Unit: meta.Unit, Direction: string(meta.Direction)},
		Points: make([]TrendPointBody, 0, len(series.Points)),
	}
	if meta.Threshold != nil {
		body.Meta.Threshold = &ThresholdBody{Value: meta.Threshold.Value}
	}
	for _, point := range series.Points {
		body.Points = append(body.Points, TrendPointBody{
			ID: string(point.ID), Value: point.Value, MeasuredAt: point.MeasuredAt,
		})
	}

	return body, nil
}

func renderDose(dose protocol.Dose) protocol.DoseBody {
	return protocol.DoseBody{Value: dose.Value, Unit: string(dose.Unit)}
}

func (s *Service) registerWrites(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "record-measurement",
		Method:        http.MethodPost,
		Path:          "/v1/me/measurements",
		DefaultStatus: http.StatusCreated,
		Summary:       "Write one reading the patient typed",
		Description: "Records one hand-typed reading and answers the row that was written. " +
			"The unit is the server's — it is a function of the metric, so a request cannot " +
			"carry one — and so is the source, which is why the reply reads it back off the " +
			"row rather than asserting it: a hand-typed reading cannot claim to have come " +
			"off a watch. Seven metrics can be written where both reads answer eight: the " +
			"sleep score is computed from imported sessions, and there is no number for a " +
			"patient to type. Answers 422 for a reading measured after it was recorded, for " +
			"a value outside what the metric can plausibly read — a slipped decimal point, " +
			"not a reading the clinic would worry about, which is what the threshold is for " +
			"— and for a note of nothing but whitespace.",
		Tags: []string{"measurements"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden,
			http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, s.record)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-measurement",
		Method:        http.MethodDelete,
		Path:          "/v1/me/measurements/{id}",
		DefaultStatus: http.StatusNoContent,
		Summary:       "Remove one reading the patient typed",
		Description: "Removes one of the patient's own hand-typed readings — a typo is " +
			"corrected by deleting the point and entering it again, because a reading is a " +
			"clinical fact and rewriting one would leave no trace that it had been. A " +
			"reading this patient does not hold answers 404, and so does an identifier " +
			"nobody holds: the two cannot differ, or the status alone would report whether " +
			"another patient's reading exists. Their own imported reading answers 409 and " +
			"says why — that row IS on their screen, so calling it absent would be a lie, " +
			"and the sample returns on the next sync anyway.",
		Tags: []string{"measurements"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
			http.StatusConflict, http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, s.remove)
}

// RecordInput is the add sheet's payload. No unit and no source: both are the server's, for
// the reason Draft records, and a field for either would be a value the request could lie in.
//
// The metric set is the write's own seven, written out because a struct tag cannot call
// WritableMetrics(); the two spellings are reconciled by the test that reads this back off the
// registered document. The note's bounds are 000025's `length(note) BETWEEN 1 AND 2000` — the
// column also refuses one that is all whitespace, which no keyword spells and Record names.
type RecordInput struct {
	Body struct {
		Metric     string    `json:"metric" enum:"weight,hrv,rhr,bodyfat,waist,hip,chest" doc:"The sleep score is absent: it is computed from imported sessions and there is nothing to type."`
		Value      float64   `json:"value" doc:"In the unit the metric is read in, which the reply carries back."`
		MeasuredAt time.Time `json:"measured_at" doc:"When the reading was taken, which is where it lands on the axis — not when it was entered. A reading measured after the server's own instant is refused."`
		Note       *string   `json:"note,omitempty" minLength:"1" maxLength:"2000"`
	}
}

type RecordOutput struct {
	Body RecordedBody
}

// RecordedBody is the row as it was written, and the identifier is the point the patient will
// be looking at when they ask for it to be deleted.
type RecordedBody struct {
	ID         string    `json:"id" format:"uuid"`
	Metric     string    `json:"metric" enum:"weight,hrv,rhr,bodyfat,waist,hip,chest"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit" doc:"The server's, off the metric: a point on the axis carries none of its own."`
	MeasuredAt time.Time `json:"measured_at"`
	Source     string    `json:"source" enum:"manual,healthkit,health_connect" doc:"Read back off the row rather than asserted. This operation writes nothing but a hand-typed reading, so it is the column's default on a column the patient holds no grant on."`
}

type DeleteInput struct {
	ID string `path:"id" format:"uuid" doc:"The reading, as the point on the axis carries it."`
}

type DeleteOutput struct{}

func (s *Service) record(ctx context.Context, in *RecordInput) (*RecordOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}
	metric, err := askedMetric(in.Body.Metric)
	if err != nil {
		return nil, err
	}

	var recorded Recorded
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		recorded, err = Record(ctx, tx, patient, s.now(), Draft{
			Metric:     metric,
			Value:      in.Body.Value,
			MeasuredAt: in.Body.MeasuredAt,
			Note:       in.Body.Note,
		})

		return err
	}); err != nil {
		return nil, answerWrite(err)
	}

	return &RecordOutput{Body: RecordedBody{
		ID:         string(recorded.ID),
		Metric:     string(recorded.Metric),
		Value:      recorded.Value,
		Unit:       recorded.Unit,
		MeasuredAt: recorded.MeasuredAt,
		Source:     string(recorded.Source),
	}}, nil
}

func (s *Service) remove(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		return Delete(ctx, tx, patient, ReadingID(in.ID))
	}); err != nil {
		return nil, answerDelete(err)
	}

	return &DeleteOutput{}, nil
}

// answer maps a refusal to a status. Only what a read can produce is here; the write and the
// delete of step 8 bring their own. ErrUnknownMetric is deliberately absent: on a read it can
// only mean Meta and ParseMetric disagree about the eight, which is a fault of this process
// and not of the request, so it falls through to the 500 below.
func answer(doing string, err error) error {
	switch {
	case errors.Is(err, ErrNoTimezone):
		// A provisioning fault rather than a bad request: the patient's account is
		// missing something the clinic was meant to record.
		return huma.Error500InternalServerError("the patient's timezone is not recorded", err)
	case database.IsUnavailable(err):
		return huma.Error503ServiceUnavailable("the database could not serve the request", err)
	default:
		return huma.Error500InternalServerError(doing, err)
	}
}

// answerWrite maps the write's own refusals, which the reads cannot produce. Separate from
// answer rather than folded into it: ErrUnknownMetric means the same thing on both paths —
// Meta and ParseMetric disagreeing about the eight — and it falls through to the 500 there,
// while every refusal below is one the patient can act on by editing the sheet.
func answerWrite(err error) error {
	switch {
	case errors.Is(err, ErrMetricNotWritable), errors.Is(err, ErrMeasuredInTheFuture),
		errors.Is(err, ErrValueImplausible), errors.Is(err, ErrNoteSaysNothing):
		return huma.Error422UnprocessableEntity(err.Error())
	default:
		return answer("recording the reading", err)
	}
}

// Each answer carries its sentinel's own sentence and not the wrapped error's: a sentence
// naming the reading would split one answer into two.
func answerDelete(err error) error {
	switch {
	case errors.Is(err, ErrNoSuchReading):
		return huma.Error404NotFound(ErrNoSuchReading.Error())
	case errors.Is(err, ErrReadingWasImported):
		return huma.Error409Conflict(ErrReadingWasImported.Error())
	default:
		return answer("deleting the reading", err)
	}
}

package dosing

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// One pool, and it is the request one: everything here is the patient acting on their own
// rows, so it runs under their identity and RLS answers. Nothing reaches the service seam.
type Service struct {
	requests *pgxpool.Pool

	// Injected: see NewService.
	now func() time.Time

	photos      Photos
	photoBucket string
}

// Deps is what this context needs from outside itself.
type Deps struct {
	RequestPool *pgxpool.Pool

	// Useless apart, and both absent is how the document generator builds this:
	// the operations are declared either way.
	Photos      Photos
	PhotoBucket string
}

// NewService takes the clock positionally, for the reason protocol.NewService records.
func NewService(now func() time.Time, deps Deps) *Service {
	return &Service{
		requests:    deps.RequestPool,
		now:         now,
		photos:      deps.Photos,
		photoBucket: deps.PhotoBucket,
	}
}

// Register mounts this context's operations on the API.
func (s *Service) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "log-dose",
		Method:        http.MethodPost,
		Path:          "/v1/me/dose-events",
		DefaultStatus: http.StatusOK,
		Summary:       "Record a dose",
		Description: "Writes the dose and the day's diary entry in one transaction — one " +
			"action, two facts — and answers which of four things happened. The four are a " +
			"200 with a body and not HTTP errors: the client branches on them, so they are " +
			"expected results rather than failures. `written` carries the identifiers; " +
			"`already_logged` means every occurrence of that item today is recorded, so a " +
			"repeat cannot take another dose out of the vial; `not_scheduled_today` means " +
			"the item has no occurrence in the patient's own day, which is what an app left " +
			"open across midnight produces; `incomplete` means the draft cannot make an " +
			"event. A repeat carrying the same client_request_id answers what the first " +
			"answered and writes nothing, and one carrying a different draft is a 409 " +
			"naming the field that differs. The dose recorded is the one the request " +
			"carries: the wizard's dose is editable, and a patient who stepped down " +
			"from what the course prescribes has done so — the prescription stays in " +
			"the course, so the two can be compared.",
		Tags: []string{"dosing"},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.log)

	s.registerPhotos(api)
}

// photoCaller is the /v1/me refusal, shared by the two photograph operations.
//
// A patient-only surface for the same reason logging a dose is one: for an admin
// every policy on dose_events is USING (true), so the identifier in the path would
// be the only boundary — and it is a client-supplied one.
func (s *Service) photoCaller(ctx context.Context) (database.Caller, error) {
	if s.requests == nil || s.photos == nil || s.photoBucket == "" {
		return database.Caller{}, huma.Error500InternalServerError(
			"this API was assembled without somewhere to keep photographs",
		)
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return database.Caller{}, huma.Error401Unauthorized("no verified principal on the request context")
	}
	if principal.Role != "patient" {
		return database.Caller{}, huma.Error403Forbidden("only a patient reads their own photographs")
	}

	return database.Caller{Subject: principal.Subject, Role: principal.Role}, nil
}

// LogDoseInput is the wizard's payload. Everything but the item, the dose and the key is
// optional: the wizard's step 4 is «Короткая сверка — всё по желанию» (LogDoseScreen.tsx).
type LogDoseInput struct {
	Body struct {
		ItemID string `json:"protocol_item_id" format:"uuid" doc:"The prescribed item this dose answers."`

		DoseValue float64 `json:"dose_value" exclusiveMinimum:"0" doc:"What the patient took, which is what is recorded: the wizard's dose is editable, and a patient who steps down from what the course prescribes has done so. The prescription stays in the course, so the two can be compared."`
		DoseUnit  string  `json:"dose_unit" enum:"мг,мкг"`

		VialID    *string  `json:"vial_id,omitempty" format:"uuid" doc:"The vial it was drawn from. Absent when the picker was skipped, which costs that vial's count one dose — a truth about what is known."`
		Site      *string  `json:"site_code,omitempty" enum:"l-abdomen,r-abdomen,l-delt,r-delt,l-glute,r-glute,l-thigh,r-thigh,l-lback,r-lback" doc:"The body zone, one of the ten the map draws."`
		Mood      *int     `json:"mood,omitempty" minimum:"1" maximum:"5"`
		Sides     []string `json:"side_effects,omitempty" enum:"nausea,fatigue,headache,bloating,insomnia,site,appetite" doc:"The seven §03 names. The same set the diary's tags come from, because one action writes both rows."`
		Note      *string  `json:"note,omitempty" maxLength:"2000"`
		PhotoPath *string  `json:"photo_path,omitempty" doc:"The stored key of the photo, under the patient's own prefix."`

		ClientRequestID string `json:"client_request_id" minLength:"8" maxLength:"128" pattern:"^[A-Za-z0-9][A-Za-z0-9._:-]*$" doc:"The client's own key. A retry from the offline queue carries the key generated when the patient tapped save, and the repeat answers what the first answered."`
	}
}

// LogDoseOutput is the closed set of outcomes, with the identifiers the written one carries.
type LogDoseOutput struct {
	Body struct {
		Outcome string `json:"outcome" enum:"written,incomplete,not_scheduled_today,already_logged"`

		EventID     string  `json:"dose_event_id,omitempty" doc:"Present when the outcome is written."`
		JournalDate string  `json:"journal_date,omitempty" format:"date" doc:"The day whose diary entry this dose wrote. The entry has no id of its own — the day is its identity."`
		DoseValue   float64 `json:"dose_value,omitempty" doc:"Read back off the event: what was recorded as taken."`
		DoseUnit    string  `json:"dose_unit,omitempty"`
		VialID      *string `json:"vial_id,omitempty"`
	}
}

func (s *Service) log(ctx context.Context, in *LogDoseInput) (*LogDoseOutput, error) {
	if s.requests == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a request pool")
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}
	// A patient-only surface, refused rather than left to the policies. For a doctor the
	// answer would be a bland «not scheduled today»; for an admin every policy on all
	// seven tables this transaction touches is USING (true), so the subject Log fixes
	// would be the only boundary on every one of its nine statements. Every neighbouring
	// /v1/me surface refuses the same way.
	if principal.Role != "patient" {
		return nil, huma.Error403Forbidden("only a patient records their own doses")
	}

	draft, err := s.draft(in)
	if err != nil {
		return nil, err
	}

	logged, err := Log(ctx, s.requests,
		database.Caller{Subject: principal.Subject, Role: principal.Role}, s.now(), draft)
	if err != nil {
		return nil, answer(err)
	}

	out := &LogDoseOutput{}
	out.Body.Outcome = string(logged.Outcome)
	if logged.Outcome == Written {
		out.Body.EventID = logged.EventID
		out.Body.JournalDate = logged.JournalDate.String()
		out.Body.DoseValue = logged.Dose.Value
		out.Body.DoseUnit = string(logged.Dose.Unit)
		out.Body.VialID = logged.VialID
	}

	return out, nil
}

// draft is where a string becomes a member of a closed set. Over HTTP the schema's enum
// keyword refuses an unknown value first, so these parsers answer for the callers that reach
// this package without one — and for the day the keyword is dropped from a field.
func (s *Service) draft(in *LogDoseInput) (Draft, error) {
	unit, ok := protocol.ParseDoseUnit(in.Body.DoseUnit)
	if !ok {
		return Draft{}, huma.Error422UnprocessableEntity("dose_unit is not one this API knows: " + in.Body.DoseUnit)
	}

	draft := Draft{
		ItemID:          protocol.ProtocolItemID(in.Body.ItemID),
		Dose:            &protocol.Dose{Value: in.Body.DoseValue, Unit: unit},
		VialID:          in.Body.VialID,
		Mood:            in.Body.Mood,
		Note:            blankIsAbsent(in.Body.Note),
		PhotoPath:       in.Body.PhotoPath,
		ClientRequestID: in.Body.ClientRequestID,
	}

	if in.Body.Site != nil {
		site, ok := parseSite(*in.Body.Site)
		if !ok {
			return Draft{}, huma.Error422UnprocessableEntity("site_code is not a zone the body map draws: " + *in.Body.Site)
		}
		draft.Site = &site
	}

	for _, reported := range in.Body.Sides {
		side, ok := journal.ParseTag(reported)
		if !ok {
			return Draft{}, huma.Error422UnprocessableEntity("side_effects carries one this API does not know: " + reported)
		}
		draft.Sides = append(draft.Sides, side)
	}

	return draft, nil
}

// answer maps a refusal to a status. The four outcomes are not here: they are results, and
// they travel in the body with a 200.
func answer(err error) error {
	switch {
	case errors.Is(err, ErrNoTimezone):
		// A provisioning fault rather than a bad request: the patient's account is
		// missing something the clinic was meant to record.
		return huma.Error500InternalServerError("the patient's timezone is not recorded", err)
	case errors.Is(err, ErrRequestChanged):
		// A client error and not a repeat: the retry queue sends what it saved, so a
		// divergence means it saved the wrong thing, and the message names the field.
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, ErrNoSuchVial), errors.Is(err, ErrPhotoNotTheirs),
		errors.Is(err, ErrNoteSaysNothing):
		// The schema refuses both on every path; this is that refusal read back as the
		// field the caller filled in rather than as a constraint they cannot see.
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, journal.ErrAnotherPatientsDay), errors.Is(err, journal.ErrAnotherDay):
		// Programmer errors by that package's own account, and the record owed this
		// step a 500 for them: a 4xx would tell a patient their form is wrong about a
		// bug in this process, and put another patient's identifier in the reply.
		return huma.Error500InternalServerError("the day being merged is not the one being written", err)
	case database.IsUnavailable(err):
		return huma.Error503ServiceUnavailable("the database could not serve the request", err)
	default:
		return huma.Error500InternalServerError("recording the dose", err)
	}
}

// blankIsAbsent is journal.Merge's rule about a note, applied one layer earlier because this
// path reaches a column that does not share it: dose_events refuses a note of nothing, so a
// client serialising a cleared box as "" rather than omitting the field raised 23514 — an
// unclassified 500, forever, because the offline queue would keep re-sending it.
func blankIsAbsent(note *string) *string {
	if note == nil || strings.TrimSpace(*note) == "" {
		return nil
	}

	return note
}

// parseSite is the zones' own constructor, beside protocol's four and journal's two.
func parseSite(s string) (Site, bool) {
	for _, site := range Sites() {
		if site == Site(s) {
			return site, true
		}
	}

	return "", false
}

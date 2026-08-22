package protocol

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// Service is this context's operations together with what they need to answer.
//
// The service pool alone: prescribing is a cross-actor write, and cadence_doctor holds no
// INSERT on any of these tables. The request pool would answer nothing here.
type Service struct {
	service *pgxpool.Pool
}

func NewService(servicePool *pgxpool.Pool) *Service { return &Service{service: servicePool} }

// Register mounts this context's operations on the API.
func (s *Service) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-protocol",
		Method:        http.MethodPost,
		Path:          "/v1/patients/{patientId}/protocols",
		DefaultStatus: http.StatusCreated,
		Summary:       "Prescribe a course",
		Description: "Writes a course, its items and their titration phases in one transaction, " +
			"signed with an audit record. Only for a patient on the caller's own care team; an " +
			"admin prescribes for anyone. A drug is named by its directory identifier or by its " +
			"name — a name already in the directory resolves to the row it has, whatever its case " +
			"and spacing, rather than entering a second one. Phases of one item may leave gaps " +
			"between them and may not overlap. Answers 409 when the patient already has a course " +
			"running: a course is ended by completing or cancelling it, not by being replaced.",
		Tags: []string{"protocol"},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusForbidden,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.create)

	huma.Register(api, huma.Operation{
		OperationID: "replace-protocol",
		Method:      http.MethodPut,
		Path:        "/v1/patients/{patientId}/protocols/{protocolId}",
		Summary:     "Rewrite a course",
		Description: "Replaces a course's items and their phases, keeping the course and its " +
			"identifier. The items are replaced rather than patched — the form sends a list, not " +
			"a patch — except that an item the patient has already injected cannot be removed: " +
			"the doses that answered it are the clinic's record, and such an item is stopped by " +
			"marking it not loggable. Answers 404 for a course that is not this patient's, which " +
			"is the same answer an identifier nobody holds gets.",
		Tags: []string{"protocol"},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.replace)
}

// CourseInput is a prescription as the dashboard sends it.
type CourseInput struct {
	PatientID string `path:"patientId" format:"uuid" doc:"The patient the course is for."`
	Body      Course
}

// CourseUpdateInput carries the course being rewritten as well.
type CourseUpdateInput struct {
	PatientID  string `path:"patientId" format:"uuid" doc:"The patient the course is for."`
	ProtocolID string `path:"protocolId" format:"uuid" doc:"The course being rewritten."`
	Body       Course
}

// Course is the wire shape of a prescription.
//
// Dates and times are strings in their ISO forms rather than time.Time: a course starts on a
// day, and a day carried as an instant is a day that moves with the reader's timezone.
type Course struct {
	StartDate string  `json:"start_date" format:"date" doc:"The day the course begins, and the day every cycle week is counted from."`
	Weeks     int     `json:"weeks" minimum:"1" maximum:"104" doc:"How long the course runs."`
	Status    string  `json:"status" enum:"active,completed,cancelled" doc:"A patient has at most one active course at a time."`
	Notes     *string `json:"notes,omitempty" doc:"The prescriber's own note about the course."`
	Items     []Item  `json:"items" minItems:"1" doc:"What is prescribed. A course with nothing in it is refused."`
}

// Item is one prescribed thing: an injection, a supplement or a weigh-in.
type Item struct {
	Kind       string   `json:"kind" enum:"injection,supplement,weigh_in"`
	Compound   *Drug    `json:"compound,omitempty" doc:"What is injected. Required for an injection and refused for anything else — a weigh-in is not a prescription of a drug."`
	Cadence    string   `json:"cadence" enum:"weekly,daily,n_per_week"`
	DaysOfWeek []int    `json:"days_of_week" doc:"ISO weekdays, 1 for Monday. Empty for a daily item and at least one for any other, because the two are read together and a mismatch makes the schedule disagree with itself."`
	Times      []string `json:"times" format:"time" minItems:"1" doc:"The slots within the day. Two of them is two occurrences, logged apart."`
	Loggable   bool     `json:"loggable" doc:"Whether the patient records taking it. False for a supplement the clinic tracks without asking."`
	Phases     []Phase  `json:"phases" minItems:"1" doc:"The titration bands. They may leave gaps — a washout is deliberate — and may not overlap."`
}

// Drug names what is injected: an identifier from the directory, or a new entry.
type Drug struct {
	ID          *string `json:"id,omitempty" format:"uuid" doc:"A drug already in the directory."`
	NameRU      string  `json:"name_ru,omitempty" doc:"The drug's name, if it is not in the directory yet. A name already there resolves to that row whatever its case."`
	DefaultUnit string  `json:"default_unit,omitempty" enum:"мг,мкг"`
	Route       string  `json:"route,omitempty"`
	Icon        string  `json:"icon,omitempty"`
}

// Phase is one titration band: weeks 1..4 at 0,25 мг.
type Phase struct {
	FromWeek  int     `json:"from_week" minimum:"1"`
	ToWeek    int     `json:"to_week" minimum:"1"`
	DoseValue float64 `json:"dose_value" exclusiveMinimum:"0"`
	DoseUnit  string  `json:"dose_unit" enum:"мг,мкг"`
}

// CourseOutput carries the identifiers the database chose, in the order the items were sent,
// so a client can match its own form rows to them.
type CourseOutput struct {
	Body struct {
		ProtocolID string   `json:"protocol_id" doc:"The course."`
		ItemIDs    []string `json:"item_ids" doc:"One per item, in the order they were sent."`
	}
}

func (s *Service) create(ctx context.Context, in *CourseInput) (*CourseOutput, error) {
	if s.service == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a service pool")
	}

	draft, err := in.Body.draft(in.PatientID)
	if err != nil {
		return nil, err
	}

	written, err := Create(ctx, s.service, draft)
	if err != nil {
		return nil, answer(err)
	}

	return output(written), nil
}

func (s *Service) replace(ctx context.Context, in *CourseUpdateInput) (*CourseOutput, error) {
	if s.service == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a service pool")
	}

	draft, err := in.Body.draft(in.PatientID)
	if err != nil {
		return nil, err
	}

	written, err := Replace(ctx, s.service, ProtocolID(in.ProtocolID), draft)
	if err != nil {
		return nil, answer(err)
	}

	return output(written), nil
}

func output(written Written) *CourseOutput {
	out := &CourseOutput{}
	out.Body.ProtocolID = string(written.ProtocolID)
	out.Body.ItemIDs = make([]string, len(written.ItemIDs))
	for i, id := range written.ItemIDs {
		out.Body.ItemIDs[i] = string(id)
	}

	return out
}

// draft turns the wire shape into the domain one. The parsing is here and the rules are in
// Draft.Check: this is where a string stops being a string, and the schema's enum keywords
// are a courtesy to the client rather than the guard — a request that bypasses the generated
// client reaches Check either way.
func (c Course) draft(patientID string) (Draft, error) {
	start, err := time.Parse(time.DateOnly, c.StartDate)
	if err != nil {
		return Draft{}, huma.Error422UnprocessableEntity("start_date is not a date", err)
	}

	status, ok := ParseStatus(c.Status)
	if !ok {
		return Draft{}, huma.Error422UnprocessableEntity("status is not one this API knows: " + c.Status)
	}

	draft := Draft{
		PatientID: civil.UserID(patientID),
		StartDate: civil.NewDate(start.Year(), start.Month(), start.Day()),
		Weeks:     c.Weeks,
		Status:    status,
		Notes:     c.Notes,
		Items:     make([]DraftItem, 0, len(c.Items)),
	}

	for _, item := range c.Items {
		converted, err := item.draft()
		if err != nil {
			return Draft{}, err
		}
		draft.Items = append(draft.Items, converted)
	}

	return draft, nil
}

func (i Item) draft() (DraftItem, error) {
	kind, ok := ParseKind(i.Kind)
	if !ok {
		return DraftItem{}, huma.Error422UnprocessableEntity("kind is not one this API knows: " + i.Kind)
	}
	cadence, ok := ParseCadence(i.Cadence)
	if !ok {
		return DraftItem{}, huma.Error422UnprocessableEntity("cadence is not one this API knows: " + i.Cadence)
	}

	days := make([]time.Weekday, 0, len(i.DaysOfWeek))
	for _, iso := range i.DaysOfWeek {
		day, ok := civil.WeekdayFromISO(iso)
		if !ok {
			return DraftItem{}, huma.Error422UnprocessableEntity("days_of_week carries a day that is not 1..7")
		}
		days = append(days, day)
	}

	times := make([]civil.Slot, 0, len(i.Times))
	for _, at := range i.Times {
		slot, err := time.Parse("15:04", at)
		if err != nil {
			return DraftItem{}, huma.Error422UnprocessableEntity("times carries a value that is not a time: "+at, err)
		}
		times = append(times, civil.Slot{Hour: slot.Hour(), Minute: slot.Minute()})
	}

	phases := make([]ProtocolPhase, 0, len(i.Phases))
	for _, phase := range i.Phases {
		unit, ok := ParseDoseUnit(phase.DoseUnit)
		if !ok {
			return DraftItem{}, huma.Error422UnprocessableEntity("dose_unit is not one this API knows: " + phase.DoseUnit)
		}
		phases = append(phases, ProtocolPhase{
			FromWeek: phase.FromWeek,
			ToWeek:   phase.ToWeek,
			Dose:     Dose{Value: phase.DoseValue, Unit: unit},
		})
	}

	drafted := DraftItem{
		Kind:       kind,
		Cadence:    cadence,
		DaysOfWeek: days,
		Times:      times,
		Loggable:   i.Loggable,
		Phases:     phases,
	}
	if i.Compound != nil {
		drafted.Compound = i.Compound.ref()
	}

	return drafted, nil
}

func (d Drug) ref() CompoundRef {
	ref := CompoundRef{}
	if d.ID != nil {
		id := CompoundID(*d.ID)
		ref.ID = &id
	}
	if d.NameRU != "" {
		ref.New = &NewCompound{
			NameRU:      d.NameRU,
			DefaultUnit: DoseUnit(d.DefaultUnit),
			Route:       d.Route,
			Icon:        d.Icon,
		}
	}

	return ref
}

// answer maps a refusal to a status. Every named error of this context appears here, and
// anything unnamed is a 500 — a default that mapped the unknown onto 422 would tell a doctor
// their form is wrong about a bug in this process.
func answer(err error) error {
	switch {
	case errors.Is(err, ErrNotAPrescriber), errors.Is(err, ErrNotYourPatient):
		// The same answer for both, and deliberately: telling an unassigned doctor
		// that the patient exists is a fact about somebody else's care team.
		return huma.Error403Forbidden("this caller does not prescribe for this patient")
	case errors.Is(err, ErrMalformedIdentifier):
		return huma.Error422UnprocessableEntity("an identifier is not a UUID", err)
	case errors.Is(err, ErrNoSuchProtocol):
		return huma.Error404NotFound("no such course for this patient")
	case errors.Is(err, ErrAlreadyRunning):
		return huma.Error409Conflict("this patient already has a course running")
	case errors.Is(err, ErrItemHasBeenInjected):
		return huma.Error409Conflict("an item the patient has already injected cannot be removed; stop it by marking it not loggable", err)
	case errors.Is(err, ErrNoSuchCompound):
		return huma.Error422UnprocessableEntity("no such drug in the directory", err)
	case isAShapeRefusal(err):
		return huma.Error422UnprocessableEntity(err.Error())
	case database.IsUnavailable(err):
		return huma.Error503ServiceUnavailable("the database could not serve the request", err)
	default:
		return huma.Error500InternalServerError("writing the course", err)
	}
}

// isAShapeRefusal asks the list rather than a naming convention: an error added to shape.go
// and not added here would reach the doctor as a 500 about their own form.
func isAShapeRefusal(err error) bool {
	for _, refusal := range shapeRefusals() {
		if errors.Is(err, refusal) {
			return true
		}
	}

	return false
}

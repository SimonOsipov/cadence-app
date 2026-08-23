package protocol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// Service is this context's operations together with what they need to answer.
//
// The service pool alone: prescribing is a cross-actor write, and cadence_doctor holds no
// INSERT on any of these tables. The request pool would answer nothing here.
type Service struct {
	service *pgxpool.Pool

	// The request pool, and the three neighbours, for the reads: a patient asking about
	// their own day is answered under their own identity, so RLS is the boundary — unlike
	// the writes above it, which are cross-actor and go through the service seam.
	requests *pgxpool.Pool
	doses    Doses
	rotation Rotation
	cabinet  Cabinet

	// The clock, injected for the reason every day in this feature is: the summary is
	// about the patient's own day, and a handler reading time.Now cannot be run against
	// a calendar.
	now func() time.Time
}

// Deps is what this context's operations answer from. The three neighbours arrive as
// interfaces this package declared, which is what keeps the dependency running one way.
type Deps struct {
	ServicePool *pgxpool.Pool
	RequestPool *pgxpool.Pool
	Doses       Doses
	Rotation    Rotation
	Cabinet     Cabinet
}

// NewService takes the clock as an argument and not as a field of Deps, because a field is a
// thing a caller forgets: this wiring shipped without it for one commit, and all three reads
// answered 500 in the assembled application while every test in this package passed — each
// builds its own assembly. A positional parameter is the one form the compiler checks.
//
// It is the caller's and not this package's for the reason the generator's `today` parameter
// exists, and the guard in this package's own suite enforces: a file here that read the clock
// could not be run against a calendar.
func NewService(now func() time.Time, deps Deps) *Service {
	return &Service{
		service:  deps.ServicePool,
		requests: deps.RequestPool,
		doses:    deps.Doses,
		rotation: deps.Rotation,
		cabinet:  deps.Cabinet,
		now:      now,
	}
}

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

	s.registerReads(api)
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
	// Present when this item already exists: the reply to a create carries the
	// identifiers in order, and an editor sends back the ones it kept. An item named
	// this way is rewritten in place, which is what lets a course be titrated after a
	// dose has been logged against it.
	ID *string `json:"id,omitempty" format:"uuid" doc:"The item being kept, from an earlier reply. Absent for an item just added."`

	Kind       string `json:"kind" enum:"injection,supplement,weigh_in"`
	Compound   *Drug  `json:"compound,omitempty" doc:"What is injected. Required for an injection and refused for anything else — a weigh-in is not a prescription of a drug."`
	Cadence    string `json:"cadence" enum:"weekly,daily,n_per_week"`
	DaysOfWeek []int  `json:"days_of_week" doc:"ISO weekdays, 1 for Monday. Empty for a daily item and at least one for any other, because the two are read together and a mismatch makes the schedule disagree with itself."`
	// A pattern and not format:"time", which huma reads as RFC 3339 full-time. Measured
	// on huma v2.39.0: with the format keyword «08:00» is refused as not a time and
	// «08:00:00» is accepted here and then fails the parse below — no value passed both,
	// and the two operations answered nothing but 422. A slot is HH:MM in §03, in the
	// KMP's LocalTime and in the prototype, so the pattern is the contract.
	Times    []string `json:"times" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" minItems:"1" doc:"The slots within the day, as HH:MM. Two of them is two occurrences, logged apart."`
	Loggable bool     `json:"loggable" doc:"Whether the patient records taking it. False for a supplement the clinic tracks without asking."`
	Phases   []Phase  `json:"phases" minItems:"1" doc:"The titration bands. They may leave gaps — a washout is deliberate — and may not overlap."`
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
	if i.ID != nil {
		id := ProtocolItemID(*i.ID)
		drafted.ID = &id
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
	case errors.Is(err, ErrNoTimezone):
		// A provisioning fault rather than a bad request: the patient's account is
		// missing something the clinic was meant to record.
		return huma.Error500InternalServerError("the patient's timezone is not recorded", err)
	case errors.Is(err, ErrNotADay):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, ErrMalformedIdentifier):
		return huma.Error422UnprocessableEntity("an identifier is not a UUID", err)
	case errors.Is(err, ErrNoSuchProtocol):
		return huma.Error404NotFound("no such course for this patient")
	case errors.Is(err, ErrNoSuchItem):
		return huma.Error404NotFound("no such item in this course")
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

// The three reads of the schedule. All on the request seam: a patient asking about their own
// day is answered under their own identity, and RLS is the boundary rather than a Go
// predicate — unlike the writes above, which are cross-actor.
func (s *Service) registerReads(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-today",
		Method:      http.MethodGet,
		Path:        "/v1/me/today",
		Summary:     "The patient's day",
		Description: "Everything the hero screen draws: the day and the part of it in the " +
			"patient's own zone, the cycle week, the next injection still open with the " +
			"drug it names, the week's prescription strip, whether a dose was logged " +
			"today, the open vial's remaining doses, a reorder hint, the next titration " +
			"step and the injection zone to suggest. Fields of contexts that are not " +
			"built — nutrition and measurements — are null rather than zero: an absent " +
			"value is a dash on the screen, and a zero is a sentence about a " +
			"prescription that does not exist. A patient with no running course still " +
			"gets the day, the part of it and the suggested zone.",
		Tags:   []string{"protocol"},
		Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable},
	}, s.today)

	huma.Register(api, huma.Operation{
		OperationID: "get-schedule-month",
		Method:      http.MethodGet,
		Path:        "/v1/me/schedule",
		Summary:     "A month of the calendar",
		Description: "One cell per day of the month, every day of it — days prescribing " +
			"nothing included, because the calendar draws a month and the window's " +
			"arithmetic is the server's: it came from the patient's own zone. Each cell " +
			"carries the cycle week, whether an injection falls that day, whether " +
			"anything is still pending and whether everything is done. A day with " +
			"nothing prescribed and a day whose occurrences were all missed are both " +
			"neither pending nor done; the client tells them apart by opening the day.",
		Tags: []string{"protocol"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden,
			http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, s.month)

	huma.Register(api, huma.Operation{
		OperationID: "get-schedule-day",
		Method:      http.MethodGet,
		Path:        "/v1/me/schedule/day",
		Summary:     "The occurrences of one day",
		Description: "What the sheet that opens on a tap draws: every occurrence of that " +
			"day with its time, its dose and its state. Generated rather than stored — " +
			"there is no schedule table, so a course edited yesterday changes what " +
			"yesterday's day says about tomorrow.",
		Tags: []string{"protocol"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden,
			http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, s.day)
}

// TodayOutput is the hero screen.
type TodayOutput struct {
	Body TodayBody
}

type TodayBody struct {
	Date      string `json:"date" format:"date"`
	PartOfDay string `json:"part_of_day" enum:"night,morning,afternoon,evening"`
	CycleWeek *int   `json:"cycle_week,omitempty" doc:"Absent outside the course and for a cancelled one."`

	NextDose         *OccurrenceBody `json:"next_dose,omitempty"`
	NextDoseCompound *CompoundBody   `json:"next_dose_compound,omitempty"`
	SuggestedSite    string          `json:"suggested_site" enum:"l-abdomen,r-abdomen,l-delt,r-delt,l-glute,r-glute,l-thigh,r-thigh,l-lback,r-lback" doc:"Computed from what was logged, not frozen."`
	WeekProtocol     []RowBody       `json:"week_protocol"`
	DoseLoggedToday  bool            `json:"dose_logged_today"`

	VialDosesLeft *int         `json:"vial_doses_left,omitempty" doc:"Of the open vial. Absent when the patient holds none of that drug."`
	Reorder       *ReorderBody `json:"reorder,omitempty"`
	NextTitration *StepBody    `json:"next_titration,omitempty"`

	MealCount      *int        `json:"meal_count" doc:"Null until the nutrition context is built."`
	MealMacros     *MacrosBody `json:"meal_macros" doc:"Null until the nutrition context is built."`
	Targets        *MacrosBody `json:"targets" doc:"Null until the nutrition context is built."`
	WeightKG       *float64    `json:"weight_kg" doc:"Null until the measurements context is built."`
	WeightSeries   []float64   `json:"weight_series" doc:"Null until the measurements context is built."`
	TargetWeightKG *float64    `json:"target_weight_kg" doc:"Null until the measurements context is built."`
}

// MacrosBody is the wire's own, tagged: the domain struct has no tags, and using it here put
// PascalCase keys into an API that is snake_case everywhere else — the generated client got
// `{ Carbs: number; Fat: number }` alone among its types.
type MacrosBody struct {
	Kcal    int `json:"kcal"`
	Protein int `json:"protein"`
	Carbs   int `json:"carbs"`
	Fat     int `json:"fat"`
}

type OccurrenceBody struct {
	ItemID string    `json:"protocol_item_id"`
	Kind   string    `json:"kind"`
	Date   string    `json:"date" format:"date"`
	Time   string    `json:"time"`
	Dose   *DoseBody `json:"dose,omitempty"`
	Status string    `json:"status" enum:"done,pending,missed,scheduled"`
}

type DoseBody struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type CompoundBody struct {
	ID     string `json:"id"`
	NameRU string `json:"name_ru"`
	Unit   string `json:"default_unit"`
	Route  string `json:"route"`
	Icon   string `json:"icon"`
}

type RowBody struct {
	ItemID      string    `json:"protocol_item_id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Dose        *DoseBody `json:"dose,omitempty"`
	Times       []string  `json:"times"`
	TodayStatus *string   `json:"today_status,omitempty"`
	Loggable    bool      `json:"loggable"`
}

type ReorderBody struct {
	CompoundID string `json:"compound_id"`
	WeeksLeft  int    `json:"weeks_left"`
}

type StepBody struct {
	Week int      `json:"week"`
	On   string   `json:"on" format:"date"`
	From DoseBody `json:"from"`
	To   DoseBody `json:"to"`
}

// ScheduleInput takes the month as a string rather than two numbers: «2026-05» is one field
// the client cannot get half right, and a year without a month is a request that means
// nothing.
type ScheduleInput struct {
	Month string `query:"month" pattern:"^[0-9]{4}-(0[1-9]|1[0-2])$" doc:"The month to draw, as YYYY-MM. Defaults to the one the patient is in."`
}

type ScheduleOutput struct {
	Body struct {
		Days []DayBody `json:"days"`
	}
}

type DayBody struct {
	Date         string `json:"date" format:"date"`
	CycleWeek    *int   `json:"cycle_week,omitempty"`
	HasInjection bool   `json:"has_injection"`
	AnyPending   bool   `json:"any_pending"`
	AllDone      bool   `json:"all_done"`
}

type DayInput struct {
	Date string `query:"date" format:"date" doc:"The day to open. Defaults to the patient's today."`
}

type DayOutput struct {
	Body struct {
		Date        string           `json:"date" format:"date"`
		CycleWeek   *int             `json:"cycle_week,omitempty"`
		Occurrences []OccurrenceBody `json:"occurrences"`
	}
}

func (s *Service) today(ctx context.Context, _ *struct{}) (*TodayOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}

	var body TodayBody
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		at, clock, err := s.dayOf(ctx, tx, patient)
		if err != nil {
			return err
		}

		plan, running, err := ActivePlanFor(ctx, tx, patient)
		if err != nil {
			return err
		}
		compounds, err := CompoundsFor(ctx, tx, plan)
		if err != nil {
			return err
		}

		summary, err := TodayFor(ctx, tx, patient, plan, compounds, running, at, clock,
			s.doses, s.rotation, s.cabinet)
		if err != nil {
			return err
		}
		body = renderToday(summary)

		return nil
	}); err != nil {
		return nil, answer(err)
	}

	return &TodayOutput{Body: body}, nil
}

func (s *Service) month(ctx context.Context, in *ScheduleInput) (*ScheduleOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}

	out := &ScheduleOutput{}
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		at, _, err := s.dayOf(ctx, tx, patient)
		if err != nil {
			return err
		}

		window, ok := monthAsked(in.Month, at)
		if !ok {
			return fmt.Errorf("%q: %w", in.Month, ErrNotADay)
		}

		plan, running, err := ActivePlanFor(ctx, tx, patient)
		if err != nil {
			return err
		}

		var logged []LoggedSlot
		if running {
			if logged, err = s.doses.LoggedSlotsIn(ctx, tx, patient, window); err != nil {
				return err
			}
		}

		for _, day := range ScheduleFor(plan, logged, window, at) {
			out.Body.Days = append(out.Body.Days, DayBody{
				Date:         day.Date.String(),
				CycleWeek:    day.CycleWeek,
				HasInjection: day.HasInjection,
				AnyPending:   day.AnyPending,
				AllDone:      day.AllDone,
			})
		}

		return nil
	}); err != nil {
		return nil, answer(err)
	}

	return out, nil
}

func (s *Service) day(ctx context.Context, in *DayInput) (*DayOutput, error) {
	patient, caller, err := s.patient(ctx)
	if err != nil {
		return nil, err
	}

	out := &DayOutput{}
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		at, _, err := s.dayOf(ctx, tx, patient)
		if err != nil {
			return err
		}
		asked := at
		if in.Date != "" {
			parsed, err := time.Parse(time.DateOnly, in.Date)
			if err != nil {
				return fmt.Errorf("%q: %w", in.Date, ErrNotADay)
			}
			asked = civil.NewDate(parsed.Year(), parsed.Month(), parsed.Day())
		}

		plan, running, err := ActivePlanFor(ctx, tx, patient)
		if err != nil {
			return err
		}

		out.Body.Date = asked.String()
		// An empty list and not a null, the way the strip beside it answers: two list
		// fields of one step disagreeing about emptiness is a client branch per field.
		out.Body.Occurrences = []OccurrenceBody{}
		if !running {
			return nil
		}
		if week, inCourse := CycleWeek(plan.Protocol, asked); inCourse {
			out.Body.CycleWeek = &week
		}

		oneDay, ok := civil.NewRange(asked, asked)
		if !ok {
			return fmt.Errorf("%v: %w", asked, ErrNotADay)
		}
		logged, err := s.doses.LoggedSlotsIn(ctx, tx, patient, oneDay)
		if err != nil {
			return err
		}

		for _, occurrence := range OccurrencesFor(plan, logged, asked, at) {
			out.Body.Occurrences = append(out.Body.Occurrences, renderOccurrence(occurrence))
		}

		return nil
	}); err != nil {
		return nil, answer(err)
	}

	return out, nil
}

// patient is the caller, refused unless they are one. Every neighbouring /v1/me surface
// refuses the same way, and here it matters twice: an admin's policies are USING (true) on
// every table these reads touch, so the caller's own subject would be the only boundary.
func (s *Service) patient(ctx context.Context) (civil.UserID, database.Caller, error) {
	if s.requests == nil || s.now == nil || s.doses == nil || s.rotation == nil || s.cabinet == nil {
		return "", database.Caller{},
			huma.Error500InternalServerError("this API was assembled without its reads")
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return "", database.Caller{},
			huma.Error401Unauthorized("no verified principal on the request context")
	}
	if principal.Role != "patient" {
		return "", database.Caller{}, huma.Error403Forbidden("this is a patient's own day")
	}

	subject := strings.ToLower(principal.Subject)

	return civil.UserID(subject), database.Caller{Subject: subject, Role: principal.Role}, nil
}

// dayOf is the patient's own day and the hour within it. There is no default zone: every
// occurrence is generated in the patient's day, so the server's would answer about a
// different one for half a clinic.
func (s *Service) dayOf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID,
) (civil.Date, civil.Slot, error) {
	var zone *string
	if err := tx.QueryRow(ctx,
		`SELECT timezone FROM app.profiles WHERE user_id = $1`, string(patient)).Scan(&zone); err != nil {
		return civil.Date{}, civil.Slot{}, fmt.Errorf("reading the timezone of %s: %w", patient, err)
	}
	if zone == nil || *zone == "" {
		return civil.Date{}, civil.Slot{}, fmt.Errorf("%s: %w", patient, ErrNoTimezone)
	}

	where, err := time.LoadLocation(*zone)
	if err != nil {
		return civil.Date{}, civil.Slot{}, fmt.Errorf("the zone %q of %s: %w", *zone, patient, err)
	}

	local := s.now().In(where)

	return civil.NewDate(local.Year(), local.Month(), local.Day()),
		civil.Slot{Hour: local.Hour(), Minute: local.Minute()}, nil
}

// monthAsked is the window a month parameter means, or the patient's own month when it is
// absent — the client opening the calendar has not chosen one yet.
func monthAsked(asked string, at civil.Date) (civil.Range, bool) {
	if asked == "" {
		return civil.MonthOf(at.Year, at.Month)
	}

	parsed, err := time.Parse("2006-01", asked)
	if err != nil {
		return civil.Range{}, false
	}

	return civil.MonthOf(parsed.Year(), parsed.Month())
}

func renderToday(summary Today) TodayBody {
	body := TodayBody{
		Date:            summary.Date.String(),
		PartOfDay:       string(summary.PartOfDay),
		CycleWeek:       summary.CycleWeek,
		SuggestedSite:   summary.SuggestedSite,
		DoseLoggedToday: summary.DoseLoggedToday,
		VialDosesLeft:   summary.VialDosesLeft,
		WeekProtocol:    make([]RowBody, 0, len(summary.WeekProtocol)),
	}

	if summary.NextDose != nil {
		occurrence := renderOccurrence(*summary.NextDose)
		body.NextDose = &occurrence
	}
	if summary.NextDoseCompound != nil {
		body.NextDoseCompound = &CompoundBody{
			ID: string(summary.NextDoseCompound.ID), NameRU: summary.NextDoseCompound.NameRU,
			Unit:  string(summary.NextDoseCompound.DefaultUnit),
			Route: summary.NextDoseCompound.Route, Icon: summary.NextDoseCompound.Icon,
		}
	}
	if summary.Reorder != nil {
		body.Reorder = &ReorderBody{
			CompoundID: string(summary.Reorder.CompoundID), WeeksLeft: summary.Reorder.WeeksLeft,
		}
	}
	if summary.NextTitration != nil {
		body.NextTitration = &StepBody{
			Week: summary.NextTitration.Week, On: summary.NextTitration.Date.String(),
			From: DoseBody{summary.NextTitration.From.Value, string(summary.NextTitration.From.Unit)},
			To:   DoseBody{summary.NextTitration.To.Value, string(summary.NextTitration.To.Unit)},
		}
	}

	for _, row := range summary.WeekProtocol {
		rendered := RowBody{
			ItemID:   string(row.ItemID),
			Kind:     string(row.Kind),
			Times:    make([]string, 0, len(row.Times)),
			Loggable: row.Loggable,
		}
		if row.Compound != nil {
			rendered.Title = row.Compound.NameRU
		}
		if row.Dose != nil {
			rendered.Dose = &DoseBody{row.Dose.Value, string(row.Dose.Unit)}
		}
		for _, at := range row.Times {
			rendered.Times = append(rendered.Times, at.String())
		}
		if row.TodayStatus != nil {
			status := string(*row.TodayStatus)
			rendered.TodayStatus = &status
		}
		body.WeekProtocol = append(body.WeekProtocol, rendered)
	}

	return body
}

func renderOccurrence(occurrence Occurrence) OccurrenceBody {
	body := OccurrenceBody{
		ItemID: string(occurrence.ItemID),
		Kind:   string(occurrence.Kind),
		Date:   occurrence.Date.String(),
		Time:   occurrence.Time.String(),
		Status: string(occurrence.Status),
	}
	if occurrence.Dose != nil {
		body.Dose = &DoseBody{occurrence.Dose.Value, string(occurrence.Dose.Unit)}
	}

	return body
}

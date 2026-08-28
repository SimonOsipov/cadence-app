package inventory

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// ErrNoVial is «not here», «not yours» and «thrown away by somebody else» in one: which of
// them it was is a fact about a cabinet the caller cannot see.
var ErrNoVial = errors.New("no vial is readable here")

// CabinetInput takes nothing: the cabinet is the caller's own.
type CabinetInput struct{}

type CabinetOutput struct {
	Body CabinetBody
}

// CabinetBody is the shelf and the hints together, because they are one read: a hint is about
// the vials in the same answer, and a client assembling it from two calls would show a
// «buy more» card over a cabinet that has already been refilled.
type CabinetBody struct {
	// Live vials only — one thrown away is history, and it is still readable one by one.
	Vials []VialBody `json:"vials" nullable:"false"`
	// One per prescribed compound, not one per course item: two items of a drug produce one
	// hint about one cabinet.
	Reorder []VialReorderBody `json:"reorder" nullable:"false"`
}

type VialInput struct {
	VialID string `path:"vialId" format:"uuid"`
}

type VialOutput struct {
	Body VialBody
}

// VialBody is one vial as the cabinet answers it. Nothing here is stored but the columns:
// remaining substance, the count of injections left, and the status are computed on read.
type VialBody struct {
	ID                 string `json:"id" format:"uuid"`
	CompoundID         string `json:"compound_id" format:"uuid"`
	ConcentrationLabel string `json:"concentration_label" doc:"What the box says, transcribed. Nothing computes with it."`

	TotalAmount VialAmountBody `json:"total_amount"`
	// Substance, not injections: it answers without a course, and the count below does not.
	RemainingAmount VialAmountBody `json:"remaining_amount"`
	DosesLeft       *int           `json:"doses_left,omitempty" doc:"Absent where no running course prescribes this drug: a count needs a dose to divide by, and inventing one would put a number on the screen nobody prescribed."`
	CurrentDose     *VialDoseBody  `json:"current_dose,omitempty" doc:"The dose in force for this drug today. Absent for the same reasons as doses_left, and absent where two course items name the drug."`

	Status     string  `json:"status" enum:"disposed,expiring,sealed,low,active"`
	OpenedAt   *string `json:"opened_at,omitempty" format:"date" doc:"Absent while the vial is sealed; that absence is the whole of «sealed»."`
	ExpiresOn  string  `json:"expires_on" format:"date"`
	HeldBackAt *string `json:"held_back_at,omitempty" format:"date" doc:"The day the patient set it aside. A held-back vial is chosen by nothing the server decides for them."`
	DisposedAt *string `json:"disposed_at,omitempty" format:"date"`
	Lot        *string `json:"lot,omitempty"`
	LocationRU *string `json:"location_ru,omitempty"`

	// The flag and never the key: the key is an object-store path, and a client holding one
	// would be a client that could ask for somebody else's. The link is minted per read by
	// GET /v1/me/vials/{vialId}/label-photo.
	HasLabelPhoto bool `json:"has_label_photo"`
}

// VialAmountBody is a quantity in the unit the clinic wrote on the box. The arithmetic behind
// it is whole micrograms; this is that number carried back into its own unit.
type VialAmountBody struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit" enum:"мг,мкг"`
}

type VialDoseBody struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit" enum:"мг,мкг"`
}

type VialReorderBody struct {
	CompoundID string `json:"compound_id" format:"uuid"`
	WeeksLeft  int    `json:"weeks_left"`
}

func (s *Service) registerReads(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "read-cabinet",
		Method:      http.MethodGet,
		Path:        "/v1/me/vials",
		Summary:     "The patient's cabinet",
		Description: "Every vial the patient has not thrown away, with what is left in it, " +
			"how many injections that buys at today's dose, and the status the screen " +
			"groups by. None of those three is stored. The reorder hints travel with the " +
			"shelf they are about.",
		Tags:   []string{"inventory"},
		Errors: readErrors,
	}, s.readCabinet)

	huma.Register(api, huma.Operation{
		OperationID: "read-vial",
		Method:      http.MethodGet,
		Path:        "/v1/me/vials/{vialId}",
		Summary:     "One vial, including one thrown away",
		Description: "The same computed fields the cabinet answers, for a single vial — a " +
			"disposed one included, because its card is the history of what was drawn " +
			"from it. A vial that is not here and one that is not the caller's are one 404.",
		Tags:   []string{"inventory"},
		Errors: readErrors,
	}, s.readVial)
}

var readErrors = []int{
	http.StatusUnauthorized,
	http.StatusForbidden,
	http.StatusNotFound,
	http.StatusUnprocessableEntity,
	http.StatusServiceUnavailable,
}

func (s *Service) readCabinet(ctx context.Context, _ *CabinetInput) (*CabinetOutput, error) {
	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}

	out := &CabinetOutput{}
	out.Body.Vials, out.Body.Reorder = []VialBody{}, []VialReorderBody{}
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		shelf, err := s.shelfOf(ctx, tx, civil.UserID(caller.Subject))
		if err != nil {
			return err
		}
		for _, vial := range shelf.cabinet.vials {
			if vial.DisposedAt != nil {
				continue
			}
			out.Body.Vials = append(out.Body.Vials, shelf.render(vial))
		}
		out.Body.Reorder = shelf.hints()

		return nil
	}); err != nil {
		return nil, answerRead(err)
	}

	return out, nil
}

func (s *Service) readVial(ctx context.Context, in *VialInput) (*VialOutput, error) {
	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}

	out := &VialOutput{}
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		shelf, err := s.shelfOf(ctx, tx, civil.UserID(caller.Subject))
		if err != nil {
			return err
		}
		for _, vial := range shelf.cabinet.vials {
			if string(vial.ID) == in.VialID {
				out.Body = shelf.render(vial)

				return nil
			}
		}

		return ErrNoVial
	}); err != nil {
		return nil, answerRead(err)
	}

	return out, nil
}

// shelf is one read of everything both answers are computed from, so a cabinet and a card of
// the same vial cannot disagree about how much is left in it.
type shelf struct {
	cabinet Cabinet
	draws   []Draw
	plan    protocol.Plan
	today   civil.Date
}

func (s *Service) shelfOf(ctx context.Context, tx pgx.Tx, patient civil.UserID) (shelf, error) {
	today, _, err := protocol.DayOf(ctx, tx, patient, s.now())
	if err != nil {
		return shelf{}, err
	}
	vials, err := vialsOf(ctx, tx, patient)
	if err != nil {
		return shelf{}, err
	}
	draws, err := drawsOf(ctx, tx, patient)
	if err != nil {
		return shelf{}, err
	}
	// The bool is dropped on purpose: it says a course was found, and the zero Plan it
	// comes with answers every question here the way «no course» should.
	plan, _, err := protocol.ActivePlanFor(ctx, tx, patient)
	if err != nil {
		return shelf{}, err
	}

	return shelf{cabinet: CabinetOf(patient, vials), draws: draws, plan: plan, today: today}, nil
}

func (s shelf) render(vial Vial) VialBody {
	body := VialBody{
		ID:                 string(vial.ID),
		CompoundID:         string(vial.CompoundID),
		ConcentrationLabel: vial.ConcentrationLabel,
		TotalAmount:        amountBody(vial.TotalAmount, vial.AmountUnit),
		RemainingAmount:    amountBody(RemainingAmount(vial, s.draws), vial.AmountUnit),
		Status:             string(StatusOf(vial, s.draws, s.today)),
		OpenedAt:           dayString(vial.OpenedAt),
		ExpiresOn:          vial.ExpiresOn.String(),
		HeldBackAt:         dayString(vial.HeldBackAt),
		DisposedAt:         dayString(vial.DisposedAt),
		Lot:                vial.Lot,
		LocationRU:         vial.LocationRU,
		HasLabelPhoto:      vial.LabelPhotoPath != nil && *vial.LabelPhotoPath != "",
	}

	if dose := s.doseOf(vial.CompoundID); dose != nil {
		body.CurrentDose = &VialDoseBody{Value: dose.Value, Unit: string(dose.Unit)}
		body.DosesLeft = RemainingDoses(vial, s.draws, dose)
	}

	return body
}

// doseOf is nothing without a running course, and needs no guard saying so: ActivePlanFor
// selects on status = 'active', so a patient with none holds the zero Plan, which names no
// item — and a compound no item names has no dose in force. Measured: a guard here dies to no
// test, because there is no state in which it answers differently.
func (s shelf) doseOf(compound protocol.CompoundID) *protocol.Dose {
	return protocol.CurrentDoseFor(s.plan, compound, s.today)
}

// hints is one per compound and not one per item: DosesPerWeek is an item's, so the hint is
// asked per item, and two items of one drug would otherwise answer twice about one shelf.
func (s shelf) hints() []VialReorderBody {
	out := []VialReorderBody{}
	seen := map[protocol.CompoundID]bool{}
	for _, item := range s.plan.Items {
		if item.CompoundID == nil || seen[*item.CompoundID] {
			continue
		}
		hint := ReorderHintFor(item, s.cabinet, s.draws, protocol.PhaseDose(s.plan, item.ID, s.today), s.today)
		if hint == nil {
			continue
		}
		seen[*item.CompoundID] = true
		out = append(out, VialReorderBody{
			CompoundID: string(hint.CompoundID), WeeksLeft: hint.WeeksLeft,
		})
	}

	return out
}

func amountBody(amount Amount, unit protocol.DoseUnit) VialAmountBody {
	return VialAmountBody{Value: AmountIn(amount, unit), Unit: string(unit)}
}

func dayString(day *civil.Date) *string {
	if day == nil {
		return nil
	}
	text := day.String()

	return &text
}

func (s *Service) patientCalling(ctx context.Context) (database.Caller, error) {
	if s.requests == nil {
		return database.Caller{}, huma.Error500InternalServerError(
			"this API was assembled without a request pool",
		)
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return database.Caller{}, huma.Error401Unauthorized("no verified principal on the request context")
	}
	// A patient-only surface, for the reason readLabelPhoto records: a doctor reads the
	// cabinet through the policies the day the dashboard draws one, and admitting them
	// now publishes a surface nobody asked for.
	if principal.Role != "patient" {
		return database.Caller{}, huma.Error403Forbidden("only a patient reads their own cabinet")
	}

	return database.Caller{Subject: principal.Subject, Role: principal.Role}, nil
}

func answerRead(err error) error {
	switch {
	case errors.Is(err, ErrNoVial):
		return huma.Error404NotFound("no vial is readable here")
	case errors.Is(err, protocol.ErrNoTimezone):
		// The clinic owes this patient a zone, and every computed field here is a
		// question about their day: answering in the server's would be a remaining
		// count off by a day for half a cabinet.
		return huma.Error500InternalServerError("the patient's timezone is not recorded", err)
	case database.IsUnavailable(err):
		return huma.Error503ServiceUnavailable("the database is not answering", err)
	default:
		return huma.Error500InternalServerError("reading the cabinet", err)
	}
}

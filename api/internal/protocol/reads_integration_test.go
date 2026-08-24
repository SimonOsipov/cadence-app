//go:build integration

package protocol_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The three reads through the registered handler, with the real neighbours behind the
// interfaces — which is the half the stubs cannot measure: that dosing and inventory answer
// the questions this context declared, and that the wiring hands them over.
//
// 10 May 2026 is a Sunday, the day the seeded weekly injection falls on; 09:15 in
// Yekaterinburg, where the seeded patients live, is morning.
var theHour = time.Date(2026, time.May, 10, 4, 15, 0, 0, time.UTC)

func get(t *testing.T, service, requests *pgxpool.Pool, subject, role, path string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: role})))
		})
	})
	history := dosing.NewHistory(func() time.Time { return theHour })
	protocol.NewService(func() time.Time { return theHour }, protocol.Deps{
		ServicePool: service,
		RequestPool: requests,
		Doses:       history,
		Rotation:    history,
		Cabinet:     inventory.NewSupply(),
	}).Register(httpserver.NewAPI(mux))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec.Code, rec.Body.String()
}

func getUnverified(t *testing.T, service, requests *pgxpool.Pool, path string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	history := dosing.NewHistory(func() time.Time { return theHour })
	protocol.NewService(func() time.Time { return theHour }, protocol.Deps{
		ServicePool: service,
		RequestPool: requests,
		Doses:       history,
		Rotation:    history,
		Cabinet:     inventory.NewSupply(),
	}).Register(httpserver.NewAPI(mux))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec.Code, rec.Body.String()
}

func TestThePatientsDayIsAnsweredThroughTheTransport(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	status, body := get(t, service, requests, writePatientA, "patient", "/v1/me/today")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var today struct {
		Date          string   `json:"date"`
		PartOfDay     string   `json:"part_of_day"`
		CycleWeek     *int     `json:"cycle_week"`
		SuggestedSite string   `json:"suggested_site"`
		MealCount     *int     `json:"meal_count"`
		WeightKG      *float64 `json:"weight_kg"`
		NextDose      *struct {
			Kind   string `json:"kind"`
			Time   string `json:"time"`
			Status string `json:"status"`
		} `json:"next_dose"`
		WeekProtocol []struct {
			Kind     string `json:"kind"`
			Cadence  string `json:"cadence"`
			Compound *struct {
				NameRU string `json:"name_ru"`
				Route  string `json:"route"`
				Icon   string `json:"icon"`
			} `json:"compound"`
		} `json:"week_protocol"`
	}
	if err := json.Unmarshal([]byte(body), &today); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	if today.Date != "2026-05-10" || today.PartOfDay != "morning" {
		t.Errorf("the day reads %q %q", today.Date, today.PartOfDay)
	}
	if today.CycleWeek == nil || *today.CycleWeek != 1 {
		t.Errorf("the week is %v", today.CycleWeek)
	}
	// The zone comes from dosing through the interface this context declared, and the
	// patient has logged nothing — so it is the set's first, which is the rotation's own
	// rule for an empty history.
	if today.SuggestedSite != "l-abdomen" {
		t.Errorf("the suggested zone is %q", today.SuggestedSite)
	}
	if today.NextDose == nil || today.NextDose.Kind != "injection" || today.NextDose.Time != "08:00" {
		t.Errorf("the next dose reads %+v", today.NextDose)
	}
	// ProtocolStrip draws compound.icon and compound.name_ru with route · cadence under
	// them: a row carrying only a name renders as a beaker with no second line.
	if len(today.WeekProtocol) != 1 {
		t.Fatalf("the strip holds %d rows", len(today.WeekProtocol))
	}
	row := today.WeekProtocol[0]
	if row.Kind != "injection" || row.Cadence != "weekly" {
		t.Errorf("the row reads %q %q", row.Kind, row.Cadence)
	}
	if row.Compound == nil {
		t.Fatal("the row names no drug")
	}
	if row.Compound.NameRU != "Семаглутид" || row.Compound.Route != "sc" ||
		row.Compound.Icon != "syringe" {
		t.Errorf("the row's drug reads %+v", *row.Compound)
	}
	// Null and not zero — and null and not *absent*, which a pointer cannot tell apart:
	// adding omitempty to those fields would keep a nil check green while breaking the
	// criterion this asserts. The raw body is what says which of the three it is.
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("reading the raw reply: %v", err)
	}
	for _, absent := range []string{
		"meal_count", "meal_macros", "targets", "weight_kg", "weight_series", "target_weight_kg",
	} {
		value, present := raw[absent]
		if !present {
			t.Errorf("%s is missing from the reply rather than null", absent)
		}
		if value != nil {
			t.Errorf("%s answered %v, want null", absent, value)
		}
	}
}

// The cabinet's half of the answer, which nothing read: replacing SupplyFor's whole body with
// three nils kept the gate green, because the fixture seeded no vial and the reply's two
// numbers were never asserted. «Остаток флакона нигде не хранится: он считается как
// total_doses минус число событий» is an acceptance criterion of this feature, and this is
// the path it is answered on.
func TestTheOpenVialsRemainingDosesAreOnTheDay(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}
	openAVial(t, service, requests, writePatientA, 6)

	left, reorder := supplyOn(t, service, requests, writePatientA)

	// Six doses in the vial and none drawn: the count is a subtraction, and there is no
	// column it could have come from.
	if left == nil || *left != 6 {
		t.Errorf("the open vial has %v doses left, want six", shown(left))
	}
	// Weekly injection, six doses: six weeks of supply, past the four-week threshold, so
	// no hint. The pair is what says the number reached the rule rather than a constant.
	if reorder != nil {
		t.Errorf("a reorder hint at six weeks of supply: %+v", reorder)
	}

	drawDoses(t, service, requests, writePatientA, 3)

	left, reorder = supplyOn(t, service, requests, writePatientA)

	if left == nil || *left != 3 {
		t.Errorf("after three doses the vial has %v left, want three", shown(left))
	}
	if reorder == nil || reorder.WeeksLeft != 3 {
		t.Errorf("three weeks of supply answered %+v, want a hint", reorder)
	}

	// A second vial opened later: with two open vials of one drug the answer is the
	// earliest-opened's, which is what the read's ORDER BY decides. Without it the row that
	// came back first won, and two requests for the same day could differ.
	openAVialOn(t, service, requests, writePatientA, "Семаглутид", 9,
		civil.NewDate(2026, time.May, 8))

	left, _ = supplyOn(t, service, requests, writePatientA)

	if left == nil || *left != 3 {
		t.Errorf("with two vials open the count is %v, want three", shown(left))
	}

	// Thrown away and not sealed: a sealed 9-dose vial stays in the cabinet, and the
	// arithmetic below would then read 3+9+1 = thirteen weeks of supply and answer no hint
	// for that reason instead of the rule under test. Unlike sealing, disposal adds no spare.
	disposeAVial(t, requests, writePatientA, 9)

	// One dose in the spare, deliberately: a large spare would silence the hint through
	// the weeks arithmetic instead, and the rule under test — «0 sealed spares» — would be
	// unmeasured. Three doses plus this one is four weeks, which is inside the threshold.
	openAVial(t, service, requests, writePatientA, 1)
	sealASpare(t, requests, writePatientA, 1)

	left, reorder = supplyOn(t, service, requests, writePatientA)

	// A sealed spare is not «doses left» — counting it is the prototype's own bug — and it
	// is what silences the hint: «0 sealed spares & ≤4 weeks supply» is one condition.
	if left == nil || *left != 3 {
		t.Errorf("a sealed spare moved the count to %v", shown(left))
	}
	if reorder != nil {
		t.Errorf("a hint with a spare on the shelf: %+v", reorder)
	}

	sealASpare(t, requests, writePatientA, 6)

	left, _ = supplyOn(t, service, requests, writePatientA)

	// Nothing open: the count is absent rather than the sealed stock's, which is the whole
	// of «the open vial's remaining doses».
	if left != nil {
		t.Errorf("a cabinet with nothing open answered %v", *left)
	}
}

// shown exists because %v over a *int prints its address, not the count.
func shown(count *int) any {
	if count == nil {
		return "nothing"
	}

	return *count
}

func supplyOn(t *testing.T, service, requests *pgxpool.Pool, patient string) (*int, *protocol.ReorderHint) {
	t.Helper()

	_, body := get(t, service, requests, patient, "patient", "/v1/me/today")

	var today struct {
		VialDosesLeft *int `json:"vial_doses_left"`
		Reorder       *struct {
			CompoundID string `json:"compound_id"`
			WeeksLeft  int    `json:"weeks_left"`
		} `json:"reorder"`
	}
	if err := json.Unmarshal([]byte(body), &today); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}
	if today.Reorder == nil {
		return today.VialDosesLeft, nil
	}

	return today.VialDosesLeft, &protocol.ReorderHint{
		CompoundID: protocol.CompoundID(today.Reorder.CompoundID),
		WeeksLeft:  today.Reorder.WeeksLeft,
	}
}

func TestAMonthOfTheCalendarIsAnsweredThroughTheTransport(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	status, body := get(t, service, requests, writePatientA, "patient", "/v1/me/schedule?month=2026-05")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var month struct {
		Days []struct {
			Date         string `json:"date"`
			HasInjection bool   `json:"has_injection"`
			AnyPending   bool   `json:"any_pending"`
		} `json:"days"`
	}
	if err := json.Unmarshal([]byte(body), &month); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	if len(month.Days) != 31 {
		t.Fatalf("May is %d days", len(month.Days))
	}
	if month.Days[0].Date != "2026-05-01" || month.Days[30].Date != "2026-05-31" {
		t.Errorf("the month runs %s..%s", month.Days[0].Date, month.Days[30].Date)
	}

	// The Sundays of May 2026 are the 3rd, 10th, 17th, 24th and 31st, and the course
	// starts on the 4th — so the 3rd carries no injection and the rest do.
	for _, day := range month.Days {
		sunday := day.Date == "2026-05-10" || day.Date == "2026-05-17" ||
			day.Date == "2026-05-24" || day.Date == "2026-05-31"
		if day.HasInjection != sunday {
			t.Errorf("%s: injection %v, want %v", day.Date, day.HasInjection, sunday)
		}
	}

	// A month the patient's course does not reach is every day and no dots, rather than
	// an empty list: the calendar draws a month whatever is in it.
	_, next := get(t, service, requests, writePatientA, "patient", "/v1/me/schedule?month=2027-01")
	var later struct {
		Days []struct {
			HasInjection bool `json:"has_injection"`
		} `json:"days"`
	}
	if err := json.Unmarshal([]byte(next), &later); err != nil {
		t.Fatalf("reading the later month: %v", err)
	}
	if len(later.Days) != 31 {
		t.Errorf("January is %d days", len(later.Days))
	}
	for _, day := range later.Days {
		if day.HasInjection {
			t.Error("a month past the course carries an injection")
		}
	}
}

func TestOneDayOfTheCalendarIsAnsweredThroughTheTransport(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	status, body := get(t, service, requests, writePatientA, "patient",
		"/v1/me/schedule/day?date=2026-05-17")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var day struct {
		Date        string `json:"date"`
		CycleWeek   *int   `json:"cycle_week"`
		Occurrences []struct {
			Kind   string `json:"kind"`
			Time   string `json:"time"`
			Status string `json:"status"`
			Dose   *struct {
				Value float64 `json:"value"`
				Unit  string  `json:"unit"`
			} `json:"dose"`
		} `json:"occurrences"`
	}
	if err := json.Unmarshal([]byte(body), &day); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	if day.Date != "2026-05-17" || day.CycleWeek == nil || *day.CycleWeek != 2 {
		t.Errorf("the day reads %q week %v", day.Date, day.CycleWeek)
	}
	if len(day.Occurrences) != 1 {
		t.Fatalf("the day holds %d occurrences", len(day.Occurrences))
	}
	// A week ahead of the patient's today, so it is scheduled rather than pending.
	if day.Occurrences[0].Status != "scheduled" || day.Occurrences[0].Time != "08:00" {
		t.Errorf("the occurrence reads %+v", day.Occurrences[0])
	}
	if day.Occurrences[0].Dose == nil || day.Occurrences[0].Dose.Value != 0.25 {
		t.Errorf("the dose reads %+v", day.Occurrences[0].Dose)
	}
}

// A patient's own day, and nobody else's: every one of these reads runs under the caller's
// identity, so a doctor asking gets a refusal rather than a patient's screen. The admin row
// is the one that matters — every policy these reads touch is USING (true) for cadence_admin,
// so the refusal here is the whole of the boundary for that role.
func TestTheReadsAreAPatientsOwn(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	for _, path := range []string{"/v1/me/today", "/v1/me/schedule", "/v1/me/schedule/day"} {
		for _, role := range []string{"doctor", "admin", "", "PATIENT"} {
			if status, body := get(t, service, requests, writeDoctorA, role, path); status != http.StatusForbidden {
				t.Errorf("%s as %q answered %d: %s", path, role, status, body)
			}
		}
		if status, body := getUnverified(t, service, requests, path); status != http.StatusUnauthorized {
			t.Errorf("%s with no principal answered %d: %s", path, status, body)
		}
	}
}

// The two parameters a client can get wrong. Both are parsed by hand behind huma's own
// validation, and this project has already shipped a pair of endpoints where the two
// disagreed and nothing but 422 ever came back — so what is asserted is that the malformed
// value is refused *and* that the well-formed one is not.
func TestAMalformedWindowIsRefusedRatherThanCrashing(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	for _, asked := range []struct {
		path string
		want int
	}{
		{"/v1/me/schedule?month=2026-13", http.StatusUnprocessableEntity},
		{"/v1/me/schedule?month=мая", http.StatusUnprocessableEntity},
		{"/v1/me/schedule?month=2026-05", http.StatusOK},
		{"/v1/me/schedule", http.StatusOK},
		{"/v1/me/schedule/day?date=2026-02-30", http.StatusUnprocessableEntity},
		{"/v1/me/schedule/day?date=17.05.2026", http.StatusUnprocessableEntity},
		{"/v1/me/schedule/day?date=2026-05-17", http.StatusOK},
		{"/v1/me/schedule/day", http.StatusOK},
	} {
		if status, body := get(
			t, service, requests, writePatientA, "patient", asked.path,
		); status != asked.want {
			t.Errorf("%s answered %d, want %d: %s", asked.path, status, asked.want, body)
		}
	}
}

// openAVial gives the patient an opened vial of the seeded compound. As the patient, because
// the service path holds no UPDATE on vials — opening one is their own act.
func openAVial(t *testing.T, service, requests *pgxpool.Pool, patient string, total int) {
	t.Helper()

	openAVialOn(t, service, requests, patient, "Семаглутид", total, civil.NewDate(2026, time.May, 1))
}

func openAVialOf(
	t *testing.T, service, requests *pgxpool.Pool, patient, drug string, total int,
) {
	t.Helper()

	openAVialOn(t, service, requests, patient, drug, total, civil.NewDate(2026, time.May, 1))
}

func openAVialOn(
	t *testing.T, service, requests *pgxpool.Pool, patient, drug string, total int,
	opened civil.Date,
) {
	t.Helper()

	var compound string
	if err := database.WithServiceJob(
		t.Context(), service, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT id::text FROM app.compounds WHERE name_ru = $1`, drug).Scan(&compound)
		},
	); err != nil {
		t.Fatalf("finding the compound: %v", err)
	}

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.vials
				    (patient_id, compound_id, concentration_label, total_doses,
				     opened_at, expires_on)
				VALUES ($1, $2, '2,4 мг/0,75 мл', $3, $4::date, DATE '2026-12-31')
			`, patient, compound, total, opened.String())

			return err
		},
	); err != nil {
		t.Fatalf("opening a vial: %v", err)
	}
}

func sealASpare(t *testing.T, requests *pgxpool.Pool, patient string, total int) {
	t.Helper()

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.vials SET opened_at = NULL WHERE patient_id = $1 AND total_doses = $2`,
				patient, total)

			return err
		},
	); err != nil {
		t.Fatalf("sealing the spare: %v", err)
	}
}

func disposeAVial(t *testing.T, requests *pgxpool.Pool, patient string, total int) {
	t.Helper()

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.vials SET disposed_at = DATE '2026-05-09'
				  WHERE patient_id = $1 AND total_doses = $2`,
				patient, total)

			return err
		},
	); err != nil {
		t.Fatalf("disposing of the vial: %v", err)
	}
}

// drawDoses logs n injections against the seeded course, each drawn from the open vial and
// each on its own day — the slot's unique index is what makes the dates differ.
func drawDoses(t *testing.T, service, requests *pgxpool.Pool, patient string, n int) {
	t.Helper()

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			for day := range n {
				_, err := tx.Exec(ctx, `
					INSERT INTO app.dose_events
					    (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
					     scheduled_for_date, injected_at, dose_value, dose_unit,
					     client_request_id)
					SELECT $1, item.protocol_id, item.id, vial.id, item.compound_id,
					       DATE '2026-05-03' + $2::int, TIMESTAMPTZ '2026-05-03 08:00+03', 0.25, 'мг',
					       'reads-fixture-' || $2::text
					  FROM app.protocol_items AS item
					  JOIN app.protocols AS course ON course.id = item.protocol_id
					  JOIN app.vials AS vial
					    ON vial.patient_id = course.patient_id AND vial.opened_at IS NOT NULL
					 WHERE course.patient_id = $1
				`, patient, day)
				if err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("drawing doses: %v", err)
	}
}

// anotherCourse differs from aCourse in every field a read answers with, so that an answer
// assembled for the wrong patient cannot pass for the right one.
func anotherCourse(patient string) protocol.Draft {
	return protocol.Draft{
		PatientID: civil.UserID(patient),
		StartDate: civil.NewDate(2026, time.May, 4),
		Weeks:     12,
		Status:    protocol.StatusActive,
		Items: []protocol.DraftItem{{
			Kind: protocol.KindInjection,
			Compound: protocol.CompoundRef{New: &protocol.NewCompound{
				NameRU: "Тирзепатид", DefaultUnit: protocol.MG, Route: "im", Icon: "vial",
			}},
			Cadence:  protocol.CadenceDaily,
			Times:    []civil.Slot{{Hour: 20}},
			Loggable: true,
			Phases: []protocol.ProtocolPhase{
				{FromWeek: 1, ToWeek: 12, Dose: protocol.Dose{Value: 5, Unit: protocol.MG}},
			},
		}},
	}
}

// Two patients, both with a running course and an open vial, asked the same three questions.
// What this kills is an answer assembled for the wrong caller — the fixture these reads were
// written against held one patient's data, so every field could have come from anywhere.
// Which of the two locks holds is measured elsewhere and not here: RLS by
// policies_integration_test.go in this package, the Go argument by inventory's and dosing's
// own suites on the service seam, where no policy answers at all.
func TestTheThreeReadsAnswerEachPatientTheirOwn(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing for A: %v", err)
	}
	if _, err := protocol.Create(as(t, writeDoctorB, "doctor"), service, anotherCourse(writePatientB)); err != nil {
		t.Fatalf("prescribing for B: %v", err)
	}
	// B also holds A's drug, opened earlier than A's own vial: without it the two cabinets
	// share no compound and the read's compound filter answers correctly before anything
	// about the patient is consulted. It is a trap and not a measurement — measured, the
	// leak stays unreachable here because RLS filters vialsOf under the request pool
	// whether or not its WHERE and CabinetOf's filter are there. Those two are witnessed
	// on the service seam, in inventory's own suite, where no policy answers.
	//
	// Opened first as well as opened-on first, so that a leak surfaces it whether the read
	// orders by opened_at or falls back to insertion order.
	openAVialOn(t, service, requests, writePatientB, "Семаглутид", 40,
		civil.NewDate(2026, time.April, 20))
	openAVial(t, service, requests, writePatientA, 6)
	openAVialOf(t, service, requests, writePatientB, "Тирзепатид", 12)

	for _, patient := range []struct {
		name     string
		subject  string
		drug     string
		slot     string
		dose     float64
		left     int
		everyDay bool
	}{
		{"A", writePatientA, "Семаглутид", "08:00", 0.25, 6, false},
		{"B", writePatientB, "Тирзепатид", "20:00", 5, 12, true},
	} {
		t.Run(patient.name, func(t *testing.T) {
			_, body := get(t, service, requests, patient.subject, "patient", "/v1/me/today")

			var today struct {
				VialDosesLeft *int `json:"vial_doses_left"`
				NextDose      *struct {
					Time string `json:"time"`
					Dose *struct {
						Value float64 `json:"value"`
					} `json:"dose"`
				} `json:"next_dose"`
				NextDoseCompound *struct {
					NameRU string `json:"name_ru"`
				} `json:"next_dose_compound"`
				WeekProtocol []struct {
					Compound *struct {
						NameRU string `json:"name_ru"`
					} `json:"compound"`
				} `json:"week_protocol"`
			}
			if err := json.Unmarshal([]byte(body), &today); err != nil {
				t.Fatalf("reading the day: %v", err)
			}

			if today.NextDose == nil || today.NextDose.Time != patient.slot {
				t.Fatalf("the card names %+v", today.NextDose)
			}
			if today.NextDose.Dose == nil || today.NextDose.Dose.Value != patient.dose {
				t.Errorf("the dose reads %+v", today.NextDose.Dose)
			}
			if today.NextDoseCompound == nil || today.NextDoseCompound.NameRU != patient.drug {
				t.Errorf("the card's drug is %+v", today.NextDoseCompound)
			}
			if len(today.WeekProtocol) != 1 || today.WeekProtocol[0].Compound == nil ||
				today.WeekProtocol[0].Compound.NameRU != patient.drug {
				t.Errorf("the strip reads %+v", today.WeekProtocol)
			}
			// The cabinet's number is the one that crosses two context boundaries to be
			// answered, and the two vials differ by six doses.
			if today.VialDosesLeft == nil || *today.VialDosesLeft != patient.left {
				t.Errorf("the vial has %v doses left, want %d", shown(today.VialDosesLeft), patient.left)
			}

			_, monthBody := get(t, service, requests, patient.subject, "patient",
				"/v1/me/schedule?month=2026-05")

			var month struct {
				Days []struct {
					Date         string `json:"date"`
					HasInjection bool   `json:"has_injection"`
				} `json:"days"`
			}
			if err := json.Unmarshal([]byte(monthBody), &month); err != nil {
				t.Fatalf("reading the month: %v", err)
			}
			if len(month.Days) != 31 {
				t.Fatalf("May is %d days", len(month.Days))
			}
			// A's weekly course dots four Sundays; B's daily one dots every day from the
			// 4th. The two calendars share no shape, so a month answered for the wrong
			// patient cannot pass for the right one.
			dots := 0
			for _, day := range month.Days {
				if day.HasInjection {
					dots++
				}
			}
			if want := 4; !patient.everyDay && dots != want {
				t.Errorf("the weekly course dots %d days, want %d", dots, want)
			}
			if want := 28; patient.everyDay && dots != want {
				t.Errorf("the daily course dots %d days, want %d", dots, want)
			}

			_, dayBody := get(t, service, requests, patient.subject, "patient",
				"/v1/me/schedule/day?date=2026-05-17")

			var day struct {
				Occurrences []struct {
					Time string `json:"time"`
				} `json:"occurrences"`
			}
			if err := json.Unmarshal([]byte(dayBody), &day); err != nil {
				t.Fatalf("reading the day sheet: %v", err)
			}
			if len(day.Occurrences) != 1 || day.Occurrences[0].Time != patient.slot {
				t.Errorf("the sheet reads %+v", day.Occurrences)
			}
		})
	}
}

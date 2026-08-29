//go:build integration

package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// send is a write with a body against one vial, and what came back.
func send(t *testing.T, mux *chi.Mux, method, path string, body map[string]any) (int, []byte) {
	t.Helper()

	var payload *strings.Reader
	if body == nil {
		payload = strings.NewReader("")
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		payload = strings.NewReader(string(raw))
	}

	request := httptest.NewRequest(method, path, payload)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.Bytes()
}

type vialCard struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	HeldBackAt  *string `json:"held_back_at"`
	DisposedAt  *string `json:"disposed_at"`
	DosesLeft   *int    `json:"doses_left"`
	CurrentDose *struct {
		Value float64 `json:"value"`
	} `json:"current_dose"`
}

func cardFrom(t *testing.T, body []byte) vialCard {
	t.Helper()

	var card vialCard
	if err := json.Unmarshal(body, &card); err != nil {
		t.Fatalf("reading the card: %v", err)
	}

	return card
}

// Setting it aside and taking it back travel the same path, and the day is the patient's own.
//
// Idempotent by value rather than by toggle: a PUT says what the vial should be, so a retry
// from an offline queue cannot flip it back — and repeating the request must not move the day
// the patient set it aside on, which is the field the cabinet reads.
func TestAVialIsSetAsideAndTakenBackByTheSamePath(t *testing.T) {
	c := newClinic(t)
	mux, calling, moveTo := aCabinetInTime(t, c)
	calling(patientA, "patient")

	status, body := send(t, mux, http.MethodPut, "/v1/me/vials/"+c.vialA+"/held-back",
		map[string]any{"held_back": true})
	if status != http.StatusOK {
		t.Fatalf("setting a vial aside answered %d: %s", status, body)
	}
	card := cardFrom(t, body)
	// The patient's own day, which at theCabinetHour is already the 10th in Yekaterinburg
	// and still the 9th in UTC.
	if card.HeldBackAt == nil || *card.HeldBackAt != "2026-05-10" {
		t.Fatalf("the vial was set aside on %v", card.HeldBackAt)
	}

	// Again, with the value it already carries, and a day later — because on the same day
	// rewriting the date and leaving it are the same write, and this is the axis a fixed
	// clock cannot measure at all.
	moveTo(theCabinetHour.Add(24 * time.Hour))
	// The premise, measured rather than assumed: without it a harness that stopped moving
	// its clock would leave the assertion below true under every implementation, which is
	// how the mutation this case exists for survived in the first place.
	if today := dayTheServerIsOn(t, mux, c); today != "2026-05-11" {
		t.Fatalf("the clock reads %q after moving a day, want 2026-05-11", today)
	}
	repeat, body := send(t, mux, http.MethodPut, "/v1/me/vials/"+c.vialA+"/held-back",
		map[string]any{"held_back": true})
	if repeat != http.StatusOK {
		t.Fatalf("repeating answered %d: %s", repeat, body)
	}
	if again := cardFrom(t, body); again.HeldBackAt == nil || *again.HeldBackAt != *card.HeldBackAt {
		t.Errorf("the repeat moved the day to %v, want %v", again.HeldBackAt, card.HeldBackAt)
	}

	// And back, by the same path.
	taken, body := send(t, mux, http.MethodPut, "/v1/me/vials/"+c.vialA+"/held-back",
		map[string]any{"held_back": false})
	if taken != http.StatusOK {
		t.Fatalf("taking it back answered %d: %s", taken, body)
	}
	if back := cardFrom(t, body); back.HeldBackAt != nil {
		t.Errorf("the vial is still set aside on %v", back.HeldBackAt)
	}
}

// A vial the patient set aside is still one they can throw away.
//
// 000021 forbids the two flags together, so disposal has to clear «set aside» in the same
// write — the dead end that migration names, and the same shape as the RESTRICT that made a
// course untitratable once its first dose was logged.
func TestASetAsideVialCanStillBeThrownAway(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	if status, body := send(t, mux, http.MethodPut, "/v1/me/vials/"+c.vialA+"/held-back",
		map[string]any{"held_back": true}); status != http.StatusOK {
		t.Fatalf("setting it aside answered %d: %s", status, body)
	}

	status, body := send(t, mux, http.MethodPost, "/v1/me/vials/"+c.vialA+"/dispose", nil)
	if status != http.StatusOK {
		t.Fatalf("throwing it away answered %d: %s", status, body)
	}

	card := cardFrom(t, body)
	if card.DisposedAt == nil || *card.DisposedAt != "2026-05-10" {
		t.Errorf("it was thrown away on %v", card.DisposedAt)
	}
	if card.HeldBackAt != nil {
		t.Errorf("it is thrown away and still set aside on %v", card.HeldBackAt)
	}
	if card.Status != "disposed" {
		t.Errorf("the card reads %q", card.Status)
	}
	// And it leaves the shelf while keeping its card, which is step 5's own rule.
	if shelf := cabinetOf(t, mux); len(shelf.Vials) != 0 {
		t.Errorf("the shelf still holds %d vials", len(shelf.Vials))
	}
}

// «No vial of mine» and «already thrown away» are different answers, and the shelf read before
// the write is what separates them: after it, zero rows can only mean the second.
func TestNothingWrittenIsReadRatherThanGuessedAt(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	if status, _ := send(t, mux, http.MethodPost, "/v1/me/vials/"+c.vialA+"/dispose", nil); status != http.StatusOK {
		t.Fatalf("the first disposal answered %d", status)
	}

	// Twice: the vial is there and the write matches nothing, which is a conflict.
	if status, body := send(t, mux, http.MethodPost, "/v1/me/vials/"+c.vialA+"/dispose", nil); status != http.StatusConflict {
		t.Errorf("throwing away a thrown-away vial answered %d, want 409: %s", status, body)
	}

	for _, absent := range []struct {
		name string
		vial string
	}{
		{"another patient's", c.vialB},
		{"one nobody has", "8a1f3b7c-0000-4000-8000-00000000dead"},
	} {
		t.Run(absent.name, func(t *testing.T) {
			status, body := send(t, mux, http.MethodPost, "/v1/me/vials/"+absent.vial+"/dispose", nil)
			if status != http.StatusNotFound {
				t.Errorf("disposing answered %d, want 404: %s", status, body)
			}
			status, body = send(t, mux, http.MethodPut, "/v1/me/vials/"+absent.vial+"/held-back",
				map[string]any{"held_back": true})
			if status != http.StatusNotFound {
				t.Errorf("setting aside answered %d, want 404: %s", status, body)
			}
		})
	}
}

// A thrown-away vial has no lifecycle left to change, and says so as a conflict.
//
// This is the sequence idempotence-by-value is argued from: the offline queue delivers a «put
// it aside» that was written before the patient discarded the vial. Without a guard the write
// met 000021's CHECK and left as a 500 — which the queue retries for ever — and the two values
// answered differently, 200 for taking it back and 500 for setting it aside.
func TestAThrownAwayVialRefusesToBeSetAside(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	if status, body := send(t, mux, http.MethodPost,
		"/v1/me/vials/"+c.vialA+"/dispose", nil); status != http.StatusOK {
		t.Fatalf("throwing it away answered %d: %s", status, body)
	}

	// Setting it aside is the conflict; taking it back is the value the vial already
	// carries, because disposal cleared the flag — and refusing that would break the
	// idempotence this endpoint promises exactly where the queue makes it matter.
	for _, asked := range []struct {
		name  string
		body  map[string]any
		wants int
	}{
		{"set aside", map[string]any{"held_back": true}, http.StatusConflict},
		{"taken back", map[string]any{"held_back": false}, http.StatusOK},
	} {
		t.Run(asked.name, func(t *testing.T) {
			status, body := send(t, mux, http.MethodPut,
				"/v1/me/vials/"+c.vialA+"/held-back", asked.body)
			if status != asked.wants {
				t.Errorf("answered %d, want %d: %s", status, asked.wants, body)
			}
		})
	}

	// Both halves, and they are held by different things: that the disposal stayed is the
	// code's, while «set aside» could not have been written whatever happened, because
	// 000021 refuses the pair and the transaction rolls back. The second is kept as the
	// statement of what the endpoint must never produce, not as a measurement of it.
	status, body := send(t, mux, http.MethodGet, "/v1/me/vials/"+c.vialA, nil)
	if status != http.StatusOK {
		t.Fatalf("reading the card answered %d: %s", status, body)
	}
	card := cardFrom(t, body)
	if card.DisposedAt == nil || *card.DisposedAt != "2026-05-10" || card.Status != "disposed" {
		t.Errorf("the vial reads thrown away on %v with status %q", card.DisposedAt, card.Status)
	}
	if card.HeldBackAt != nil {
		t.Errorf("a thrown-away vial is set aside on %v", card.HeldBackAt)
	}
}

// A day that moved backwards under the patient is a conflict, not a server error.
//
// Nothing in this endpoint can produce it on its own — disposal writes the patient's own day —
// but opened_at is written by the dosing path on the day a dose was drawn, and the patient's
// zone is rewritten on every sign-in. A move west across midnight leaves today behind the day
// the vial was opened on, and 000015's CHECK refuses the write — which unmapped reaches the
// patient as a server error about a state they can see, where a conflict names it.
func TestAVialOpenedAfterTodayIsAConflictRatherThanAServerError(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	changeVial(t, c, patientA, c.vialA,
		`UPDATE app.vials SET opened_at = DATE '2026-05-11' WHERE id = $1`)

	status, body := send(t, mux, http.MethodPost, "/v1/me/vials/"+c.vialA+"/dispose", nil)
	if status != http.StatusConflict {
		t.Fatalf("throwing away a vial opened tomorrow answered %d, want 409: %s", status, body)
	}
	// Which conflict, because answerWrite folds both into one status: without this the
	// case passes just as well when the disposal is refused for having happened already.
	if !strings.Contains(string(body), "before the day it was opened") {
		t.Errorf("the conflict reads %s", body)
	}
}

// Neither write reaches another patient's shelf, and the witness is the row read back as its
// owner: the shelf refuses the vial before any statement runs, so a status alone would say
// nothing about whether something was written.
func TestNeitherWriteReachesAnotherPatientsVial(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	for _, attempt := range []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{"setting it aside", http.MethodPut, "/held-back", map[string]any{"held_back": true}},
		{"throwing it away", http.MethodPost, "/dispose", nil},
	} {
		t.Run(attempt.name, func(t *testing.T) {
			if status, _ := send(t, mux, attempt.method,
				"/v1/me/vials/"+c.vialB+attempt.path, attempt.body); status != http.StatusNotFound {
				t.Errorf("answered %d, want 404", status)
			}

			// Read as the owner, because the caller cannot see the row at all: a 404
			// says nothing about whether the write landed.
			var heldBack, disposed *string
			if err := c.as(t, patientB, "patient", func(ctx context.Context, tx pgx.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT to_char(held_back_at, 'YYYY-MM-DD'), to_char(disposed_at, 'YYYY-MM-DD')
					FROM app.vials WHERE id = $1
				`, c.vialB).Scan(&heldBack, &disposed)
			}); err != nil {
				t.Fatalf("reading the other patient's vial: %v", err)
			}
			if heldBack != nil || disposed != nil {
				t.Errorf("the other patient's vial reads held back %v, disposed %v", heldBack, disposed)
			}
		})
	}
}

// A patient-only surface, both halves of it.
func TestOnlyAPatientChangesAVialsLifecycle(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)

	for _, endpoint := range []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{"held-back", http.MethodPut, "/held-back", map[string]any{"held_back": true}},
		{"dispose", http.MethodPost, "/dispose", nil},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			calling(doctorA, "doctor")
			if status, body := send(t, mux, endpoint.method,
				"/v1/me/vials/"+c.vialA+endpoint.path, endpoint.body); status != http.StatusForbidden {
				t.Errorf("a doctor answered %d: %s", status, body)
			}
			calling("", "")
			if status, body := send(t, mux, endpoint.method,
				"/v1/me/vials/"+c.vialA+endpoint.path, endpoint.body); status != http.StatusUnauthorized {
				t.Errorf("an unauthenticated write answered %d: %s", status, body)
			}
		})
	}
}

// dayTheServerIsOn asks the cabinet what day it thinks it is, by setting a second vial aside
// and reading the day it wrote.
func dayTheServerIsOn(t *testing.T, mux *chi.Mux, c clinic) string {
	t.Helper()

	status, body := post(t, mux, "/v1/me/vials", aVial(c, nil))
	if status != http.StatusCreated {
		t.Fatalf("adding a vial to ask the date with answered %d: %s", status, body)
	}
	spare := cardFrom(t, body).ID

	status, body = send(t, mux, http.MethodPut, "/v1/me/vials/"+spare+"/held-back",
		map[string]any{"held_back": true})
	if status != http.StatusOK {
		t.Fatalf("setting the spare aside answered %d: %s", status, body)
	}
	card := cardFrom(t, body)
	if card.HeldBackAt == nil {
		t.Fatal("the spare came back not set aside")
	}

	return *card.HeldBackAt
}

//go:build integration

package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	mux, calling := aCabinet(t, c)
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

	// Again, with the value it already carries: not an error, and the day does not move.
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

// Zero rows affected is «no vial of mine» and «already thrown away» at once, and the two are
// different answers — so the row is read rather than the count guessed at.
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

// Neither write reaches another patient's shelf, and the witness is the row read back rather
// than the count: an UPDATE the policies filter reports success over no rows.
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

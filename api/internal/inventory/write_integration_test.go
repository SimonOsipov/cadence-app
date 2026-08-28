//go:build integration

package inventory_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// post sends a body to one of the write endpoints and answers what came back.
func post(t *testing.T, mux *chi.Mux, path string, body map[string]any) (int, []byte) {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.Bytes()
}

func aVial(c clinic, edit func(map[string]any)) map[string]any {
	body := map[string]any{
		"compound_id":         c.compound,
		"concentration_label": "2,4 мг/0,75 мл",
		"total_amount":        map[string]any{"value": 2.4, "unit": "мг"},
		"expires_on":          "2026-12-31",
	}
	if edit != nil {
		edit(body)
	}

	return body
}

// The happy path, which a suite of refusals cannot supply: a form the clinic fills in is
// accepted, stored sealed, and answered as the card the client just created.
func TestAVialTheFormFillsInIsAccepted(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	status, body := post(t, mux, "/v1/me/vials", aVial(c, func(v map[string]any) {
		v["lot"] = "LOT-7781"
		v["location_ru"] = "дверца холодильника"
	}))
	if status != http.StatusCreated {
		t.Fatalf("adding a vial answered %d: %s", status, body)
	}

	var created struct {
		ID          string  `json:"id"`
		CompoundID  string  `json:"compound_id"`
		Status      string  `json:"status"`
		OpenedAt    *string `json:"opened_at"`
		ExpiresOn   string  `json:"expires_on"`
		Lot         *string `json:"lot"`
		TotalAmount struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"total_amount"`
		RemainingAmount struct {
			Value float64 `json:"value"`
		} `json:"remaining_amount"`
		HasLabelPhoto bool `json:"has_label_photo"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}

	// Sealed, because nothing here sets an opening date: the first dose drawn from it is
	// what opens a vial, which is step 3's own rule.
	if created.OpenedAt != nil || created.Status != "sealed" {
		t.Errorf("the vial came back opened on %v with status %q", created.OpenedAt, created.Status)
	}
	// Kept in the unit the box carries, and converted by nothing: 2,4 мг in, 2,4 мг out,
	// and the whole of it still in there.
	if created.TotalAmount.Value != 2.4 || created.TotalAmount.Unit != "мг" {
		t.Errorf("the box reads %v %s", created.TotalAmount.Value, created.TotalAmount.Unit)
	}
	if created.RemainingAmount.Value != 2.4 {
		t.Errorf("a vial nobody has drawn from has %v мг left", created.RemainingAmount.Value)
	}
	if created.Lot == nil || *created.Lot != "LOT-7781" || created.CompoundID != c.compound {
		t.Errorf("the card reads %+v", created)
	}
	if created.HasLabelPhoto {
		t.Error("a vial added without a photograph says it has one")
	}

	// And it is on the shelf the next read answers, which is the fact the client's own
	// answer cannot establish: a 201 body assembled in the handler would say the same
	// thing about a row that was rolled back.
	shelf := cabinetOf(t, mux)
	if len(shelf.Vials) != 2 {
		t.Fatalf("the cabinet holds %d vials, want the seeded one and the new one", len(shelf.Vials))
	}
	var found bool
	for _, vial := range shelf.Vials {
		found = found || vial.ID == created.ID
	}
	if !found {
		t.Errorf("the vial just created is not on the shelf: %+v", shelf.Vials)
	}
}

// Every refusal the form can earn, answered as the field rather than as a constraint.
func TestAVialTheSchemaWouldRefuseIsRefusedHere(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	for _, refused := range []struct {
		name string
		edit func(map[string]any)
	}{
		{
			// The scale bound: whole micrograms are what the cabinet subtracts, and
			// a milligram past three decimals loses its tail on the way in.
			"an amount finer than a microgram",
			func(v map[string]any) { v["total_amount"] = map[string]any{"value": 0.0001, "unit": "мг"} },
		},
		{
			"an amount of nothing",
			func(v map[string]any) { v["total_amount"] = map[string]any{"value": 0, "unit": "мг"} },
		},
		{
			// 000024's ceiling for a container, which is a hundred times a dose's.
			"a vial holding more than a hundred grams",
			func(v map[string]any) { v["total_amount"] = map[string]any{"value": 100001, "unit": "мг"} },
		},
		{
			"a drug the directory does not have",
			func(v map[string]any) { v["compound_id"] = "8a1f3b7c-0000-4000-8000-00000000dead" },
		},
		{
			// The forged key, and the reason the store's own security cannot catch
			// it: there is none. A key under another patient's prefix is refused by
			// the CHECK the column carries, for every role including this one.
			"a label key under somebody else's prefix",
			func(v map[string]any) { v["label_photo_path"] = patientB + "/label.jpg" },
		},
		{
			// The shape and not the prefix: this one begins with the caller's own
			// identifier and points out of it.
			"a label key that climbs out of its own prefix",
			func(v map[string]any) { v["label_photo_path"] = patientA + "/../" + patientB + "/label.jpg" },
		},
	} {
		t.Run(refused.name, func(t *testing.T) {
			status, body := post(t, mux, "/v1/me/vials", aVial(c, refused.edit))
			if status != http.StatusUnprocessableEntity {
				t.Errorf("answered %d, want 422: %s", status, body)
			}
			// Nothing was stored: a refusal that left the row behind would be a
			// cabinet the patient did not fill.
			if shelf := cabinetOf(t, mux); len(shelf.Vials) != 1 {
				t.Errorf("the cabinet holds %d vials after a refusal", len(shelf.Vials))
			}
		})
	}
}

// The key is minted here and carries the caller's own prefix, so the vial that quotes it back
// passes the CHECK a client-chosen key would not.
func TestTheLabelKeyIsMintedForTheCallerAndAccepted(t *testing.T) {
	c := newClinic(t)
	mux, _, as := withLabelPhotos(t, c)
	as(patientA, "patient")

	status, body := post(t, mux, "/v1/me/vials/label-photo-uploads",
		map[string]any{"content_type": "image/jpeg"})
	if status != http.StatusOK {
		t.Fatalf("asking for a link answered %d: %s", status, body)
	}
	var minted struct {
		URL       string `json:"url"`
		Key       string `json:"key"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &minted); err != nil {
		t.Fatalf("reading the link: %v", err)
	}

	if !strings.HasPrefix(minted.Key, patientA+"/") {
		t.Errorf("the key %q is not under the caller's own prefix", minted.Key)
	}
	if !strings.HasSuffix(minted.Key, ".jpg") {
		t.Errorf("the key %q does not say what was stored", minted.Key)
	}
	if minted.URL == "" || minted.ExpiresAt == "" {
		t.Errorf("the answer carries %+v", minted)
	}

	// The whole point of minting it: the vial that quotes it back is accepted, which the
	// forged keys above are not.
	created, answer := post(t, mux, "/v1/me/vials", aVial(c, func(v map[string]any) {
		v["label_photo_path"] = minted.Key
	}))
	if created != http.StatusCreated {
		t.Fatalf("a vial quoting a minted key answered %d: %s", created, answer)
	}
	var card struct {
		HasLabelPhoto bool `json:"has_label_photo"`
	}
	if err := json.Unmarshal(answer, &card); err != nil {
		t.Fatalf("reading the card: %v", err)
	}
	if !card.HasLabelPhoto {
		t.Error("the vial was stored with a key and says it has no photograph")
	}
	if strings.Contains(string(answer), minted.Key) {
		t.Errorf("the key travelled back in the card: %s", answer)
	}
}

// A patient-only surface, both halves of it.
func TestOnlyAPatientAddsAVial(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)

	// Each endpoint gets its own body: huma validates the body before the handler runs, so
	// a request carrying the wrong shape answers 422 whoever sent it, and the role would
	// never be reached — the axis here is who is calling, not what they sent.
	for _, endpoint := range []struct {
		path string
		body map[string]any
	}{
		{"/v1/me/vials", aVial(c, nil)},
		{"/v1/me/vials/label-photo-uploads", map[string]any{"content_type": "image/jpeg"}},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			calling(doctorA, "doctor")
			if status, body := post(t, mux, endpoint.path, endpoint.body); status != http.StatusForbidden {
				t.Errorf("a doctor answered %d: %s", status, body)
			}
			calling("", "")
			if status, body := post(t, mux, endpoint.path, endpoint.body); status != http.StatusUnauthorized {
				t.Errorf("an unauthenticated write answered %d: %s", status, body)
			}
		})
	}
}

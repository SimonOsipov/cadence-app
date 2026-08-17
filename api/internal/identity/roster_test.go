package identity

import (
	"encoding/base64"
	"errors"
	"testing"
)

// A cursor is opaque to the client and total for the server: whatever a name contains, the pair it
// encodes has to come back byte for byte, or paging silently restarts or skips.
func TestACursorCarriesThePairItWasMadeFrom(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
	}{
		{"an ordinary name", "Марина Волкова"},
		{"a name with the separator in it", "Волкова\x00Марина"},
		{"a name with a newline", "Марина\nВолкова"},
		{"a name that is one character", "К"},
		{"a name with a quote", `О'Нил`},
	}

	const userID = "8a1f3b7c-0000-4000-8000-000000000001"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotID, err := readCursor(makeCursor(tc.fullName, userID))
			if err != nil {
				t.Fatalf("reading the cursor back: %v", err)
			}

			if gotName != tc.fullName {
				t.Errorf("full name = %q, want %q", gotName, tc.fullName)
			}
			if gotID != userID {
				t.Errorf("user id = %q, want %q", gotID, userID)
			}
		})
	}
}

// A cursor arrives from the wire, so every shape it can be mangled into has to be a refusal rather
// than a pair the query would then be run with.
func TestACursorThatIsNotOneIsRefused(t *testing.T) {
	valid := makeCursor("Марина Волкова", "8a1f3b7c-0000-4000-8000-000000000001")

	tests := []struct {
		name   string
		cursor string
	}{
		{"not base64 at all", "не курсор"},
		{"base64 without the separator", raw("8a1f3b7c-0000-4000-8000-000000000001")},
		{"an id that is not a uuid", makeCursor("Марина Волкова", "not-a-uuid")},
		{"an id that is empty", makeCursor("Марина Волкова", "")},
		{"truncated", valid[:len(valid)-4]},
		{"three fields", raw("Марина\x008a1f3b7c-0000-4000-8000-000000000001\x00extra")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := readCursor(tc.cursor); !errors.Is(err, ErrNotACursor) {
				t.Fatalf("reading %q answered %v, want ErrNotACursor", tc.cursor, err)
			}
		})
	}
}

// The empty cursor is the first page and not a malformed one: it is what a client sends before it has
// been given anything to continue from.
func TestNoCursorIsTheFirstPage(t *testing.T) {
	name, id, err := readCursor("")
	if err != nil {
		t.Fatalf("the empty cursor is refused: %v", err)
	}

	if name != "" || id != "" {
		t.Errorf("the empty cursor decoded to %q/%q, want a pair of empty strings", name, id)
	}
}

func TestWhichRefusalTheRosterHeard(t *testing.T) {
	tests := []struct {
		name       string
		refusal    error
		wantStatus int
		wantDetail string
	}{
		{"a patient asking for the roster", ErrNotForPatients, 403, detailRosterIsNotForPatients},
		{"a cursor that is not one", ErrNotACursor, 400, detailNotACursor},
		{"the database did not answer", ErrDatabaseUnavailable, 503, detailUnavailableOnTheWire},
		{"a refusal this package does not name", errors.New("boom"), 500, detailInternalOnTheWire},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			answered := answerFor(t, refusalForRoster, tc.refusal)

			if answered.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d: %s", answered.Status, tc.wantStatus, answered.Detail)
			}
			if answered.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", answered.Detail, tc.wantDetail)
			}
		})
	}
}

// The sentences, by literal, for the reason the session route's are: everywhere else they are compared
// against their own constants.
func TestTheRussianTheDashboardReads(t *testing.T) {
	if detailRosterIsNotForPatients != "Реестр пациентов доступен только сотрудникам клиники." {
		t.Errorf("detailRosterIsNotForPatients = %q", detailRosterIsNotForPatients)
	}

	if detailNotACursor != "Страница не найдена. Откройте реестр заново." {
		t.Errorf("detailNotACursor = %q", detailNotACursor)
	}
}

// raw encodes bytes the way makeCursor does, so the refusal table can carry payloads makeCursor would
// never produce.
func raw(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

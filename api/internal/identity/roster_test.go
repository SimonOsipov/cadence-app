package identity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
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
		{"an account provisioning has not reached", ErrNoRole, 403, detailNoRole},
		{"a page of no rows", ErrNotAPageSize, 400, detailNotAPageSize},
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

// The patient's refusal at the route, not at the mapper. Without this the branch in the handler can be
// deleted whole and the refusal table stays green, because it only ever asks the mapper what it would
// have said. The service is nil, so an answer that reached the database would be a 500 rather than a
// pass.
func TestAPatientAskingForTheRosterIsRefused(t *testing.T) {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{
				Subject: "8a1f3b7c-0000-4000-8000-000000000001",
				Role:    patientRole,
			})))
		})
	})
	NewService(Deps{}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}

	var problem struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Type != httpserver.ProblemForbidden {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemForbidden)
	}
	if problem.Detail != detailRosterIsNotForPatients {
		t.Errorf("detail = %q, want %q", problem.Detail, detailRosterIsNotForPatients)
	}
}

// The operation and the statuses it declares, for the reason the session route's are: a status that
// lives only in the description is a branch no generated client can write.
func TestTheOverviewIsInTheContract(t *testing.T) {
	api := httpserver.NewAPI(chi.NewRouter())
	NewService(Deps{}).Register(api)

	document, err := api.OpenAPI().MarshalJSON()
	if err != nil {
		t.Fatalf("marshalling the document: %v", err)
	}

	var spec struct {
		Paths map[string]struct {
			Get struct {
				Responses map[string]any `json:"responses"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(document, &spec); err != nil {
		t.Fatalf("decoding the document: %v", err)
	}

	operation, ok := spec.Paths["/v1/dashboard/overview"]
	if !ok {
		t.Fatalf("the document does not describe /v1/dashboard/overview; it has %v", spec.Paths)
	}

	// 422 and 500 alongside the declared ones, as the session route asserts them: both are reachable
	// here — 422 from the query tags, 500 from an assembly with no roster.
	for _, status := range []string{"200", "400", "401", "403", "422", "500", "503"} {
		if _, declared := operation.Get.Responses[status]; !declared {
			t.Errorf("the operation does not declare %s; it declares %v", status, operation.Get.Responses)
		}
	}
}

// A bound below what makeCursor emits is a page whose own next_cursor the route then answers 422.
func TestTheDeclaredCursorBoundCoversTheLongestOneThisServerCanIssue(t *testing.T) {
	// The column allows 200 characters, and a character in this product's copy is up to four bytes.
	longest := makeCursor(strings.Repeat("𝛑", 200), "8a1f3b7c-0000-4000-8000-000000000001")

	if len(longest) > maxCursorLength {
		t.Errorf("the longest cursor is %d characters, and the bound is %d", len(longest), maxCursorLength)
	}

	declared := reflect.TypeOf(OverviewInput{})
	field, ok := declared.FieldByName("Cursor")
	if !ok {
		t.Fatal("OverviewInput has no Cursor field")
	}

	if got := field.Tag.Get("maxLength"); got != strconv.Itoa(maxCursorLength) {
		t.Errorf("the route declares maxLength %q, and maxCursorLength is %d", got, maxCursorLength)
	}
}

// The service assembled without a roster. The patient's refusal happens before this check, so without
// a caller who gets past the role switch the guard is unreachable and its deletion is invisible.
func TestADoctorIsRefusedWhenTheRosterServiceIsAbsent(t *testing.T) {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{
				Subject: "8a1f3b7c-0000-4000-8000-000000000002",
				Role:    providerRole,
			})))
		})
	})
	NewService(Deps{}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

// An account the invitation reached and provisioning did not, at the route: the seam has no Postgres
// role for it, so without this arm it reaches the database and comes back a 500.
func TestAnAccountWithNoRoleIsRefusedTheRoster(t *testing.T) {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{
				Subject: "8a1f3b7c-0000-4000-8000-000000000004",
			})))
		})
	})
	NewService(Deps{}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}

	var problem struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Detail != detailNoRole {
		t.Errorf("detail = %q, want %q", problem.Detail, detailNoRole)
	}
}

// The administrator's arm of the role switch, which nothing drove through the route: the integration
// tests call Patients directly, below it. Dropping adminRole from the case answers the clinic's
// administrator 403 «this account is not in the clinic yet», and every other test stays green.
func TestAnAdministratorReachesTheRoster(t *testing.T) {
	rec := askForTheRoster(t, auth.Principal{
		Subject: "8a1f3b7c-0000-4000-8000-000000000003",
		Role:    adminRole,
	})

	// 500 and not 200: the service is nil, so getting this far is the whole of what is asserted — the
	// administrator passed the role switch rather than being refused by it.
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

// A caller the middleware would never have let through. Unreachable in the assembled router, and
// asserted for the reason GET /v1/me asserts its own: «unreachable» is a property of the wiring, and
// without it a lost principal is answered 403 «not in the clinic yet» rather than 401.
func TestTheRosterRefusesWithoutAPrincipal(t *testing.T) {
	router := chi.NewRouter()
	NewService(Deps{}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
}

// The constructor's whole content, pinned as NewSessions' is: without it a nil pool builds a service
// whose nil check in the handler is false, and the answer is a dereference rather than a 500.
func TestNoPoolBuildsNoRoster(t *testing.T) {
	if roster := NewRoster(nil, &stubProvisioner{}); roster != nil {
		t.Errorf("NewRoster with no pool = %v, want nil", roster)
	}
}

// askForTheRoster drives the route with principal standing in for the middleware.
func askForTheRoster(t *testing.T, principal auth.Principal) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
		})
	})
	NewService(Deps{}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil))

	return rec
}

// The join between the query and database.IsUnavailable, which neither end's own test reaches: the
// classifier is measured in platform/database and the 503 mapping in the refusal table, and between
// them sits one call. A port the kernel handed out and nothing listens on is the cheapest database
// that is down.
func TestARosterReadAgainstADatabaseThatIsDownIsUnavailable(t *testing.T) {
	var config net.ListenConfig

	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("closing the listener: %v", err)
	}

	pool, err := pgxpool.New(t.Context(), "postgres://nobody:nothing@"+address+"/none?sslmode=disable")
	if err != nil {
		t.Fatalf("building the pool: %v", err)
	}
	t.Cleanup(pool.Close)

	caller := database.Caller{Subject: "8a1f3b7c-0000-4000-8000-000000000002", Role: providerRole}

	_, err = NewRoster(pool, nil).Patients(t.Context(), caller, "", 8)
	if !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("reading against a dead database answered %v, want ErrDatabaseUnavailable", err)
	}
}

// A page of none. Reachable only past the route's schema, which is the point: the method is exported,
// and without the guard the arithmetic below it indexes at -1.
func TestAPageOfNoRowsIsRefusedRatherThanIndexed(t *testing.T) {
	for _, limit := range []int{0, -1} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			// The pool is never reached: the guard is above it, and a nil pool would panic if it were not.
			caller := database.Caller{Subject: "8a1f3b7c-0000-4000-8000-000000000002", Role: providerRole}

			_, err := (&Roster{}).Patients(t.Context(), caller, "", limit)
			if !errors.Is(err, ErrNotAPageSize) {
				t.Fatalf("a page of %d answered %v, want ErrNotAPageSize", limit, err)
			}
		})
	}
}

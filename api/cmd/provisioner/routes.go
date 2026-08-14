package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	// Without maxBatchLookup, the size of a fan-out against the identity
	// provider is decided by whoever writes the request body.
	maxBatchLookup = 100

	// batchBudget bounds that fan-out in time as well: fifty times smaller than
	// the maxBatchLookup × gotrueCallTimeout the count alone would allow,
	// because a roster that has not answered in twenty seconds is a roster
	// nobody is still waiting for. The relationship is asserted by
	// TestTheBatchIsBoundedInTimeAndNotOnlyInCount, not left to this comment.
	batchBudget = 20 * time.Second

	// Generous for five small documents, small enough that nothing here is a
	// memory sink.
	maxRequestBody = 1 << 16
)

type server struct {
	environment environment
	secrets     secrets
	gotrue      *gotrueClient
	logger      *slog.Logger
}

// mux is the same function the tests walk; a second assembly written for them
// would prove things about the test rather than about what this process serves.
//
// There is no health route, no metrics route and no profiler: a sixth path is a
// sixth thing reachable by whoever reaches this component, and the platform can
// probe the port. Every operation is a POST carrying a JSON body, including the
// two that read and the one that deletes, because an address or an account
// identifier in a path or a query string is an address in every access log
// between here and the caller.
func (s *server) mux() *chi.Mux {
	mux := chi.NewMux()

	// Before any route, which chi requires and which is also the safe
	// direction: a route mounted before the guard would be served without it.
	mux.Use(s.guard)

	mux.Post("/invite", s.invite)
	mux.Post("/users/lookup", s.lookup)
	mux.Post("/users/lookup-batch", s.lookupBatch)
	mux.Post("/users/delete", s.delete)

	// Absent in production rather than refused inside the handler: a route that
	// exists is a route one flag away from serving, and an operation that sets
	// any account's password is the identity-capture chain this component
	// exists to break. Only the seeds need it, so a doctor has something to
	// sign in with.
	if s.environment != production {
		mux.Post("/users/password", s.setPassword)
	}

	return mux
}

func (s *server) invite(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email string `json:"email"`
	}
	if !s.decode(w, r, &payload) {
		return
	}

	address, ok := emailAddress(payload.Email)
	if !ok {
		s.refuse(w, r, http.StatusBadRequest, "email must be one complete address")

		return
	}

	created, err := s.gotrue.invite(r.Context(), address)
	if err != nil {
		s.failed(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"account": created})
}

func (s *server) lookup(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email string `json:"email"`
	}
	if !s.decode(w, r, &payload) {
		return
	}

	// Refused before the call, not filtered after it: a substring is not an
	// address, and no shape of this operation answers with a list.
	address, ok := emailAddress(payload.Email)
	if !ok {
		s.refuse(w, r, http.StatusBadRequest, "email must be one complete address, not a fragment to search by")

		return
	}

	found, err := s.gotrue.findByAddress(r.Context(), address)
	if err != nil {
		s.failed(w, r, err)

		return
	}

	// A null account rather than a 404: the caller acts on it — it is what makes
	// an address free to invite.
	writeJSON(w, http.StatusOK, map[string]any{"account": found})
}

func (s *server) lookupBatch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		IDs []string `json:"ids"`
	}
	if !s.decode(w, r, &payload) {
		return
	}

	if len(payload.IDs) == 0 || len(payload.IDs) > maxBatchLookup {
		s.refuse(w, r, http.StatusBadRequest, "ids must name between one and one hundred accounts")

		return
	}

	for _, id := range payload.IDs {
		if !identifierShaped(id) {
			s.refuse(w, r, http.StatusBadRequest, "every id must be an account identifier")

			return
		}
	}

	// The N+1 below stays inside the component rather than crossing the
	// boundary, which is the whole reason this operation exists.
	//
	// One deadline over the whole fan-out: the per-call timeout bounds a call,
	// and WriteTimeout ends the response without cancelling the handler — so
	// without this the caller chooses how long a handler runs by choosing how
	// many identifiers to send.
	ctx, cancel := context.WithTimeout(r.Context(), batchBudget)
	defer cancel()

	accounts := make([]account, 0, len(payload.IDs))

	for _, id := range payload.IDs {
		found, err := s.gotrue.findByID(ctx, id)
		if err != nil {
			s.failed(w, r, err)

			return
		}

		// Absent from the answer rather than an error for the others: a roster
		// holding one stale row would otherwise stop rendering entirely.
		if found != nil {
			accounts = append(accounts, *found)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

// delete exists for a single scenario: the invitation went out, the transaction
// rolled back, and the person opened the link and set a password. `/invite`
// then answers 422 forever, there is no address change, and nothing here can
// reach the `auth` schema — so without this the address is burned for good.
//
// The proof comes from the caller because this component cannot see the `app`
// schema. That is the weak point, written down rather than papered over: a
// compromised API can state whatever proof it likes. What the proof buys is
// that a mistake — a handler deleting the wrong account — has to be a lie
// rather than an omission, so both halves are required explicitly and neither
// may be inferred from a field somebody forgot to send.
func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID            string `json:"id"`
		InviteExists  *bool  `json:"invite_exists"`
		ProfileExists *bool  `json:"profile_exists"`
	}
	if !s.decode(w, r, &payload) {
		return
	}

	switch {
	case !identifierShaped(payload.ID):
		s.refuse(w, r, http.StatusBadRequest, "id must be an account identifier")
	case payload.InviteExists == nil || payload.ProfileExists == nil:
		s.refuse(w, r, http.StatusBadRequest,
			"invite_exists and profile_exists must both be stated by the caller")
	case !*payload.InviteExists || *payload.ProfileExists:
		s.refuse(w, r, http.StatusConflict,
			"only an account with an invite record and no profile may be deleted")
	default:
		if err := s.gotrue.deleteAccount(r.Context(), payload.ID); err != nil {
			s.failed(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// setPassword is mounted outside production only — see mux.
func (s *server) setPassword(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}
	if !s.decode(w, r, &payload) {
		return
	}

	if !identifierShaped(payload.ID) || payload.Password == "" {
		s.refuse(w, r, http.StatusBadRequest, "id must be an account identifier and password must not be empty")

		return
	}

	if err := s.gotrue.setPassword(r.Context(), payload.ID, payload.Password); err != nil {
		s.failed(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Keeps a caller's string out of the path it would otherwise be pasted into.
var identifierPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

func identifierShaped(id string) bool {
	return identifierPattern.MatchString(id)
}

// emailAddress lowercases, because GoTrue's `?filter=` is case-sensitive while
// `/invite` stores the address folded: without this an account that exists is
// invisible to the spelling a human typed. It refuses anything but a complete
// address — `mail.ParseAddress` also accepts `Anna <anna@clinic.example>`, and
// a display name is not what any caller here means.
func emailAddress(raw string) (string, bool) {
	address := strings.ToLower(strings.TrimSpace(raw))

	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Name != "" || parsed.Address != address {
		return "", false
	}

	_, domain, found := strings.Cut(address, "@")
	if !found || !strings.Contains(domain, ".") {
		return "", false
	}

	return address, true
}

func (s *server) decode(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody)).Decode(into); err != nil {
		s.refuse(w, r, http.StatusBadRequest, "the body is not the JSON document this operation takes")

		return false
	}

	return true
}

// failed answers 502 rather than 500: this component is a narrow front for
// somebody else's API, and the difference between "we are broken" and "the
// identity provider is" is the first thing an operator needs. What the provider
// said goes to the log — see refuse.
func (s *server) failed(w http.ResponseWriter, r *http.Request, err error) {
	s.refuse(w, r, http.StatusBadGateway, err.Error())
}

// refuse decides which reasons a caller may read. A 401 says one fixed thing,
// or a refusal tells whoever is guessing which half of the guess was right; a
// 5xx says one fixed thing because it would otherwise carry the identity
// provider's own words about accounts the caller may have no business knowing
// about. Both are logged in full, where they are useful and private.
func (s *server) refuse(w http.ResponseWriter, r *http.Request, status int, reason string) {
	if status == http.StatusUnauthorized || status >= http.StatusInternalServerError {
		s.logger.WarnContext(r.Context(), "refused",
			"status", status, "method", r.Method, "path", r.URL.Path, "reason", reason)

		reason = http.StatusText(status)
	}

	writeJSON(w, status, map[string]string{"error": reason})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":"Internal Server Error"}`, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status line is already on the wire, so a failed write leaves nothing
	// to report it to.
	_, _ = w.Write(body)
}

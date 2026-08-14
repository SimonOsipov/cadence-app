package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeGoTrue reproduces the two behaviours measured against the real provider
// on 2026-07-30 that this component exists to contain:
//
//   - `?filter=` is a substring match, and it is case-sensitive
//   - `/invite` stores the address lowercased, so a mixed-case filter finds
//     nothing against an account that does exist
//
// A fake that matched on equality, or case-insensitively, would let a component
// that forgot to lowercase pass every test here. The same round trip runs
// against the pinned image in gotrue_admin_integration_test.go, which is what
// keeps this fake honest.
type fakeGoTrue struct {
	*httptest.Server

	mu       sync.Mutex
	users    map[string]*fakeUser
	requests []recordedRequest
	fail     int
}

type fakeUser struct {
	ID           string
	Email        string
	Password     string
	ConfirmedAt  *time.Time
	LastSignInAt *time.Time
}

// recordedRequest is what the component actually sent: the address it filtered
// on and the token it presented are asserted elsewhere, and neither is
// observable from the answer.
type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Token  string
	Body   string
}

func startFakeGoTrue(t *testing.T) *fakeGoTrue {
	t.Helper()

	fake := &fakeGoTrue{users: map[string]*fakeUser{}}
	fake.Server = httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(fake.Close)

	return fake
}

// withUser plants an account the way /invite would: the address lowercased.
func (f *fakeGoTrue) withUser(id, email string) *fakeUser {
	f.mu.Lock()
	defer f.mu.Unlock()

	confirmed := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	user := &fakeUser{ID: id, Email: strings.ToLower(email), ConfirmedAt: &confirmed}
	f.users[id] = user

	return user
}

func (f *fakeGoTrue) failNext(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = status
}

func (f *fakeGoTrue) recorded() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recordedRequest(nil), f.requests...)
}

func (f *fakeGoTrue) lastRequest(t *testing.T) recordedRequest {
	t.Helper()

	all := f.recorded()
	if len(all) == 0 {
		t.Fatal("the component made no call to the identity provider at all")
	}

	return all[len(all)-1]
}

func (f *fakeGoTrue) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	body := string(raw)

	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Token:  strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "),
		Body:   body,
	})
	fail := f.fail
	f.fail = 0
	f.mu.Unlock()

	if fail != 0 {
		http.Error(w, `{"msg":"refused"}`, fail)

		return
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/invite":
		f.invite(w, body)
	case r.Method == http.MethodGet && r.URL.Path == "/admin/users":
		f.list(w, r.URL.Query().Get("filter"))
	case strings.HasPrefix(r.URL.Path, "/admin/users/"):
		f.user(w, r.Method, strings.TrimPrefix(r.URL.Path, "/admin/users/"), body)
	default:
		http.Error(w, `{"msg":"not found"}`, http.StatusNotFound)
	}
}

func (f *fakeGoTrue) invite(w http.ResponseWriter, body string) {
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		http.Error(w, `{"msg":"bad request"}`, http.StatusBadRequest)

		return
	}

	f.mu.Lock()
	id := "00000000-0000-4000-8000-" + fakeSuffix(len(f.users))
	// Lowercased exactly as GoTrue does, which is why the component has to
	// lowercase before it filters.
	user := &fakeUser{ID: id, Email: strings.ToLower(payload.Email)}
	f.users[id] = user
	f.mu.Unlock()

	writeFakeUser(w, http.StatusOK, user)
}

func (f *fakeGoTrue) list(w http.ResponseWriter, filter string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	matched := make([]*fakeUser, 0, len(f.users))
	for _, user := range f.users {
		// Substring, and case-sensitive. Both measured; both load-bearing.
		if strings.Contains(user.Email, filter) {
			matched = append(matched, user)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"users": fakeUserList(matched),
		"aud":   "authenticated",
	})
}

func (f *fakeGoTrue) user(w http.ResponseWriter, method, id, body string) {
	f.mu.Lock()
	user, found := f.users[id]
	if found && method == http.MethodDelete {
		delete(f.users, id)
	}
	f.mu.Unlock()

	if !found {
		http.Error(w, `{"msg":"User not found"}`, http.StatusNotFound)

		return
	}

	switch method {
	case http.MethodGet, http.MethodDelete:
		writeFakeUser(w, http.StatusOK, user)
	case http.MethodPut:
		var payload struct {
			Password string `json:"password"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			http.Error(w, `{"msg":"bad request"}`, http.StatusBadRequest)

			return
		}

		f.mu.Lock()
		user.Password = payload.Password
		f.mu.Unlock()

		writeFakeUser(w, http.StatusOK, user)
	default:
		http.Error(w, `{"msg":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// writeFakeUser answers with the whole GoTrue user document rather than the
// three fields the component passes on, which is what makes the narrowing
// assertions in handlers_test.go mean something.
func writeFakeUser(w http.ResponseWriter, status int, user *fakeUser) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(fakeUserDocument(user))
}

func fakeUserList(users []*fakeUser) []map[string]any {
	out := make([]map[string]any, 0, len(users))
	for _, user := range users {
		out = append(out, fakeUserDocument(user))
	}

	return out
}

func fakeUserDocument(user *fakeUser) map[string]any {
	return map[string]any{
		"id":                  user.ID,
		"email":               user.Email,
		"phone":               "+70000000000",
		"role":                "authenticated",
		"confirmed_at":        user.ConfirmedAt,
		"email_confirmed_at":  user.ConfirmedAt,
		"last_sign_in_at":     user.LastSignInAt,
		"app_metadata":        map[string]any{"provider": "email"},
		"user_metadata":       map[string]any{"clinic_note": "the patient asked to be called Ann"},
		"identities":          []any{},
		"recovery_sent_at":    nil,
		"confirmation_token":  "1f3c9d2e-the-credential-itself",
		"encrypted_password":  "$2a$10$notarealhash",
		"raw_user_meta_data":  map[string]any{},
		"invited_at":          user.ConfirmedAt,
		"updated_at":          user.ConfirmedAt,
		"banned_until":        nil,
		"is_sso_user":         false,
		"is_anonymous":        false,
		"reauthentication_at": nil,
	}
}

func fakeSuffix(n int) string {
	const digits = "0123456789abcdef"

	suffix := []byte("000000000000")
	for i := len(suffix) - 1; i >= 0 && n > 0; i-- {
		suffix[i] = digits[n%16]
		n /= 16
	}

	return string(suffix)
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// TestLookupIsAnExactMatchInsideTheComponent. GoTrue's `?filter=` is a
// substring match — an empty one returns everybody — and exposing it outward
// would put the clinic's directory behind a component whose whole purpose is
// that it holds one. So the substring is an implementation detail of the call
// and the answer is one account or none.
func TestLookupIsAnExactMatchInsideTheComponent(t *testing.T) {
	fake := startFakeGoTrue(t)
	fake.withUser("11111111-0000-4000-8000-000000000001", "anna@clinic.example")
	fake.withUser("11111111-0000-4000-8000-000000000002", "anna.petrova@clinic.example")
	fake.withUser("11111111-0000-4000-8000-000000000003", "anna@clinic.example.org")

	srv := testServer(t, development, fake)

	tests := []struct {
		name   string
		body   string
		status int
		wantID string
	}{
		{
			name:   "the address itself",
			body:   `{"email":"anna@clinic.example"}`,
			status: http.StatusOK,
			wantID: "11111111-0000-4000-8000-000000000001",
		},
		{
			name: "a substring of two addresses",
			// GoTrue would answer with both. Refused before the call is made:
			// it is not an address, and there is no shape of this API that
			// returns a list.
			body:   `{"email":"anna"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "an empty string",
			body:   `{"email":""}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "a bare @",
			body:   `{"email":"@"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "no email member at all",
			body:   `{}`,
			status: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(t, srv, http.MethodPost, "/users/lookup", tc.body)
			if rec.Code != tc.status {
				t.Fatalf("answered %d, want %d; body %s", rec.Code, tc.status, rec.Body)
			}

			if tc.status != http.StatusOK {
				return
			}

			var answer struct {
				Account *struct {
					ID string `json:"id"`
				} `json:"account"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
				t.Fatalf("decoding the answer: %v", err)
			}

			switch {
			case tc.wantID == "" && answer.Account != nil:
				t.Fatalf("found %s, want no account", answer.Account.ID)
			case tc.wantID != "" && answer.Account == nil:
				t.Fatalf("found no account, want %s", tc.wantID)
			case tc.wantID != "" && answer.Account.ID != tc.wantID:
				t.Fatalf("found %s, want %s", answer.Account.ID, tc.wantID)
			}
		})
	}
}

// TestSomebodyElsesLongerAddressIsNotTheOneAskedFor is what the exact match
// inside the component is actually for, and it took a surviving mutation to
// write: replacing the match with `if true` left every other lookup test green,
// because none of their fixtures could make the provider's filter return a row
// that was not the row asked for.
//
// This one can. `?filter=` is a substring match, so asking for
// anna@clinic.example returns the account at anna@clinic.example.org — a
// different person, at an address the caller never named. Without the exact
// match the caller receives their identifier and writes a profile against it.
func TestSomebodyElsesLongerAddressIsNotTheOneAskedFor(t *testing.T) {
	fake := startFakeGoTrue(t)
	fake.withUser("77777777-0000-4000-8000-000000000001", "anna@clinic.example.org")

	srv := testServer(t, development, fake)

	rec := call(t, srv, http.MethodPost, "/users/lookup", `{"email":"anna@clinic.example"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
	}

	var answer struct {
		Account *struct {
			ID string `json:"id"`
		} `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}

	if answer.Account != nil {
		t.Fatalf("the lookup answered with %s, who is at a different address", answer.Account.ID)
	}

	// The control: the provider really did return that row, so the refusal is
	// the exact match doing its work and not a filter that found nothing.
	if len(fake.recorded()) != 1 {
		t.Fatalf("the component made %d calls, want one", len(fake.recorded()))
	}
}

// TestMixedCaseFindsAnExistingAccount is the measured one: `/invite` stores the
// address lowercased and `?filter=` is case-sensitive, so a dashboard that
// types an address the way a human wrote it finds nothing unless the component
// lowercases first.
//
// Both halves are asserted — the account comes back, and the filter actually
// sent was the lowercased form. Without the second, a component that lowercased
// nothing and compared case-insensitively after the fact would pass, and would
// then fail against real GoTrue, which never returned the row at all.
func TestMixedCaseFindsAnExistingAccount(t *testing.T) {
	fake := startFakeGoTrue(t)
	fake.withUser("22222222-0000-4000-8000-000000000001", "anna@clinic.example")

	srv := testServer(t, development, fake)

	rec := call(t, srv, http.MethodPost, "/users/lookup", `{"email":"Anna@Clinic.Example"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
	}

	if !strings.Contains(rec.Body.String(), "22222222-0000-4000-8000-000000000001") {
		t.Fatalf("a mixed-case address found nothing: %s", rec.Body)
	}

	query, err := url.ParseQuery(fake.lastRequest(t).Query)
	if err != nil {
		t.Fatalf("parsing the filter the component sent: %v", err)
	}

	if got := query.Get("filter"); got != "anna@clinic.example" {
		t.Errorf("the component filtered on %q; GoTrue's filter is case-sensitive", got)
	}
}

// TestTheAnswerIsNarrowedToThreeFields. The identity provider's user document
// carries the confirmation token — which is the credential itself — the
// encrypted password, and whatever user metadata a clinic has written into it.
// The component answers with three fields, so none of the rest can reach a
// caller by being forgotten.
func TestTheAnswerIsNarrowedToThreeFields(t *testing.T) {
	fake := startFakeGoTrue(t)
	fake.withUser("33333333-0000-4000-8000-000000000001", "anna@clinic.example")

	srv := testServer(t, development, fake)

	rec := call(t, srv, http.MethodPost, "/users/lookup", `{"email":"anna@clinic.example"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
	}

	var answer struct {
		Account map[string]json.RawMessage `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}

	got := make([]string, 0, len(answer.Account))
	for name := range answer.Account {
		got = append(got, name)
	}
	slices.Sort(got)

	want := []string{"confirmed_at", "id", "last_sign_in_at"}
	if !slices.Equal(got, want) {
		t.Fatalf("the answer carries %v, want exactly %v", got, want)
	}
}

// TestInviteAnswersWithTheNarrowedAccount: the caller needs the identifier of
// the account it just created — it is the primary key of the profile it is
// about to write — and nothing else.
func TestInviteAnswersWithTheNarrowedAccount(t *testing.T) {
	fake := startFakeGoTrue(t)
	srv := testServer(t, development, fake)

	rec := call(t, srv, http.MethodPost, "/invite", `{"email":"New.Patient@clinic.example"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
	}

	var answer struct {
		Account map[string]json.RawMessage `json:"account"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}

	if len(answer.Account) != 3 {
		t.Fatalf("invite answered with %d fields, want the same three a lookup answers with: %s",
			len(answer.Account), rec.Body)
	}

	// Lowercased on the way in too, or the account is created under one
	// spelling and can never be found under another.
	if !strings.Contains(fake.lastRequest(t).Body, `"new.patient@clinic.example"`) {
		t.Errorf("the component invited %s", fake.lastRequest(t).Body)
	}
}

// TestInviteRefusesSomethingThatIsNotAnAddress: the same validation as the
// lookup, because an invite is what locks an address for good.
func TestInviteRefusesSomethingThatIsNotAnAddress(t *testing.T) {
	srv := testServer(t, development, startFakeGoTrue(t))

	for _, body := range []string{`{"email":""}`, `{"email":"anna"}`, `{}`, `{"email":"a b@c.example"}`} {
		rec := call(t, srv, http.MethodPost, "/invite", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invite %s answered %d, want 400", body, rec.Code)
		}
	}
}

// TestTheBatchLookupAnswersTheSameThreeFieldsForEach. The dashboard roster
// needs many accounts at once, and without this it would make one call per row
// through a component that was never designed for it.
func TestTheBatchLookupAnswersTheSameThreeFieldsForEach(t *testing.T) {
	fake := startFakeGoTrue(t)
	fake.withUser("44444444-0000-4000-8000-000000000001", "one@clinic.example")
	fake.withUser("44444444-0000-4000-8000-000000000002", "two@clinic.example")

	srv := testServer(t, development, fake)

	rec := call(t, srv, http.MethodPost, "/users/lookup-batch", `{"ids":[
		"44444444-0000-4000-8000-000000000001",
		"44444444-0000-4000-8000-000000000002",
		"44444444-0000-4000-8000-000000000009"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
	}

	var answer struct {
		Accounts []map[string]json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}

	// Two, not three: an identifier nobody has is absent from the answer
	// rather than being an error for the other two.
	if len(answer.Accounts) != 2 {
		t.Fatalf("answered %d accounts, want 2: %s", len(answer.Accounts), rec.Body)
	}

	for _, account := range answer.Accounts {
		got := make([]string, 0, len(account))
		for name := range account {
			got = append(got, name)
		}
		slices.Sort(got)

		if want := []string{"confirmed_at", "id", "last_sign_in_at"}; !slices.Equal(got, want) {
			t.Fatalf("an account carries %v, want exactly %v", got, want)
		}
	}
}

// TestTheBatchLookupRefusesAnythingThatIsNotAnIdentifier. An identifier that is
// not one would be pasted into a path, and a list without a bound is a fan-out
// somebody else decides the size of.
func TestTheBatchLookupRefusesAnythingThatIsNotAnIdentifier(t *testing.T) {
	srv := testServer(t, development, startFakeGoTrue(t))

	tooMany := `{"ids":["` + strings.Repeat(`44444444-0000-4000-8000-000000000001","`, maxBatchLookup) +
		`44444444-0000-4000-8000-000000000001"]}`

	for _, body := range []string{
		`{"ids":[]}`,
		`{}`,
		`{"ids":["not-a-uuid"]}`,
		`{"ids":["44444444-0000-4000-8000-000000000001","../../admin/users"]}`,
		`{"ids":[""]}`,
		tooMany,
	} {
		rec := call(t, srv, http.MethodPost, "/users/lookup-batch", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("a batch lookup of %.60s answered %d, want 400", body, rec.Code)
		}
	}
}

// TestDeletionNeedsProofFromTheCaller. The component cannot see the `app`
// schema, so it cannot check the condition itself — and the condition is the
// only thing standing between "reuse an address that was burned by a rolled
// back transaction" and "delete any account". The caller states both halves
// explicitly; neither may be inferred from a missing field.
func TestDeletionNeedsProofFromTheCaller(t *testing.T) {
	const id = "55555555-0000-4000-8000-000000000001"

	tests := []struct {
		name    string
		body    string
		status  int
		deleted bool
	}{
		{
			name:    "an invite record and no profile",
			body:    `{"id":"` + id + `","invite_exists":true,"profile_exists":false}`,
			status:  http.StatusNoContent,
			deleted: true,
		},
		{
			name:   "no proof at all",
			body:   `{"id":"` + id + `"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "only half the proof",
			body:   `{"id":"` + id + `","invite_exists":true}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "an account with a profile",
			body:   `{"id":"` + id + `","invite_exists":true,"profile_exists":true}`,
			status: http.StatusConflict,
		},
		{
			name:   "an account nobody invited",
			body:   `{"id":"` + id + `","invite_exists":false,"profile_exists":false}`,
			status: http.StatusConflict,
		},
		{
			name:   "an identifier that is not one",
			body:   `{"id":"anna@clinic.example","invite_exists":true,"profile_exists":false}`,
			status: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := startFakeGoTrue(t)
			fake.withUser(id, "anna@clinic.example")

			rec := call(t, testServer(t, development, fake), http.MethodPost, "/users/delete", tc.body)
			if rec.Code != tc.status {
				t.Fatalf("answered %d, want %d; body %s", rec.Code, tc.status, rec.Body)
			}

			// The refusal is a refusal to call, not a call whose answer was
			// discarded: a nil error is not a witness, and neither is a status.
			called := slices.ContainsFunc(fake.recorded(), func(r recordedRequest) bool {
				return r.Method == http.MethodDelete
			})
			if called != tc.deleted {
				t.Fatalf("the identity provider saw a DELETE = %v, want %v", called, tc.deleted)
			}
		})
	}
}

// TestSettingAPasswordReachesTheProviderOutsideProduction is the control for
// TestSettingAPasswordDoesNotExistInProduction: without it, an operation that
// was broken everywhere would look correctly absent in production.
func TestSettingAPasswordReachesTheProviderOutsideProduction(t *testing.T) {
	const id = "66666666-0000-4000-8000-000000000001"

	fake := startFakeGoTrue(t)
	fake.withUser(id, "doctor@clinic.example")

	srv := testServer(t, development, fake)

	rec := call(t, srv, http.MethodPost, "/users/password",
		`{"id":"`+id+`","password":"a-seeded-password-nobody-uses"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("answered %d, want 204; body %s", rec.Code, rec.Body)
	}

	last := fake.lastRequest(t)
	if last.Method != http.MethodPut || last.Path != "/admin/users/"+id {
		t.Fatalf("the component called %s %s", last.Method, last.Path)
	}

	// The password is in the body, where a URL and an access log cannot see it.
	if !strings.Contains(last.Body, "a-seeded-password-nobody-uses") {
		t.Errorf("the password did not reach the provider: %s", last.Body)
	}
	if strings.Contains(last.Query, "password") {
		t.Errorf("the password reached the query string: %s", last.Query)
	}
}

// TestALookupThatCouldNotSeeThePageItNeedsIsAFailureNotAnAbsence.
//
// `?filter=` is a substring match, so an address that is a prefix of enough
// others fills the page and the exact one may sit on a page nobody read.
// Answering "no account at that address" then is a false negative, and this is
// the one answer where a false negative does damage: it is what tells the
// caller an address is free, and /invite refuses an address GoTrue still holds
// — the dead end deletion exists to clean up. A refusal is recoverable.
func TestALookupThatCouldNotSeeThePageItNeedsIsAFailureNotAnAbsence(t *testing.T) {
	fake := startFakeGoTrue(t)

	// A full page of accounts that all match the filter, and the one asked for
	// is not among them.
	for i := range lookupPageSize {
		fake.withUser(fmt.Sprintf("99999999-0000-4000-8000-%012d", i),
			fmt.Sprintf("anna@clinic.example.%02d.org", i))
	}

	rec := call(t, testServer(t, development, fake), http.MethodPost,
		"/users/lookup", `{"email":"anna@clinic.example"}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("answered %d, want 502; a full page read as 'no such address' is a false negative: %s",
			rec.Code, rec.Body)
	}
}

// TestAWrongProviderAddressIsNotAnEmptyClinic.
//
// A 404 means "no such account" only where the account's identifier is in the
// URL. For the listing and for /invite it means the base address is wrong, and
// reading it the other way turns a mistyped PROVISIONER_GOTRUE_URL into a
// roster that answers "this clinic has nobody" — a misconfiguration wearing the
// clothes of an empty database.
func TestAWrongProviderAddressIsNotAnEmptyClinic(t *testing.T) {
	fake := startFakeGoTrue(t)
	srv := testServer(t, development, fake)

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "a lookup by address", path: "/users/lookup", body: `{"email":"anna@clinic.example"}`},
		{name: "an invitation", path: "/invite", body: `{"email":"anna@clinic.example"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake.failNext(http.StatusNotFound)

			rec := call(t, srv, http.MethodPost, tc.path, tc.body)
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("answered %d, want 502; body %s", rec.Code, rec.Body)
			}
		})
	}
}

// TestEveryCallCarriesAnAdminTokenThisComponentMinted.
//
// The fake answers whether or not a token was presented, so without this the
// whole fast gate stays green with the Authorization header deleted — and the
// integration test that would catch it needs a Docker daemon, which
// scripts/gate/all.sh skips where there is none.
//
// Asserted per operation rather than once: the header is set in one place
// today, and "one place" is a property of the code as it stands rather than a
// guarantee about it.
func TestEveryCallCarriesAnAdminTokenThisComponentMinted(t *testing.T) {
	const id = "88888888-0000-4000-8000-000000000001"

	operations := []struct {
		path string
		body string
	}{
		{path: "/invite", body: `{"email":"anna@clinic.example"}`},
		{path: "/users/lookup", body: `{"email":"anna@clinic.example"}`},
		{path: "/users/lookup-batch", body: `{"ids":["` + id + `"]}`},
		{path: "/users/delete", body: `{"id":"` + id + `","invite_exists":true,"profile_exists":false}`},
		{path: "/users/password", body: `{"id":"` + id + `","password":"a-seeded-password-nobody-uses"}`},
	}

	for _, operation := range operations {
		t.Run(operation.path, func(t *testing.T) {
			fake := startFakeGoTrue(t)
			fake.withUser(id, "anna@clinic.example")

			call(t, testServer(t, development, fake), http.MethodPost, operation.path, operation.body)

			presented := fake.lastRequest(t).Token
			if presented == "" {
				t.Fatal("the call carried no bearer token at all")
			}

			claims := parseWithoutVerifying(t, presented)
			if claims["role"] != adminTokenRole {
				t.Errorf("the token names role %v", claims["role"])
			}

			// The kid testServer's key carries. Without it any well-formed
			// token would satisfy this, including one from somewhere else.
			if kid := tokenKID(t, presented); kid != "admin-key" {
				t.Errorf("the token names key id %q, not the one this component signs with", kid)
			}
		})
	}
}

// TestAProviderFailureIsNotReportedAsSuccess. Every operation here is a write
// somebody else will act on: an invite the API records a profile for, a
// deletion that frees an address. A 200 over a provider that refused would make
// the API's own state a lie.
func TestAProviderFailureIsNotReportedAsSuccess(t *testing.T) {
	fake := startFakeGoTrue(t)
	srv := testServer(t, development, fake)

	fake.failNext(http.StatusUnprocessableEntity)

	rec := call(t, srv, http.MethodPost, "/invite", `{"email":"taken@clinic.example"}`)

	// 502 specifically, not "some 5xx": the difference between "we are broken"
	// and "the identity provider is" is the first thing an operator needs, and
	// it is a decision routes.go records rather than an accident of which
	// branch was written first.
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("a refused invite answered %d, want 502", rec.Code)
	}

	// And the provider's own words do not reach the caller: they are GoTrue's
	// vocabulary, not this API's, and they describe accounts the caller may
	// have no business knowing about.
	if strings.Contains(rec.Body.String(), "refused") {
		t.Errorf("the provider's message reached the caller: %s", rec.Body)
	}
}

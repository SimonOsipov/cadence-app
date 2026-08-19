package identity

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// stubProvisioner answers a roster lookup and nothing else: the three write
// operations are not what this file measures, and a stub that implemented them
// would be a stub somebody could accidentally rely on.
type stubProvisioner struct {
	accounts []Account
	err      error
	asked    [][]string
}

func (p *stubProvisioner) Invite(context.Context, string) (Account, error) {
	panic("the roster does not invite")
}

func (p *stubProvisioner) Lookup(context.Context, string) (*Account, error) {
	panic("the roster does not look up an address")
}

func (p *stubProvisioner) Delete(context.Context, Deletion) error {
	panic("the roster does not delete")
}

func (p *stubProvisioner) LookupBatch(_ context.Context, ids []string) ([]Account, error) {
	p.asked = append(p.asked, ids)

	return p.accounts, p.err
}

func statesOf(rows []RosterRow) map[string]InviteState {
	states := make(map[string]InviteState, len(rows))
	for _, row := range rows {
		states[row.UserID] = row.Invite
	}

	return states
}

// The answer is a set, not a sequence: the component drops identifiers it holds
// no account for, so the second account can belong to the third row.
func TestEachRowGetsTheStateOfItsOwnAccount(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Hour)
	stale := now.Add(-InviteLinkLifetime - time.Hour)

	rows := []RosterRow{
		{UserID: "row-accepted"},
		{UserID: "row-nobody-has"},
		{UserID: "row-expired"},
		{UserID: "row-pending"},
	}

	roster := &Roster{provisioner: &stubProvisioner{accounts: []Account{
		{ID: "row-pending", InvitedAt: &recent},
		{ID: "row-expired", InvitedAt: &stale},
		{ID: "row-accepted", InvitedAt: &stale, ConfirmedAt: &recent},
	}}}

	roster.fillInviteStates(context.Background(), rows, now)

	want := map[string]InviteState{
		"row-accepted":   InviteAccepted,
		"row-nobody-has": InviteUnknown,
		"row-expired":    InviteExpired,
		"row-pending":    InvitePending,
	}
	if got := statesOf(rows); !equalStates(got, want) {
		t.Errorf("states = %v, want %v", got, want)
	}
}

// The page is what the caller came for. A provisioner that is down costs the
// states, not the roster.
func TestAProvisionerThatDoesNotAnswerLeavesTheRosterStanding(t *testing.T) {
	rows := []RosterRow{{UserID: "one"}, {UserID: "two"}}

	roster := &Roster{provisioner: &stubProvisioner{err: errors.New("the provisioner is not answering")}}
	roster.fillInviteStates(context.Background(), rows, time.Now())

	for _, row := range rows {
		if row.Invite != InviteUnknown {
			t.Errorf("%s came back %q, want %q", row.UserID, row.Invite, InviteUnknown)
		}
	}
}

// The document generator builds this context with no dependencies at all, and
// an API assembled without a provisioner still answers the page.
func TestNoProvisionerAtAllIsAnUnknownStateAndNotAPanic(t *testing.T) {
	rows := []RosterRow{{UserID: "one"}}

	(&Roster{}).fillInviteStates(context.Background(), rows, time.Now())

	if rows[0].Invite != InviteUnknown {
		t.Errorf("state = %q, want %q", rows[0].Invite, InviteUnknown)
	}
}

// One call for the page, which is the entire reason the batch operation exists.
func TestThePageIsLookedUpInOneCall(t *testing.T) {
	rows := []RosterRow{{UserID: "one"}, {UserID: "two"}, {UserID: "three"}}
	stub := &stubProvisioner{}

	(&Roster{provisioner: stub}).fillInviteStates(context.Background(), rows, time.Now())

	if len(stub.asked) != 1 {
		t.Fatalf("the page was looked up in %d calls, want one", len(stub.asked))
	}
	if got, want := len(stub.asked[0]), len(rows); got != want {
		t.Errorf("the call carried %d identifiers, want %d", got, want)
	}
}

// A page of nobody is a state the clinic starts in, and a call carrying no
// identifiers is one the component refuses.
func TestAnEmptyPageIsNotACallAtAll(t *testing.T) {
	stub := &stubProvisioner{}

	(&Roster{provisioner: stub}).fillInviteStates(context.Background(), nil, time.Now())

	if len(stub.asked) != 0 {
		t.Errorf("an empty page was looked up anyway: %v", stub.asked)
	}
}

func equalStates(got, want map[string]InviteState) bool {
	if len(got) != len(want) {
		return false
	}

	for id, state := range want {
		if got[id] != state {
			return false
		}
	}

	return true
}

// The page and the batch are one bound. A page the identity provider cannot be
// asked about in one call is a page whose states all come back unknown, and the
// method is exported past the schema that pins the route's maximum.
func TestAPageLargerThanOneLookupIsRefused(t *testing.T) {
	roster := &Roster{provisioner: &stubProvisioner{}}

	_, err := roster.Patients(context.Background(), database.Caller{}, "", MaxPageSize+1)
	if !errors.Is(err, ErrNotAPageSize) {
		t.Errorf("a page of %d answered %v, want ErrNotAPageSize", MaxPageSize+1, err)
	}
}

// And the route's own schema does not promise more than that: a maximum raised
// there without this constant moving would ask for a page nothing can look up.
func TestTheRouteAsksForNoMoreThanAPage(t *testing.T) {
	field, ok := reflect.TypeOf(OverviewInput{}).FieldByName("Limit")
	if !ok {
		t.Fatal("the overview input has no Limit field")
	}

	if got, want := field.Tag.Get("maximum"), strconv.Itoa(MaxPageSize); got != want {
		t.Errorf("the route's schema pins maximum %q, want %q", got, want)
	}
}

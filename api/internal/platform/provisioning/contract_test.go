package provisioning

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// The three bounds of one request, ordered from the inside out.
//
// An outward call made inside a service transaction leaves the session idle in
// transaction for its whole duration: holding its locks and running nothing. A
// call that may take longer than the database's idle limit is a session Postgres
// terminates while the invitation is in flight — the transaction is gone and the
// email has gone out, which is the one ordering here that loses work somebody
// outside this system can see.
//
// The request deadline is outermost because it is also the only budget a
// connection acquisition has: pgxpool.Acquire waits for a free connection or for
// the context, and it has no timeout of its own. If it fired first, every
// failure would read "the deadline" and name none of the things that actually
// went slow.
//
// Compared rather than written down, and each constant read from the package
// that owns it rather than copied, because a copy is how three numbers stop
// being one ordering.
func TestAnOutwardCallCannotOutlastTheTransactionItIsMadeIn(t *testing.T) {
	if callTimeout >= database.ServiceIdleInTransactionTimeout {
		t.Errorf("one call is bounded at %s and a service transaction may be idle for %s: "+
			"Postgres would end the session while the invitation is in flight",
			callTimeout, database.ServiceIdleInTransactionTimeout)
	}

	if database.ServiceIdleInTransactionTimeout >= httpserver.RequestDeadline {
		t.Errorf("a service transaction may be idle for %s inside a request bounded at %s: "+
			"the request ends first and what went slow is never named",
			database.ServiceIdleInTransactionTimeout, httpserver.RequestDeadline)
	}
}

// The component's surface is spelled in two places — its own package main, which
// cannot be imported from here, and this client. This reads the component's
// source and fails when the header, a path or the envelope of an answer stops
// agreeing.
//
// Each of the three fails silently in its own way. A renamed header means every
// call is refused with the component's fixed 401, which by design says nothing
// about which check failed. A renamed path answers 404. And a renamed envelope
// key is the quietest of all: json.Unmarshal does not object to a key it never
// finds, so Lookup would answer "nobody has that address" — the false negative
// that sends the caller to /invite, which then refuses an address GoTrue still
// holds. That is the dead end the delete operation exists to clean up.
func TestTheClientSpeaksTheSurfaceTheComponentServes(t *testing.T) {
	root := moduleRoot(t)

	secretSource, err := os.ReadFile(filepath.Join(root, "cmd", "provisioner", "secret.go"))
	if err != nil {
		t.Fatalf("reading the component's guard: %v", err)
	}

	if want := `secretHeader = "` + secretHeader + `"`; !strings.Contains(string(secretSource), want) {
		t.Errorf("cmd/provisioner does not declare %s, so every call this client makes is "+
			"refused with a 401 that names no reason", want)
	}

	routesSource, err := os.ReadFile(filepath.Join(root, "cmd", "provisioner", "routes.go"))
	if err != nil {
		t.Fatalf("reading the component's routes: %v", err)
	}

	for _, path := range []string{invitePath, lookupPath, lookupBatchPath, deletePath} {
		if want := `mux.Post("` + path + `"`; !strings.Contains(string(routesSource), want) {
			t.Errorf("cmd/provisioner mounts no %s, so this client calls a path that answers 404", path)
		}
	}

	// Three spellings of each envelope key: the constant, the struct tag that has
	// to repeat it because a tag cannot be one, and the component's own.
	for key, tag := range map[string]string{
		accountKey:  tagOf(t, reflect.TypeOf(accountAnswer{}), "Account"),
		accountsKey: tagOf(t, reflect.TypeOf(accountsAnswer{}), "Accounts"),
	} {
		if tag != key {
			t.Errorf("the envelope is decoded by the tag %q while the constant says %q", tag, key)
		}

		if want := `map[string]any{"` + key + `":`; !strings.Contains(string(routesSource), want) {
			t.Errorf("cmd/provisioner answers under no %q key, so this client decodes nothing "+
				"out of the answer and reports it as an absence", key)
		}
	}

	// And the fields inside the envelope, one level down and failing the same
	// silent way. confirmed_at is the one that matters most: decoded as nothing,
	// an account somebody has already confirmed reads as an invitation nobody
	// opened, which is what the onboarding rules turn on.
	accountSource, err := os.ReadFile(filepath.Join(root, "cmd", "provisioner", "gotrue.go"))
	if err != nil {
		t.Fatalf("reading the component's account type: %v", err)
	}

	fields := reflect.TypeOf(account{})
	for i := range fields.NumField() {
		tag := fields.Field(i).Tag.Get("json")
		if want := "`json:\"" + tag + "\"`"; !strings.Contains(string(accountSource), want) {
			t.Errorf("cmd/provisioner's account carries no %s, so %s decodes to nothing and "+
				"the answer is wrong rather than absent", want, fields.Field(i).Name)
		}
	}
}

func tagOf(t *testing.T, envelope reflect.Type, name string) string {
	t.Helper()

	field, ok := envelope.FieldByName(name)
	if !ok {
		t.Fatalf("%s has no field %s, so this compared nothing", envelope, name)
	}

	return field.Tag.Get("json")
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("reading the working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}

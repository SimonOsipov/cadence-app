package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Every route, and through the stack the composition root builds rather than
// through the middleware on its own: a deadline registered where the routes
// cannot see it is a deadline nothing carries.
func TestEveryRequestCarriesADeadline(t *testing.T) {
	server := New(Config{Port: "0"}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var remaining time.Duration

	var carried bool

	server.Router.Get("/probe", func(_ http.ResponseWriter, r *http.Request) {
		var at time.Time

		at, carried = r.Context().Deadline()
		remaining = time.Until(at)
	})

	server.Router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil))

	if !carried {
		t.Fatal("the handler ran on a context with no deadline: a request is bounded by the " +
			"caller's patience and a pool acquisition waits as long as that lasts")
	}

	// A window rather than an equality: the clock moves between the middleware
	// and the handler. What it has to exclude is the deadline being some other
	// number — the shutdown timeout, say, or one somebody wrote twice.
	if remaining > RequestDeadline || remaining < RequestDeadline-time.Second {
		t.Errorf("the handler had %s left of a %s deadline", remaining, RequestDeadline)
	}
}

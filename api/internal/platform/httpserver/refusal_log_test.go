package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// serveWithLog runs one request through the two middleware the error writer
// reads from — the logger and the request id — and returns what was written to
// both the response and the log.
func serveWithLog(t *testing.T, r *http.Request, handler http.HandlerFunc) (*httptest.ResponseRecorder, []map[string]any) {
	t.Helper()

	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	w := httptest.NewRecorder()
	chimw.RequestID(withLogger(logger)(handler)).ServeHTTP(w, r)

	return w, logLines(t, logged.Bytes())
}

func logLines(t *testing.T, written []byte) []map[string]any {
	t.Helper()

	lines := make([]map[string]any, 0, 2)
	for _, raw := range bytes.Split(bytes.TrimSpace(written), []byte("\n")) {
		if len(raw) == 0 {
			continue
		}

		var line map[string]any
		if err := json.Unmarshal(raw, &line); err != nil {
			t.Fatalf("unmarshal log line %q: %v", raw, err)
		}

		lines = append(lines, line)
	}

	return lines
}

func onlyLine(t *testing.T, lines []map[string]any) map[string]any {
	t.Helper()

	if len(lines) != 1 {
		t.Fatalf("the request produced %d log lines, want one: %v", len(lines), lines)
	}

	return lines[0]
}

// A refusal below 500 keeps its detail in the response, so nothing is being
// preserved here — what the line adds is the reason. Without it a systematically
// broken cursor is a flat run of 400s in the access log with nothing to say why.
func TestAClientRefusalSaysWhyInTheLog(t *testing.T) {
	const detail = "Ссылка на следующую страницу устарела."

	r := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview?cursor=nonsense", nil)

	_, lines := serveWithLog(t, r, func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{Status: http.StatusBadRequest, Type: ProblemValidation, Detail: detail})
	})

	line := onlyLine(t, lines)
	if line["detail"] != detail {
		t.Errorf("detail = %v, want the reason the caller was refused", line["detail"])
	}
	if line["status"] != float64(http.StatusBadRequest) {
		t.Errorf("status = %v, want %d", line["status"], http.StatusBadRequest)
	}
	if id, _ := line["request_id"].(string); id == "" {
		t.Errorf("the line carries no request id, so it cannot be tied to the response: %v", line)
	}

	// Not a fault of this server: a 400 at ERROR trains its readers to ignore ERROR.
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", line["level"])
	}
}

// A 403 is the other half of the same complaint: the caller is told what to do
// and the operator is told nothing at all.
func TestAForbiddenRefusalSaysWhyInTheLog(t *testing.T) {
	const detail = "Аккаунт ещё не заведён в клинике."

	r := httptest.NewRequest(http.MethodPost, "/v1/me/session", nil)

	_, lines := serveWithLog(t, r, func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{Status: http.StatusForbidden, Type: ProblemForbidden, Detail: detail})
	})

	if got := onlyLine(t, lines)["detail"]; got != detail {
		t.Errorf("detail = %v, want the reason the caller was refused", got)
	}
}

// A refusal that carries neither a detail nor field errors has nothing to say
// beyond the access line, which already records the status and the path.
func TestARefusalWithNothingToSayWritesNoLine(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/nowhere", nil)

	_, lines := serveWithLog(t, r, func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{Status: http.StatusNotFound, Type: ProblemNotFound})
	})

	if len(lines) != 0 {
		t.Errorf("a refusal with no reason still wrote %d lines: %v", len(lines), lines)
	}
}

// The caller hung up. Whatever status the handler settled on goes to a closed
// socket, and no engineer has anything to do about it — an ERROR here is an
// alarm about somebody navigating away. The route cannot tell the difference,
// and every route would have to.
func TestACallerWhoWentAwayIsNotAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil).WithContext(ctx)

	_, lines := serveWithLog(t, r, func(w http.ResponseWriter, r *http.Request) {
		cancel()

		WriteProblem(w, r, Problem{
			Status: http.StatusInternalServerError,
			Type:   ProblemInternal,
			Detail: "reading the roster: context canceled",
		})
	})

	line := onlyLine(t, lines)
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO: nobody is left to be told, so nobody should be alarmed", line["level"])
	}
	if line["msg"] != msgCallerGone {
		t.Errorf("msg = %v, want %q", line["msg"], msgCallerGone)
	}
	if line["detail"] != "reading the roster: context canceled" {
		t.Errorf("detail = %v, want the cause kept", line["detail"])
	}
}

// The demotion is about the caller, not about the status: a 500 on a request
// nobody abandoned is the one line that has to stay an alarm.
func TestAServerErrorOnALiveRequestIsStillAnError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil)

	_, lines := serveWithLog(t, r, func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{
			Status: http.StatusInternalServerError,
			Type:   ProblemInternal,
			Detail: `pq: relation "app.dose_events" does not exist`,
		})
	})

	if got := onlyLine(t, lines)["level"]; got != "ERROR" {
		t.Errorf("level = %v, want ERROR", got)
	}
}

// A request that ran out of time is not a caller who left: the deadline is this
// server's own bound, the 503 it produces is a fault to answer for, and pgx
// reports it as DeadlineExceeded rather than Canceled.
func TestARequestThatRanOutOfTimeIsStillAnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	r := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil).WithContext(ctx)

	_, lines := serveWithLog(t, r, func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{
			Status: http.StatusServiceUnavailable,
			Type:   ProblemUnavailable,
			Detail: "reading the roster: context deadline exceeded",
		})
	})

	if got := onlyLine(t, lines)["level"]; got != "ERROR" {
		t.Errorf("level = %v, want ERROR", got)
	}
}

// The access line is where an error rate is read from, and a 500 written to a
// socket the caller closed inflates it. Marked rather than restated as some
// other status: this line says what was written.
func TestTheAccessLineMarksACallerWhoWentAway(t *testing.T) {
	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil).WithContext(ctx)

	requestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		w.WriteHeader(http.StatusInternalServerError)
	})).ServeHTTP(httptest.NewRecorder(), r)

	line := onlyLine(t, logLines(t, logged.Bytes()))
	if line["caller_gone"] != true {
		t.Errorf("the access line does not mark the abandoned request: %v", line)
	}
	if line["status"] != float64(http.StatusInternalServerError) {
		t.Errorf("status = %v, want the status that was actually written", line["status"])
	}
}

func TestTheAccessLineOfAnAnsweredRequestCarriesNoSuchMark(t *testing.T) {
	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	r := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil)

	requestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(httptest.NewRecorder(), r)

	if _, marked := onlyLine(t, logLines(t, logged.Bytes()))["caller_gone"]; marked {
		t.Error("a request nobody abandoned is marked as abandoned")
	}
}

// The route is not where this is decided, so the route's own path has to be the
// one measured: a handler that gives up because the caller did returns an error
// no context can classify, and huma writes it as a 500 through a hook of its own
// rather than through WriteProblem.
func TestACallerWhoWentAwayIsNotAnErrorOnTheHumaPath(t *testing.T) {
	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(withLogger(logger))

	huma.Register(NewAPI(router), huma.Operation{
		OperationID: "probe-abandoned",
		Method:      http.MethodGet,
		Path:        "/v1/abandoned",
		Summary:     "Probe whose caller hangs up mid-request",
	}, func(context.Context, *struct{}) (*probeOutput, error) {
		cancel()

		return nil, huma.Error500InternalServerError("reading the roster: context canceled")
	})

	router.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/v1/abandoned", nil).WithContext(ctx),
	)

	line := onlyLine(t, logLines(t, logged.Bytes()))
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", line["level"])
	}
	if line["msg"] != msgCallerGone {
		t.Errorf("msg = %v, want %q", line["msg"], msgCallerGone)
	}
}

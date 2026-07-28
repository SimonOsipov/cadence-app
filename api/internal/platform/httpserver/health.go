package httpserver

import (
	"context"
	"log/slog"
	"net/http"
)

// Health returns the handler for /healthz, the unauthenticated availability
// endpoint polled by the external uptime monitor.
//
// probe is run on every request and reports whether the API's dependencies are
// reachable; a nil probe means the endpoint answers for the process alone. A
// failing probe answers 503 and logs the cause — the response body stays
// deliberately opaque, since anyone on the internet can call this route.
func Health(probe func(context.Context) error, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if probe != nil {
			if err := probe(r.Context()); err != nil {
				logger.Error("health probe failed", "error", err)
				WriteProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Type: ProblemUnhealthy})

				return
			}
		}

		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

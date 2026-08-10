package httpserver

import (
	"context"
	"log/slog"
	"net/http"
)

type loggerKey struct{}

// withLogger puts the server's logger on the request context.
//
// It is how the error writer reaches a logger without being handed one at every
// call site, and without a package-level variable that tests would have to take
// turns with. A request that never passed through this middleware falls back to
// the default logger rather than losing the line.
func withLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), loggerKey{}, logger)))
		})
	}
}

func loggerFrom(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}

	return slog.Default()
}

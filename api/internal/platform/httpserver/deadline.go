package httpserver

import (
	"context"
	"net/http"
	"time"
)

// Without a deadline a handler's only bound is the client's patience:
// ReadTimeout and WriteTimeout end the connection without cancelling the context
// the handler runs under, so the work carries on with nobody left to answer.
// What actually waits is the connection pool — pgxpool.Acquire blocks until a
// connection frees up or the context ends, and it has no timeout of its own —
// which is why this arrived with the service pool's explicit MaxConns rather
// than after it.
//
// It only cancels and writes no status of its own: a cancelled handler already
// answers in this package's one error shape, and a second WriteHeader would add
// nothing but a "superfluous" line in the log.
func deadline(within time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), within)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

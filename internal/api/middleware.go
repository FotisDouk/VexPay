package api

import (
	"log"
	"net/http"
	"time"
)

// withRequestTimeout bounds the lifetime of every request context.
func withRequestTimeout(d time.Duration, next http.Handler) http.Handler {
	if d <= 0 {
		return next
	}
	return http.TimeoutHandler(next, d, `{"error":"request timeout"}`)
}

// withRecover turns a panic in a handler into a 500 instead of crashing the
// server, and logs it. This keeps a single bad request from taking down a
// gateway that is actively settling payments.
func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered on %s %s: %v", r.Method, r.URL.Path, rec)
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

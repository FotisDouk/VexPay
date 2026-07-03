package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/vexarnetwork/vexpay/internal/auth"
)

type principalCtxKey struct{}

// authed wraps a handler so it only runs for authenticated callers. The key may
// be passed as "Authorization: Bearer <key>" or the "X-API-Key" header.
func (s *Server) authed(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := extractAPIKey(r)
		if raw == "" {
			writeError(w, http.StatusUnauthorized, "missing API key")
			return
		}
		principal, ok := s.deps.Auth.Authenticate(raw)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		ctx := context.WithValue(r.Context(), principalCtxKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractAPIKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(auth.Principal)
	return p, ok
}

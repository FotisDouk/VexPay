// Package api exposes VexPay's HTTP surface: the REST API, health checks, and
// (later) the embedded dashboard and hosted checkout.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/vexarnetwork/vexpay/internal/config"
	"github.com/vexarnetwork/vexpay/internal/store"
	"github.com/vexarnetwork/vexpay/internal/version"
)

// Server wires configuration and dependencies into an http.Handler.
type Server struct {
	cfg   config.Config
	store store.Store
	mux   *http.ServeMux
}

// New constructs a Server and registers routes.
func New(cfg config.Config, st store.Store) *Server {
	s := &Server{cfg: cfg, store: st, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the root http.Handler with global middleware applied.
//
// recover sits inside the timeout handler because TimeoutHandler runs the next
// handler in its own goroutine; a panic must be recovered in that same
// goroutine.
func (s *Server) Handler() http.Handler {
	return withRequestTimeout(s.cfg.RequestTimeout, withRecover(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/readyz", s.handleReady)
	s.mux.HandleFunc("/version", s.handleVersion)
}

// handleHealth is a liveness probe: the process is up and serving.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"env":    s.cfg.Env,
	})
}

// handleReady is a readiness probe: dependencies (the store) are reachable.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
			"store":  "unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": version.Version,
		"commit":  version.Commit,
		"built":   version.BuildDate,
	})
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"error": "method not allowed",
	})
}

// Package api exposes VexPay's HTTP surface: the REST API, health checks, and
// (later) the embedded dashboard and hosted checkout.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/vexarnetwork/vexpay/internal/auth"
	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/chain/mock"
	"github.com/vexarnetwork/vexpay/internal/config"
	"github.com/vexarnetwork/vexpay/internal/dashboard"
	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/store"
	"github.com/vexarnetwork/vexpay/internal/version"
)

// Deps are the dependencies the HTTP layer needs.
type Deps struct {
	Config   config.Config
	Store    store.Store
	Invoices *invoice.Service
	Chains   *chain.Registry
	Auth     *auth.Store
	// Sandbox is the mock adapter used by the payment simulator. Nil disables
	// the sandbox endpoints.
	Sandbox *mock.Adapter
}

// Server wires dependencies into an http.Handler.
type Server struct {
	deps Deps
	mux  *http.ServeMux
}

// New constructs a Server and registers routes.
func New(deps Deps) *Server {
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the root http.Handler with global middleware applied.
//
// recover sits inside the timeout handler because TimeoutHandler runs the next
// handler in its own goroutine; a panic must be recovered in that same
// goroutine.
func (s *Server) Handler() http.Handler {
	return withRequestTimeout(s.deps.Config.RequestTimeout, withRecover(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/readyz", s.handleReady)
	s.mux.HandleFunc("/version", s.handleVersion)

	s.mux.Handle("/v1/invoices", s.authed(s.handleInvoicesCollection))
	s.mux.Handle("/v1/invoices/", s.authed(s.handleInvoiceItem))

	if s.deps.Sandbox != nil {
		s.mux.Handle("/v1/sandbox/pay/", s.authed(s.handleSandboxPay))
	}

	if s.deps.Config.EnableDashboard {
		s.mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", dashboard.Handler()))
		s.mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
		})
	}
}

// handleHealth is a liveness probe: the process is up and serving.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"env":    s.deps.Config.Env,
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
	if err := s.deps.Store.Ping(ctx); err != nil {
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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

package reconcile

// Read-only Broker Reconcile HTTP API (Phase 6.5.9.2.1.3.2).
//
// Handler flow:
//   HTTP → LocalOrderSource (optional) → Service.ReconcileMany → View DTO
//
// The API layer must NOT implement Broker Query, Compare, divergence generation,
// repair, retry, cancel, or order update. It only orchestrates Service + mapping.

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const brokerReconcilePath = "/api/execution/broker-reconcile"

// LocalOrderSource supplies candidate LocalOrderSnapshot values for observation.
// Implementations must be read-only (no Order/Fill/Position writes).
// Nil source ⇒ empty observation (no auto-created snapshots).
type LocalOrderSource interface {
	ListCandidates() ([]LocalOrderSnapshot, error)
}

// Handler serves GET /api/execution/broker-reconcile as a read-only visibility adapter.
type Handler struct {
	Service *Service
	Source  LocalOrderSource
	Now     func() time.Time
}

// NewHandler constructs a production/test handler. Nil Service/Source yields empty READY.
func NewHandler(svc *Service, source LocalOrderSource) *Handler {
	return &Handler{Service: svc, Source: source}
}

// ServeHTTP serves GET /api/execution/broker-reconcile only.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewHandler(nil, nil)
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != brokerReconcilePath {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}

	var locals []LocalOrderSnapshot
	if h.Source != nil {
		got, err := h.Source.ListCandidates()
		if err != nil {
			// Observation error only — never mutate orders / auto-create snapshots.
			writeJSON(w, http.StatusOK, ObservationErrorView(now, err.Error()))
			return
		}
		locals = got
	}
	if len(locals) == 0 {
		writeJSON(w, http.StatusOK, EmptyBrokerReconcileView(now))
		return
	}

	if h.Service == nil {
		writeJSON(w, http.StatusOK, ObservationErrorView(now, "reconcile service not configured"))
		return
	}

	// Align Service clock with handler clock for deterministic observation.
	opt := h.Service.Options
	opt.Now = now
	svc := &Service{
		Provider: h.Service.Provider,
		Metrics:  h.Service.Metrics,
		Options:  opt,
	}

	results := svc.ReconcileMany(locals)
	obs := make([]OrderObservation, len(locals))
	for i := range locals {
		obs[i] = OrderObservation{OrderID: locals[i].OrderID, Result: results[i]}
	}
	writeJSON(w, http.StatusOK, MapBrokerReconcileView(now, obs))
}

// RegisterRoutes mounts the read-only route on an http.ServeMux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	if h == nil {
		h = NewHandler(nil, nil)
	}
	mux.Handle(brokerReconcilePath, h)
}

// AssetMiddleware mounts /api/execution/broker-reconcile on Wails AssetServer.
func AssetMiddleware(next http.Handler) http.Handler {
	h := NewHandler(nil, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/execution/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AssetMiddlewareWithHandler mounts a preconfigured handler (tests / DI).
func AssetMiddlewareWithHandler(h *Handler) func(http.Handler) http.Handler {
	if h == nil {
		h = NewHandler(nil, nil)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/execution/") {
				h.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

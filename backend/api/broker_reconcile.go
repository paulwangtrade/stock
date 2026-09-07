package api

import (
	"net/http"

	"go-stock/backend/execution/reconcile"
)

// BrokerReconcileAssetMiddleware mounts GET /api/execution/broker-reconcile
// on the Wails AssetServer. The handler is read-only observation; with no
// LocalOrderSource wired it returns an empty READY view (no auto snapshots).
func BrokerReconcileAssetMiddleware(next http.Handler) http.Handler {
	return reconcile.AssetMiddleware(next)
}

// RegisterBrokerReconcileRoutes mounts the read-only route for httptest/server.
func RegisterBrokerReconcileRoutes(mux *http.ServeMux) {
	reconcile.RegisterRoutes(mux, reconcile.NewHandler(nil, nil))
}

// RegisterBrokerReconcileHandler mounts an injected reconcile handler (tests).
func RegisterBrokerReconcileHandler(mux *http.ServeMux, h *reconcile.Handler) {
	reconcile.RegisterRoutes(mux, h)
}

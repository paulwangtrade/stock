package api

import (
	"net/http"

	"go-stock/backend/exitwatch"
)

const exitWatchPath = "/api/exit-watch"

// ExitWatchHandler serves the shared read-only exit observation list.
type ExitWatchHandler struct{}

func NewExitWatchHandler() *ExitWatchHandler {
	return &ExitWatchHandler{}
}

// RegisterExitWatchRoutes mounts GET /api/exit-watch.
func RegisterExitWatchRoutes(mux *http.ServeMux) {
	mux.Handle(exitWatchPath, NewExitWatchHandler())
}

// ExitWatchAssetMiddleware mounts /api/exit-watch on the Wails asset server.
func ExitWatchAssetMiddleware(next http.Handler) http.Handler {
	h := NewExitWatchHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == exitWatchPath {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *ExitWatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != exitWatchPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok": false, "code": 405, "message": "GET required",
		})
		return
	}
	sources, ok := exitwatch.ParseSource(r.URL.Query().Get("source"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "code": 400, "reason": "invalid_source",
			"message": "来源仅支持 paper_sim、external_mirror 或 all",
		})
		return
	}
	view := exitwatch.Build(exitwatch.Options{Sources: sources})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "code": 0, "exit_watch": view,
	})
}

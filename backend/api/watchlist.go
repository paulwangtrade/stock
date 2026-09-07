package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/opportunity"
)

// WatchlistHandler serves GET /api/watchlist (read-only watching list).
type WatchlistHandler struct{}

func NewWatchlistHandler() *WatchlistHandler {
	return &WatchlistHandler{}
}

func (h *WatchlistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != "/api/watchlist" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}

	q := r.URL.Query()
	var accountID uint
	if raw := strings.TrimSpace(q.Get("account_id")); raw != "" {
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			accountID = uint(v)
		}
	}
	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{
		AccountID: accountID,
		Limit:     limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":      0,
		"ok":        true,
		"watchlist": view,
	})
}

// RegisterWatchlistRoutes mounts GET /api/watchlist.
func RegisterWatchlistRoutes(mux *http.ServeMux) {
	mux.Handle("/api/watchlist", NewWatchlistHandler())
}

// WatchlistAssetMiddleware mounts /api/watchlist on Wails AssetServer.
func WatchlistAssetMiddleware(next http.Handler) http.Handler {
	h := NewWatchlistHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "/api/watchlist" {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

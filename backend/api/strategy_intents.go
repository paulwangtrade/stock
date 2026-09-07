package api

import (
	"errors"
	"net/http"
	"strings"

	"go-stock/backend/strategyintent"
)

// StrategyIntentsHandler serves Phase13 Strategy Intent APIs.
// B1-A: GET list/detail. B1-B: create/update/submit/approve (no promote/AI).
type StrategyIntentsHandler struct{}

func NewStrategyIntentsHandler() *StrategyIntentsHandler {
	return &StrategyIntentsHandler{}
}

func (h *StrategyIntentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewStrategyIntentsHandler()
	}
	path := strings.TrimSuffix(r.URL.Path, "/")

	switch {
	case path == "/api/strategy/intents":
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			writeIntentMethodNotAllowed(w)
		}
	case strings.HasPrefix(path, "/api/strategy/intents/"):
		rest := strings.TrimPrefix(path, "/api/strategy/intents/")
		rest = strings.Trim(rest, "/")
		if rest == "" || strings.Count(rest, "/") > 1 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		parts := strings.Split(rest, "/")
		id := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				h.handleDetail(w, r, id)
			case http.MethodPatch:
				h.handleUpdate(w, r, id)
			default:
				writeIntentMethodNotAllowed(w)
			}
			return
		}
		if r.Method != http.MethodPost {
			writeIntentMethodNotAllowed(w)
			return
		}
		switch parts[1] {
		case "submit":
			h.handleSubmit(w, r, id)
		case "approve":
			h.handleApprove(w, r, id)
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *StrategyIntentsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	out, err := strategyintent.ListIntents()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "list_failed", "message": err.Error()})
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	candidateID := strings.TrimSpace(r.URL.Query().Get("candidate_id"))
	if status != "" || candidateID != "" {
		filtered := make([]strategyintent.ListItem, 0, len(out.Items))
		for _, it := range out.Items {
			if status != "" && it.Status != status {
				continue
			}
			if candidateID != "" && it.CandidateID != candidateID {
				continue
			}
			filtered = append(filtered, it)
		}
		out.Items = filtered
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategyIntentsHandler) handleDetail(w http.ResponseWriter, r *http.Request, id string) {
	out, err := strategyintent.GetIntent(id)
	if err != nil {
		writeIntentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategyIntentsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req strategyintent.CreateDraftRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json", "message": err.Error()})
		return
	}
	out, err := strategyintent.CreateDraft(req)
	if err != nil {
		writeIntentErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *StrategyIntentsHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var req strategyintent.UpdateDraftRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json", "message": err.Error()})
		return
	}
	out, err := strategyintent.UpdateDraft(id, req)
	if err != nil {
		writeIntentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategyIntentsHandler) handleSubmit(w http.ResponseWriter, r *http.Request, id string) {
	out, err := strategyintent.SubmitReview(id)
	if err != nil {
		writeIntentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StrategyIntentsHandler) handleApprove(w http.ResponseWriter, r *http.Request, id string) {
	out, err := strategyintent.Approve(id)
	if err != nil {
		writeIntentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeIntentMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"error":   "method_not_allowed",
		"message": "allowed: GET; POST create/submit/approve; PATCH draft only (B1-B)",
	})
}

func writeIntentErr(w http.ResponseWriter, err error) {
	var nf strategyintent.ErrNotFound
	if errors.As(err, &nf) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found", "message": err.Error()})
		return
	}
	var we strategyintent.WriteError
	if errors.As(err, &we) {
		status := http.StatusBadRequest
		switch we.Code {
		case strategyintent.CodeDraftConflict, strategyintent.CodeCreateNotAllowed:
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]any{"error": we.Code, "message": we.Message})
		return
	}
	var ve strategyintent.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": ve.Code, "message": ve.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal", "message": err.Error()})
}

// RegisterStrategyIntentsRoutes mounts intent routes.
func RegisterStrategyIntentsRoutes(mux *http.ServeMux) {
	h := NewStrategyIntentsHandler()
	mux.Handle("/api/strategy/intents", h)
	mux.Handle("/api/strategy/intents/", h)
}

// StrategyIntentsAssetMiddleware mounts /api/strategy/intents* on Wails AssetServer.
func StrategyIntentsAssetMiddleware(next http.Handler) http.Handler {
	h := NewStrategyIntentsHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "/api/strategy/intents" || strings.HasPrefix(path, "/api/strategy/intents/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

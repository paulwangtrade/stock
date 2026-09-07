package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/research"
)

// ResearchCandidatesHandler serves Phase13 Research Candidate + Explain APIs.
// Paths:
//   GET   /api/research/candidates
//   GET   /api/research/candidates/{id}
//   PATCH /api/research/candidates/{id}           (status / note / tags)
//   GET   /api/research/candidates/{id}/explain
//   PATCH /api/research/candidates/{id}/explain   (manual summary / reason / risk_note)
//   GET   /api/research/explains/{explain_id}     (alias)
// Does NOT implement Promote / TradeCandidate / Draft / Broker / AI.
type ResearchCandidatesHandler struct{}

func NewResearchCandidatesHandler() *ResearchCandidatesHandler {
	return &ResearchCandidatesHandler{}
}

func (h *ResearchCandidatesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewResearchCandidatesHandler()
	}
	path := strings.TrimSuffix(r.URL.Path, "/")

	if strings.HasPrefix(path, "/api/research/explains/") {
		eid := strings.TrimPrefix(path, "/api/research/explains/")
		eid = strings.Trim(eid, "/")
		if eid == "" || strings.Contains(eid, "/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		h.handleExplainByExplainID(w, r, eid)
		return
	}

	switch {
	case path == "/api/research/candidates":
		h.handleList(w, r)
	case strings.HasPrefix(path, "/api/research/candidates/"):
		rest := strings.TrimPrefix(path, "/api/research/candidates/")
		rest = strings.Trim(rest, "/")
		if rest == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if strings.EqualFold(rest, "promote") {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"error":   "not_implemented",
				"message": "promote is not available in explain MVP",
			})
			return
		}
		// .../explain sub-resource
		if strings.HasSuffix(rest, "/explain") {
			candID := strings.TrimSuffix(rest, "/explain")
			candID = strings.Trim(candID, "/")
			if candID == "" || strings.Contains(candID, "/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			h.handleExplain(w, r, candID)
			return
		}
		if strings.Contains(rest, "/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		h.handleByID(w, r, rest)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *ResearchCandidatesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := research.ListQuery{
		TradeDate: strings.TrimSpace(r.URL.Query().Get("trade_date")),
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		Source:    strings.TrimSpace(r.URL.Query().Get("source")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("min_score")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			q.MinScore = v
		}
	}
	out, err := research.ListCandidates(q)
	if err != nil {
		var bad research.ErrBadTradeDate
		if errors.As(err, &bad) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "invalid trade_date",
				"message": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "list_failed",
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ResearchCandidatesHandler) handleByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		h.handleDetail(w, r, id)
	case http.MethodPatch, http.MethodPut:
		h.handleUpdate(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *ResearchCandidatesHandler) handleDetail(w http.ResponseWriter, r *http.Request, id string) {
	out, err := research.GetCandidate(id)
	if err != nil {
		writeResearchErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ResearchCandidatesHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_body", "message": err.Error()})
		return
	}
	var raw map[string]json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json", "message": err.Error()})
			return
		}
	}
	var patch research.UpdatePatch
	if v, ok := raw["status"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_status", "message": err.Error()})
			return
		}
		patch.Status = &s
	}
	if v, ok := raw["note"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_note", "message": err.Error()})
			return
		}
		patch.Note = &s
	}
	if v, ok := raw["tags"]; ok {
		var tags []string
		if err := json.Unmarshal(v, &tags); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_tags", "message": err.Error()})
			return
		}
		if tags == nil {
			tags = []string{}
		}
		patch.Tags = &tags
	}

	out, err := research.UpdateCandidate(id, patch)
	if err != nil {
		writeResearchErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ResearchCandidatesHandler) handleExplain(w http.ResponseWriter, r *http.Request, candidateID string) {
	switch r.Method {
	case http.MethodGet:
		out, err := research.GetExplain(candidateID)
		if err != nil {
			writeResearchErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPatch, http.MethodPut:
		h.handleExplainUpdate(w, r, candidateID)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *ResearchCandidatesHandler) handleExplainByExplainID(w http.ResponseWriter, r *http.Request, explainID string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	out, err := research.GetExplainByExplainID(explainID)
	if err != nil {
		writeResearchErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ResearchCandidatesHandler) handleExplainUpdate(w http.ResponseWriter, r *http.Request, candidateID string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_body", "message": err.Error()})
		return
	}
	var raw map[string]json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json", "message": err.Error()})
			return
		}
	}
	var patch research.ExplainPatch
	if v, ok := raw["summary"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_summary", "message": err.Error()})
			return
		}
		patch.Summary = &s
	}
	if v, ok := raw["research_reason"]; ok {
		var rr research.ResearchReason
		if err := json.Unmarshal(v, &rr); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_research_reason", "message": err.Error()})
			return
		}
		patch.ResearchReason = &rr
	}
	if v, ok := raw["risk_note"]; ok {
		if string(v) == "null" {
			patch.ClearRiskNote = true
		} else {
			var rn research.RiskNote
			if err := json.Unmarshal(v, &rn); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_risk_note", "message": err.Error()})
				return
			}
			patch.RiskNote = &rn
		}
	}
	if v, ok := raw["clear_risk_note"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil && b {
			patch.ClearRiskNote = true
		}
	}

	out, err := research.UpdateExplain(candidateID, patch)
	if err != nil {
		writeResearchErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeResearchErr(w http.ResponseWriter, err error) {
	var nf research.ErrNotFound
	if errors.As(err, &nf) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found", "message": err.Error()})
		return
	}
	var badDate research.ErrBadTradeDate
	if errors.As(err, &badDate) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid trade_date", "message": err.Error()})
		return
	}
	var badStatus research.ErrBadStatus
	if errors.As(err, &badStatus) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_status", "message": err.Error()})
		return
	}
	var badPatch research.ErrBadPatch
	if errors.As(err, &badPatch) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad_patch", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal", "message": err.Error()})
}

// RegisterResearchCandidatesRoutes mounts research candidate + explain routes.
func RegisterResearchCandidatesRoutes(mux *http.ServeMux) {
	h := NewResearchCandidatesHandler()
	mux.Handle("/api/research/candidates", h)
	mux.Handle("/api/research/candidates/", h)
	mux.Handle("/api/research/explains/", h)
}

// ResearchCandidatesAssetMiddleware mounts /api/research/candidates* and /api/research/explains* .
func ResearchCandidatesAssetMiddleware(next http.Handler) http.Handler {
	h := NewResearchCandidatesHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "/api/research/candidates" ||
			strings.HasPrefix(path, "/api/research/candidates/") ||
			strings.HasPrefix(path, "/api/research/explains/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

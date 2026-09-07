package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/opportunity"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/opportunity/projection"
)

// OpportunitiesHandler serves Phase14-G1.1 opportunity user action APIs.
//   POST /api/opportunities/action
//   GET  /api/opportunities/list
//   GET  /api/opportunities/projections
//   GET  /api/opportunities/outcomes
type OpportunitiesHandler struct {
	projections *projection.Service
	outcomes    *outcome.Service
}

func NewOpportunitiesHandler() *OpportunitiesHandler {
	return &OpportunitiesHandler{
		projections: projection.NewService(),
		outcomes:    outcome.NewService(),
	}
}

func (h *OpportunitiesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/api/opportunities/action":
		h.handlePostAction(w, r)
	case path == "/api/opportunities/list":
		h.handleList(w, r)
	case path == "/api/opportunities/projections":
		h.handleProjections(w, r)
	case path == "/api/opportunities/outcomes":
		h.handleOutcomes(w, r)
	default:
		http.NotFound(w, r)
	}
}

type opportunityActionRequest struct {
	AccountID     uint   `json:"account_id"`
	ScanBatchKey  string `json:"scan_batch_key"`
	OpportunityID string `json:"opportunity_id"`
	StockCode     string `json:"stock_code"`
	Secucode      string `json:"secucode"`
	SignalTime    string `json:"signal_time"`
	SignalTag     string `json:"signal_tag"`
	Action        string `json:"action"`
	CreatedBy     string `json:"created_by"`
}

func (h *OpportunitiesHandler) handlePostAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "POST required",
		})
		return
	}
	var req opportunityActionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": 400, "ok": false, "message": "invalid body: " + err.Error(),
		})
		return
	}

	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     req.AccountID,
		ScanBatchKey:  req.ScanBatchKey,
		OpportunityID: req.OpportunityID,
		StockCode:     req.StockCode,
		Secucode:      req.Secucode,
		SignalTime:    req.SignalTime,
		SignalTag:     req.SignalTag,
		Action:        req.Action,
		CreatedBy:     req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": 400, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":   0,
		"ok":     true,
		"action": row,
	})
}

func (h *OpportunitiesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	q := r.URL.Query()
	includeUser := strings.EqualFold(q.Get("include_user_action"), "true") ||
		q.Get("include_user_action") == "1"

	var accountID uint
	if raw := strings.TrimSpace(q.Get("account_id")); raw != "" {
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			accountID = uint(v)
		}
	}

	view, err := opportunity.ListFromSnapshot(opportunity.ListQuery{
		TradeDate:         strings.TrimSpace(q.Get("trade_date")),
		Session:           strings.TrimSpace(q.Get("session")),
		StrategyID:        strings.TrimSpace(q.Get("strategy_id")),
		AccountID:         accountID,
		IncludeUserAction: includeUser,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"ok":   true,
		"pool": view,
	})
}

// RegisterOpportunitiesRoutes mounts opportunity routes (httptest).
func RegisterOpportunitiesRoutes(mux *http.ServeMux) {
	h := NewOpportunitiesHandler()
	mux.Handle("/api/opportunities/action", h)
	mux.Handle("/api/opportunities/list", h)
	mux.Handle("/api/opportunities/projections", h)
	mux.Handle("/api/opportunities/outcomes", h)
}

// OpportunitiesAssetMiddleware mounts /api/opportunities/* on Wails AssetServer.
func OpportunitiesAssetMiddleware(next http.Handler) http.Handler {
	h := NewOpportunitiesHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/opportunities/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

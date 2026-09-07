package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/papertrading"
)

// ExitReviewHandler serves POST /api/exit-review/outcome.
type ExitReviewHandler struct{}

func NewExitReviewHandler() *ExitReviewHandler { return &ExitReviewHandler{} }

type exitReviewOutcomeRequest struct {
	StockCode                 string   `json:"stock_code"`
	Decision                  string   `json:"decision"`
	Reason                    string   `json:"reason"`
	ReviewTime                string   `json:"review_time"`
	CreatedBy                 string   `json:"created_by"`
	ExitStateSnapshot         string   `json:"exit_state_snapshot"`
	ReasonCodesSnapshot       []string `json:"reason_codes_snapshot"`
	EvaluationSummarySnapshot string   `json:"evaluation_summary_snapshot"`
	RelatedTradePlanID        *uint    `json:"related_trade_plan_id"`
	AccountID                 uint     `json:"account_id"`
}

func (h *ExitReviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/exit-review/outcome" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "POST required",
		})
		return
	}
	var req exitReviewOutcomeRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": 400, "ok": false, "message": "invalid body: " + err.Error(),
		})
		return
	}

	var reviewTime time.Time
	if rt := strings.TrimSpace(req.ReviewTime); rt != "" {
		parsed, err := time.Parse(time.RFC3339, rt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code": 400, "ok": false, "message": "invalid review_time: " + err.Error(),
			})
			return
		}
		reviewTime = parsed
	}

	row, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:                 req.AccountID,
		StockCode:                 req.StockCode,
		Decision:                  req.Decision,
		Reason:                    req.Reason,
		ReviewTime:                reviewTime,
		CreatedBy:                 req.CreatedBy,
		ExitStateSnapshot:         req.ExitStateSnapshot,
		ReasonCodesSnapshot:       req.ReasonCodesSnapshot,
		EvaluationSummarySnapshot: req.EvaluationSummarySnapshot,
		RelatedTradePlanID:        req.RelatedTradePlanID,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": 400, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"ok":      true,
		"outcome": row,
	})
}

// RegisterExitReviewRoutes mounts exit review outcome routes (httptest).
func RegisterExitReviewRoutes(mux *http.ServeMux) {
	h := NewExitReviewHandler()
	mux.Handle("/api/exit-review/outcome", h)
}

// ExitReviewAssetMiddleware mounts /api/exit-review/* on Wails AssetServer.
func ExitReviewAssetMiddleware(next http.Handler) http.Handler {
	h := NewExitReviewHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/exit-review/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

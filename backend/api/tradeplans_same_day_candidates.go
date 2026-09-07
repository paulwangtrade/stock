package api

import (
	"net/http"
	"time"

	"go-stock/backend/data"
)

// SameDayCandidatesResponse GET /api/tradeplans/same-day-candidates envelope.
type SameDayCandidatesResponse struct {
	Code       int                   `json:"code"`
	OK         bool                  `json:"ok"`
	TradeDate  string                `json:"trade_date"`
	Count      int                   `json:"count"`
	Candidates []SameDayCandidateDTO `json:"candidates"`
	Message    string                `json:"message,omitempty"`
}

// SameDayCandidateDTO snake_case wire for plan selector (read-only).
type SameDayCandidateDTO struct {
	ID            uint   `json:"id"`
	TradeDate     string `json:"trade_date"`
	Status        string `json:"status"`
	Source        string `json:"source"`
	SourceSession string `json:"source_session"`
	PlanVersion   int    `json:"plan_version"`
	Side          string `json:"side,omitempty"`
	IsFrozen      bool   `json:"is_frozen"`
	CreatedAt     string `json:"created_at,omitempty"`
}

func (h *TradePlansHandler) handleSameDayCandidates(w http.ResponseWriter, r *http.Request) {
	tradeDate, err := resolveUpcomingTradeDate(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SameDayCandidatesResponse{
			Code: TradePlanCodeBadTradeDate, OK: false, TradeDate: tradeDate, Message: err.Error(),
		})
		return
	}

	repo := data.NewTradePlanRepo()
	if h != nil && h.repo != nil {
		if tr, ok := h.repo.(*data.TradePlanRepo); ok {
			repo = tr
		}
	}

	rows, err := repo.ListSameDayCandidates(tradeDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SameDayCandidatesResponse{
			Code: TradePlanCodeInternalError, OK: false, TradeDate: tradeDate, Message: err.Error(),
		})
		return
	}

	out := make([]SameDayCandidateDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, SameDayCandidateDTO{
			ID:            row.ID,
			TradeDate:     row.TradeDate,
			Status:        row.Status,
			Source:        row.Source,
			SourceSession: row.SourceSession,
			PlanVersion:   row.PlanVersion,
			Side:          row.Side,
			IsFrozen:      row.IsFrozen,
			CreatedAt:     formatCandidateCreatedAt(row.CreatedAt),
		})
	}

	writeJSON(w, http.StatusOK, SameDayCandidatesResponse{
		Code:       TradePlanCodeOK,
		OK:         true,
		TradeDate:  tradeDate,
		Count:      len(out),
		Candidates: out,
	})
}

func formatCandidateCreatedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

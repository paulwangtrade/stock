package data

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// SameDayCandidateView is a read-only summary row for multi-origin plan switching (Phase16.26-B1.2.1).
// Does not load items; does not affect upcoming / execution selection.
type SameDayCandidateView struct {
	ID            uint      `json:"id"`
	TradeDate     string    `json:"tradeDate"`
	Status        string    `json:"status"`
	Source        string    `json:"source"` // product bucket: strategy | watchlist | manual_sell | other
	SourceSession string    `json:"sourceSession"`
	PlanVersion   int       `json:"planVersion"`
	Side          string    `json:"side"`
	IsFrozen      bool      `json:"isFrozen"`
	CreatedAt     time.Time `json:"createdAt"`
}

// ListSameDayCandidates returns non-terminal plans for an exact trade_date (read-only).
//
// Include: draft / ready / executing (and any non-terminal status).
// Exclude: done / partial / skipped / failed / superseded (terminal; no soft-delete column).
// Sort: ready first, then executing, then draft, then plan_version DESC, id DESC.
// Does NOT change GetUpcomingTradePlan selection rules.
func (r *TradePlanRepo) ListSameDayCandidates(tradeDate string) ([]SameDayCandidateView, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return nil, fmt.Errorf("trade_date is required")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return nil, fmt.Errorf("invalid trade_date %q: %w", tradeDate, err)
	}

	terminal := []string{
		models.TradePlanStatusDone,
		models.TradePlanStatusPartial,
		models.TradePlanStatusSkipped,
		models.TradePlanStatusFailed,
		models.TradePlanStatusSuperseded,
	}

	var plans []models.TradePlan
	err := db.Dao.Where("trade_date = ? AND status NOT IN ?", tradeDate, terminal).
		Order(sameDayCandidateOrderSQL()).
		Find(&plans).Error
	if err != nil {
		return nil, err
	}

	out := make([]SameDayCandidateView, 0, len(plans))
	for i := range plans {
		p := plans[i]
		out = append(out, SameDayCandidateView{
			ID:            p.ID,
			TradeDate:     p.TradeDate,
			Status:        p.Status,
			Source:        mapTradePlanSourceBucket(p.SourceSession),
			SourceSession: p.SourceSession,
			PlanVersion:   p.PlanVersion,
			Side:          p.Side,
			IsFrozen:      p.IsFrozen(),
			CreatedAt:     p.CreatedAt,
		})
	}
	return out, nil
}

func sameDayCandidateOrderSQL() string {
	// Ready (incl. frozen ready) first for selector UX; does not alter upcoming First() rules.
	return `CASE status
		WHEN 'ready' THEN 0
		WHEN 'executing' THEN 1
		WHEN 'draft' THEN 2
		ELSE 3
	END ASC, plan_version DESC, id DESC`
}

// mapTradePlanSourceBucket maps source_session to a stable product bucket for UI.
func mapTradePlanSourceBucket(sourceSession string) string {
	switch strings.TrimSpace(sourceSession) {
	case models.TradePlanSourceAfterClose,
		models.TradePlanSourceMorningRebuild,
		models.TradePlanSourceCashRescale:
		return "strategy"
	case models.TradePlanSourceWatchlist:
		return "watchlist"
	case models.TradePlanSourceTSell,
		models.TradePlanSourceExitReview:
		return "manual_sell"
	case "":
		return "other"
	default:
		return "other"
	}
}

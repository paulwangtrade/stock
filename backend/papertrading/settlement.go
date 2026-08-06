package papertrading

import (
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// SettlementResult summarizes the EOD mark-to-market job.
// IMPORTANT: This job does NOT unlock same-day buys (T+1). Unlock is SettleNewTradingDay on a later day.
type SettlementResult struct {
	Enabled           bool    `json:"enabled"`
	TradeDate         string  `json:"tradeDate"`
	AccountID         uint    `json:"accountId"`
	PositionsUpdated  int     `json:"positionsUpdated"`
	StaleMarks        int     `json:"staleMarks"`
	MarketValue       float64 `json:"marketValue"`
	UnrealizedPnl     float64 `json:"unrealizedPnl"`
	Equity            float64 `json:"equity"`
	LockedVolumeTotal int64   `json:"lockedVolumeTotal"` // still locked — T+1 prep only
	Message           string  `json:"message"`
}

// MarkPricer yields a mark price for settlement (latest/close). Optional; falls back to OpenQuote.
type MarkPricer interface {
	MarkPrice(symbol string) (float64, bool)
}

// SettlementJob performs end-of-day mark-to-market for the default paper-sim account.
// It updates market_value / unrealized_pnl / equity and logs T+1 prep state.
// It does NOT call SettleNewTradingDay (would unlock today's buys prematurely).
//
// weekdayCheck: when true (production), skip on weekends; tests may pass false.
func SettlementJob(tradeDate string, price PriceProvider, weekdayCheck bool) (*SettlementResult, error) {
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}
	out := &SettlementResult{Enabled: IsEnabled(), TradeDate: tradeDate}
	if !IsEnabled() {
		out.Message = "enablePaperTrading=false"
		logger.SugaredLogger.Infof("PaperSettlementJob skipped disabled trade_date=%s", tradeDate)
		return out, nil
	}
	if weekdayCheck && !data.IsWeekdayLocal(time.Now()) {
		out.Message = "non trading day"
		logger.SugaredLogger.Infof("PaperSettlementJob skipped non_trading_day trade_date=%s", tradeDate)
		return out, nil
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}
	if price == nil {
		price = DefaultOpenPriceProvider()
	}

	acc, err := GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	if acc == nil {
		out.Message = "no paper_sim account yet"
		return out, nil
	}
	out.AccountID = acc.ID

	positions, err := GetPositions(acc.ID)
	if err != nil {
		return nil, err
	}

	marker, _ := price.(MarkPricer)
	now := time.Now()
	for i := range positions {
		p := &positions[i]
		mark, ok := 0.0, false
		if marker != nil {
			mark, ok = marker.MarkPrice(p.StockCode)
		}
		if !ok {
			if q, qok := price.OpenQuote(p.StockCode, tradeDate); qok && q.Open > 0 {
				mark, ok = q.Open, true
			}
		}
		if !ok || mark <= 0 {
			out.StaleMarks++
			out.LockedVolumeTotal += p.LockedVolume
			continue
		}
		p.MarkPrice = mark
		p.UpdatedAt = now
		if err := db.Dao.Model(&PaperSimPosition{}).Where("id = ?", p.ID).
			Updates(map[string]any{"mark_price": mark, "updated_at": now}).Error; err != nil {
			return nil, err
		}
		out.PositionsUpdated++
		out.LockedVolumeTotal += p.LockedVolume
	}

	// Revalue account from updated marks (do not unlock).
	broker := NewPaperBroker(price)
	if err := broker.revalueAccount(acc.ID); err != nil {
		return nil, err
	}
	acc2, _ := GetDefaultAccount()
	if acc2 != nil {
		out.MarketValue = acc2.MarketValue
		out.UnrealizedPnl = acc2.UnrealizedPnl
		out.Equity = acc2.Equity
	}
	out.Message = fmt.Sprintf(
		"settlement mark-to-market done; t1_unlock_pending locked_volume=%d (NOT unlocked today)",
		out.LockedVolumeTotal,
	)
	logger.SugaredLogger.Infof(
		"PaperSettlementJob done trade_date=%s account_id=%d positions=%d stale=%d locked=%d equity=%.2f unrealized=%.2f",
		tradeDate, out.AccountID, out.PositionsUpdated, out.StaleMarks, out.LockedVolumeTotal, out.Equity, out.UnrealizedPnl,
	)

	// Freeze immutable daily report after successful settlement (idempotent).
	rpt, rerr := GenerateDailyReport(tradeDate)
	if rerr != nil {
		logger.SugaredLogger.Errorf("PaperDailyReport after settlement failed trade_date=%s err=%v", tradeDate, rerr)
		// Settlement already succeeded; do not fail the settle job.
	} else if rpt != nil {
		logger.SugaredLogger.Infof("PaperDailyReport after settlement generated=%v skipped=%v message=%s",
			rpt.Generated, rpt.Skipped, rpt.Message)
	}
	return out, nil
}

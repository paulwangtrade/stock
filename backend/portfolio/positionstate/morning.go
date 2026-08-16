package positionstate

import (
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/papertrading"
	"go-stock/backend/tradingcalendar"
)

// MorningSettlementResult is the 09:20 PositionState settlement observation.
// Unlock writes go through existing PositionUnlockJob (no Gateway / fill changes).
type MorningSettlementResult struct {
	TradeDate          string             `json:"trade_date"`
	PreviousTradeDay   string             `json:"previous_trade_day,omitempty"`
	UnlockSkipped      bool               `json:"unlock_skipped"`
	UnlockMessage      string             `json:"unlock_message,omitempty"`
	PositionsUnlocked  int                `json:"positions_unlocked"`
	UnlockVolumeTotal  int64              `json:"unlock_volume_total"`
	PositionStates     []PositionStateView `json:"position_states"`
	DataSourceNote     string             `json:"data_source_note"`
}

// RunMorningSettlement executes T+1 unlock via existing paper job, then recomputes PositionState.
// Intended cron: 09:20 trading days. Does not alter TradingEvent emit or order fills.
func RunMorningSettlement(now time.Time) (*MorningSettlementResult, error) {
	if now.IsZero() {
		now = time.Now()
	}
	td := tradingcalendar.FormatDate(now)
	prev := previousTradingDay(now)
	out := &MorningSettlementResult{
		TradeDate:        td,
		PreviousTradeDay: prev,
		DataSourceNote:   "PositionState Morning Settlement · delegates unlock to PositionUnlockJob; then recalculate states",
	}

	unlock, err := papertrading.PositionUnlockJob(now)
	if err != nil {
		return out, err
	}
	if unlock != nil {
		out.UnlockSkipped = unlock.Skipped
		out.UnlockMessage = unlock.Message
		out.PositionsUnlocked = unlock.PositionsUnlocked
		out.UnlockVolumeTotal = unlock.UnlockVolumeTotal
	}

	bundle := NewService(nil).Evaluate(Query{TradeDate: td, AsOf: now})
	if bundle != nil {
		out.PositionStates = bundle.Positions
	}
	logger.SugaredLogger.Infof(
		"PositionStateMorningSettlement trade_date=%s prev=%s unlocked=%d volume=%d states=%d skipped=%v",
		td, prev, out.PositionsUnlocked, out.UnlockVolumeTotal, len(out.PositionStates), out.UnlockSkipped,
	)
	return out, nil
}

func previousTradingDay(now time.Time) string {
	// Walk back up to 10 calendar days for prior trading day (K-Beta).
	d := now.AddDate(0, 0, -1)
	for i := 0; i < 10; i++ {
		if tradingcalendar.IsTradingDay(d) {
			return tradingcalendar.FormatDate(d)
		}
		d = d.AddDate(0, 0, -1)
	}
	return strings.TrimSpace(tradingcalendar.FormatDate(now.AddDate(0, 0, -1)))
}

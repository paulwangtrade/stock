package intelligence

import (
	"math"
	"strings"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/tradingcalendar"
)

const (
	dataSourceNote = "Position Intelligence · Snapshot + PositionState (K-Beta) + optional Quote; read-only; no trade actions"
	disclaimer     = "持仓观察标签，不是交易建议。不生成买卖指令，不修改 Gateway / TradePlan / PaperTrading。"

	pnlEpsilon = 1e-6
	// Align with holding evaluation spirit: watch ~-5%, high/danger zone ~-10%.
	riskWatchReturn = -0.05
	riskHighReturn  = -0.10
)

// Options configures a read-only build.
type Options struct {
	AsOf      time.Time
	TradeDate string
	// Snapshot required for holdings; nil → empty Found=false.
	Snapshot *portfolio.Snapshot
	// QuoteService optional; nil or errors → use mark_price.
	Quotes marketdata.QuoteService
	// StrategyHints optional map stock_code → strategy_status.
	StrategyHints map[string]string
	// ExtraCodes: emit NO_POSITION rows for codes not in snapshot (tests / watchlist).
	ExtraCodes []string
	// BuyLotsByCode / SellLotsByCode feed PositionState (first_buy_date). Key: normalized code.
	BuyLotsByCode  map[string][]positionstate.LotRecord
	SellLotsByCode map[string][]positionstate.LotRecord
}

// Build projects PositionIntelligenceView rows (pure aside from optional quote fetch).
// Never writes DB; quote failures are swallowed.
func Build(opts Options) *Bundle {
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	out := &Bundle{
		AsOf:           asOf,
		Found:          false,
		Positions:      []PositionIntelligenceView{},
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}
	if opts.Snapshot == nil || !opts.Snapshot.Found {
		for _, code := range opts.ExtraCodes {
			out.Positions = append(out.Positions, noPositionRow(code, opts.StrategyHints))
		}
		return out
	}
	out.Found = true

	quoteMap := fetchQuotesSafe(opts.Quotes, collectCodes(opts.Snapshot, opts.ExtraCodes))

	seen := map[string]struct{}{}
	for _, p := range opts.Snapshot.Positions {
		code := strings.TrimSpace(p.StockCode)
		if code == "" || p.Volume <= 0 {
			continue
		}
		seen[code] = struct{}{}
		mark := p.MarkPrice
		src := "persisted"
		if q, ok := quoteMap[code]; ok {
			if q.Price > 0 {
				mark = q.Price
				src = "live"
			} else if q.Open > 0 {
				mark = q.Open
				src = "open_fallback"
			}
		}
		pnl := (mark - p.AvgCost) * float64(p.Volume)
		var ret *float64
		if p.AvgCost > 0 {
			r := (mark - p.AvgCost) / p.AvgCost
			ret = &r
		}
		risk := classifyRisk(ret)
		strat := strategyStatus(code, opts.StrategyHints)
		locked := p.LockedVolume
		ps := positionstate.Calculate(positionstate.SnapshotInput{
			Symbol:       code,
			TotalQty:     p.Volume,
			AvailableQty: p.AvailableVolume,
			LockedQty:    &locked,
			BuyRecords:   lotsFor(opts.BuyLotsByCode, code),
			SellRecords:  lotsFor(opts.SellLotsByCode, code),
			TradeDate:    tradeDateOf(asOf, opts.TradeDate),
			CurrentDate:  tradeDateOf(asOf, opts.TradeDate),
		})
		status := classifyPositionStatusFromState(ps, pnl, risk)
		out.Positions = append(out.Positions, PositionIntelligenceView{
			StockCode:       code,
			StockName:       strings.TrimSpace(p.StockName),
			PositionStatus:  status,
			CurrentPosition: p.Volume,
			Cost:            p.AvgCost,
			MarketPrice:     mark,
			PnL:             pnl,
			RiskLevel:       risk,
			StrategyStatus:  strat,
			AttentionReason: attentionReason(status, strat, risk, pnl),
			QuoteSource:     src,
			AvailableVolume: p.AvailableVolume,
			LockedVolume:    p.LockedVolume,
			PositionState:   ps.State,
			IsNewPosition:   ps.IsNewPosition,
			CanSell:         ps.CanSell,
			HoldingDays:     ps.HoldingDays,
		})
	}

	for _, code := range opts.ExtraCodes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		out.Positions = append(out.Positions, noPositionRow(code, opts.StrategyHints))
	}
	return out
}

func noPositionRow(code string, hints map[string]string) PositionIntelligenceView {
	code = strings.TrimSpace(code)
	strat := strategyStatus(code, hints)
	return PositionIntelligenceView{
		StockCode:       code,
		PositionStatus:  StatusNoPosition,
		CurrentPosition: 0,
		RiskLevel:       RiskUnknown,
		StrategyStatus:  strat,
		AttentionReason: attentionReason(StatusNoPosition, strat, RiskUnknown, 0),
	}
}

func collectCodes(snap *portfolio.Snapshot, extra []string) []string {
	set := map[string]struct{}{}
	if snap != nil {
		for _, p := range snap.Positions {
			c := strings.TrimSpace(p.StockCode)
			if c != "" {
				set[c] = struct{}{}
			}
		}
	}
	for _, c := range extra {
		c = strings.TrimSpace(c)
		if c != "" {
			set[c] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func fetchQuotesSafe(svc marketdata.QuoteService, codes []string) map[string]marketdata.Quote {
	out := map[string]marketdata.Quote{}
	if svc == nil || len(codes) == 0 {
		return out
	}
	defer func() { _ = recover() }()
	quotes, err := svc.GetQuotes(codes)
	if err != nil || len(quotes) == 0 {
		return out
	}
	for i := range quotes {
		q := quotes[i]
		c := strings.TrimSpace(q.Code)
		if c == "" {
			continue
		}
		out[c] = q
	}
	return out
}

func strategyStatus(code string, hints map[string]string) string {
	if hints == nil {
		return StrategyUnknown
	}
	if s, ok := hints[code]; ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return StrategyUnknown
}

func classifyRisk(ret *float64) string {
	if ret == nil || math.IsNaN(*ret) {
		return RiskUnknown
	}
	r := *ret
	if r <= riskHighReturn {
		return RiskHigh
	}
	if r <= riskWatchReturn {
		return RiskMedium
	}
	return RiskLow
}

func classifyPositionStatusFromState(ps positionstate.PositionStateView, pnl float64, risk string) string {
	if ps.TotalQty <= 0 || ps.State == positionstate.S0NoPosition {
		return StatusNoPosition
	}
	if risk == RiskHigh {
		return StatusNeedReview
	}
	// NEW_POSITION only when PositionState says is_new_position (first_buy_date == trade_date).
	// Never from available_qty == 0 alone.
	if ps.IsNewPosition {
		return StatusNewPosition
	}
	if pnl > pnlEpsilon {
		return StatusHoldingProfit
	}
	if pnl < -pnlEpsilon {
		if risk == RiskMedium {
			return StatusNeedReview
		}
		return StatusHoldingLoss
	}
	return StatusWaiting
}

func lotsFor(m map[string][]positionstate.LotRecord, code string) []positionstate.LotRecord {
	if m == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(code))
	if v, ok := m[key]; ok {
		return v
	}
	return m[strings.TrimSpace(code)]
}

func tradeDateOf(asOf time.Time, tradeDate string) string {
	if strings.TrimSpace(tradeDate) != "" {
		return strings.TrimSpace(tradeDate)
	}
	return tradingcalendar.FormatDate(asOf)
}

func attentionReason(status, strategy, risk string, pnl float64) string {
	switch status {
	case StatusNoPosition:
		if strategy == StrategyUnknown {
			return "无持仓；缺少策略解释"
		}
		if strategy == StrategyActiveSignal {
			return "无持仓，但存在策略信号（观察用，非下单建议）"
		}
		return "无持仓"
	case StatusNewPosition:
		return "新建仓（当日锁定），请关注次日可用与计划是否加仓场景"
	case StatusNeedReview:
		if pnl < 0 {
			return "当前持仓亏损，风险升高，建议复盘（未触发自动退出）"
		}
		return "风险标签升高，建议复盘"
	case StatusHoldingLoss:
		if strategy == StrategyUnknown {
			return "当前持仓亏损，但缺少策略解释；未触发策略退出条件（观察）"
		}
		return "当前持仓亏损，但未触发策略退出条件"
	case StatusHoldingProfit:
		if strategy == StrategyUnknown {
			return "持仓盈利；缺少策略解释"
		}
		return "持仓盈利，维持观察"
	case StatusWaiting:
		return "持仓接近成本，等待进一步信号"
	default:
		if strategy == StrategyUnknown {
			return "缺少策略解释"
		}
		return ""
	}
}

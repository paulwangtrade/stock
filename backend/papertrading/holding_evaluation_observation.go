// Holding Evaluation Observation DTO — stock-level aggregation for read-only API.
package papertrading

import (
	"sort"
	"strings"
	"time"
)

// HoldingEvalObservationView is the Observation API payload (stock rows + lots).
type HoldingEvalObservationView struct {
	Enabled             bool                  `json:"enabled"`
	AccountID           uint                  `json:"account_id,omitempty"`
	AsOf                time.Time             `json:"as_of"`
	ReconcileAllMatched bool                  `json:"reconcile_all_matched"`
	Holdings            []HoldingEvalStockRow `json:"holdings"`
	Unattributable      []UnattributedHolding `json:"unattributable,omitempty"`
	Summary             HoldingEvalSummary    `json:"summary"`
	DataSourceNote      string                `json:"data_source_note"`
	PriceProviderNote   string                `json:"price_provider_note,omitempty"`
}

// HoldingEvalStockRow aggregates evaluated lots for one stock_code.
type HoldingEvalStockRow struct {
	StockCode          string              `json:"stock_code"`
	StockName          string              `json:"stock_name"`
	TotalVolume        int64               `json:"total_volume"`
	AvgCost            *float64            `json:"avg_cost"`
	MarketPrice        *float64            `json:"market_price"`
	CurrentPrice       *float64            `json:"current_price"`
	QuoteSource        string              `json:"quote_source,omitempty"`
	QuoteTime          *time.Time          `json:"quote_time,omitempty"`
	MarketValue        *float64            `json:"market_value"`
	UnrealizedPnL      *float64            `json:"unrealized_pnl"`
	UnrealizedReturn   *float64            `json:"unrealized_return"`    // ratio (not percent)
	HoldingDays        int                 `json:"holding_days"`         // as_of − earliest lot buy_date
	TrendState         string              `json:"trend_state"`          // D.1.4: always UNKNOWN
	RiskState          string              `json:"risk_state"`           // NORMAL|WATCH|DANGER (label only)
	ProfitState        string              `json:"profit_state"`         // PROFIT|BREAKEVEN|LOSS|UNKNOWN
	HoldingPeriodState string              `json:"holding_period_state"` // SHORT_TERM|MID_TERM|LONG_TERM
	EvalState          string              `json:"eval_state"`
	Lots               []HoldingEvalLotRow `json:"lots"`
	ReconcileStatus    string              `json:"reconcile_status,omitempty"`
	FirstBuyDate       string                         `json:"first_buy_date,omitempty"`
	DisplayName        SecurityDisplayName            `json:"display_name"`
	// Explanation is Phase17-C2 read-only explainability (optional; no trade actions).
	Explanation *PositionEvaluationExplanation `json:"explanation,omitempty"`
	// HealthScore is Phase17-C3 position quality score (optional; not a sell signal).
	HealthScore *HoldingHealthScore `json:"health_score,omitempty"`
	// TSuitability is Phase17.1 做 T suitability (optional; not a trade signal).
	TSuitability *HoldingTSuitability `json:"t_suitability,omitempty"`
}

// HoldingEvalLotRow is one attributed fill lot for Observation expand UI.
type HoldingEvalLotRow struct {
	FillID             uint       `json:"fill_id"`
	PlanID             uint       `json:"plan_id"`
	PlanItemID         uint       `json:"plan_item_id"`
	BuyDate            string     `json:"buy_date,omitempty"`
	Volume             int64      `json:"volume"`
	CostPrice          float64    `json:"cost_price"`
	CurrentPrice       *float64   `json:"current_price"`
	QuoteSource        string     `json:"quote_source,omitempty"`
	QuoteTime          *time.Time `json:"quote_time,omitempty"`
	PnL                *float64   `json:"pnl"`
	ReturnRate         *float64   `json:"return_rate"` // ratio
	HoldingDays        int        `json:"holding_days"`
	EvalState          string     `json:"eval_state"`
	ProfitState        string     `json:"profit_state"`
	HoldingPeriodState string     `json:"holding_period_state"`
}

// BuildHoldingEvaluationObservation builds the Observation API view.
// Pipeline: Attribution → ProjectHoldingEvaluation → stock aggregation.
// Does not reimplement PnL math; does not write DB / Broker / Gateway.
func BuildHoldingEvaluationObservation(opts HoldingEvaluationBuildOptions) (*HoldingEvalObservationView, error) {
	attr, err := BuildPositionAttribution(AttributionOptions{StockCode: opts.StockCode})
	if err != nil {
		empty := AggregateHoldingEvaluationObservation(nil, nil)
		empty.Enabled = IsEnabled()
		return empty, err
	}
	provider := NewQuoteOverlayPriceProvider(attr, holdingEvalQuoteFetcher())
	flat := ProjectHoldingEvaluation(attr, provider, HoldingEvaluationOptions{
		AsOf:      opts.AsOf,
		StockCode: opts.StockCode,
	})
	flat.PriceProviderNote = "quote_overlay_tencent_fallback_position_mark"
	enrichHoldingNamesFromItems(flat)
	out := AggregateHoldingEvaluationObservation(flat, attr)
	return out, nil
}

// NewAttributionMarkPriceProviderStrict omits marks with non-positive price (→ missing / null metrics).
func NewAttributionMarkPriceProviderStrict(attr *PositionAttributionView) *AttributionMarkPriceProvider {
	p := &AttributionMarkPriceProvider{byCode: map[string]MarkQuote{}}
	if attr == nil {
		return p
	}
	asOf := attr.AsOf
	for _, row := range attr.Positions {
		code := normalizeAttrCode(row.StockCode)
		if code == "" || row.CurrentPrice <= 0 {
			continue
		}
		p.byCode[code] = MarkQuote{
			Price:   row.CurrentPrice,
			AsOf:    asOf,
			Source:  "position_mark",
			Quality: "ok",
		}
	}
	return p
}

// AggregateHoldingEvaluationObservation groups flat lot evaluations into stock rows.
func AggregateHoldingEvaluationObservation(flat *HoldingEvaluationView, attr *PositionAttributionView) *HoldingEvalObservationView {
	out := &HoldingEvalObservationView{
		Enabled:        true,
		Holdings:       []HoldingEvalStockRow{},
		Unattributable: []UnattributedHolding{},
		DataSourceNote: holdingEvaluationDataSourceNote,
		Summary: HoldingEvalSummary{
			ByEvalState: map[string]int{
				EvalStateNormal:        0,
				EvalStateWatch:         0,
				EvalStateExitCandidate: 0,
			},
		},
	}
	if flat == nil {
		out.ReconcileAllMatched = true
		return out
	}
	out.Enabled = flat.Enabled
	out.AccountID = flat.AccountID
	out.AsOf = flat.AsOf
	out.ReconcileAllMatched = flat.ReconcileAllMatched
	out.Unattributable = flat.Unattributable
	out.Summary = flat.Summary
	if out.Summary.ByEvalState == nil {
		out.Summary.ByEvalState = map[string]int{}
	}
	out.DataSourceNote = flat.DataSourceNote
	out.PriceProviderNote = flat.PriceProviderNote

	recByCode := map[string]string{}
	nameByCode := map[string]string{}
	if attr != nil {
		for _, row := range attr.Positions {
			code := normalizeAttrCode(row.StockCode)
			recByCode[code] = row.Reconcile.Status
			if strings.TrimSpace(row.StockName) != "" {
				nameByCode[code] = row.StockName
			}
		}
	}

	type acc struct {
		row          HoldingEvalStockRow
		costBasis    float64
		vol          int64
		sumMV        float64
		sumPnL       float64
		haveAllPrice bool
		marketPrice  *float64
		firstBuyDate string
	}
	byCode := map[string]*acc{}
	order := []string{}

	for _, lot := range flat.Holdings {
		code := normalizeAttrCode(lot.StockCode)
		if code == "" {
			continue
		}
		a, ok := byCode[code]
		if !ok {
			name := lot.StockNameSnapshot
			if n, ok := nameByCode[code]; ok && n != "" {
				name = n
			}
			a = &acc{
				row: HoldingEvalStockRow{
					StockCode:       lot.StockCode,
					StockName:       name,
					EvalState:       EvalStateNormal,
					TrendState:      TrendStateUnknown, // no MA / no market fetch
					RiskState:       RiskStateNormal,
					Lots:            []HoldingEvalLotRow{},
					ReconcileStatus: recByCode[code],
				},
				haveAllPrice: true,
			}
			byCode[code] = a
			order = append(order, code)
		}
		ret := pctToRatio(lot.UnrealizedReturnPct)
		a.row.Lots = append(a.row.Lots, HoldingEvalLotRow{
			FillID:             lot.LotID,
			PlanID:             lot.PlanID,
			PlanItemID:         lot.PlanItemID,
			BuyDate:            lot.BuyDate,
			Volume:             lot.Volume,
			CostPrice:          lot.CostPrice,
			CurrentPrice:       lot.CurrentPrice,
			QuoteSource:        lot.PriceSource,
			QuoteTime:          lot.QuoteTime,
			PnL:                lot.UnrealizedPnL,
			ReturnRate:         ret,
			HoldingDays:        lot.HoldingDays,
			EvalState:          lot.EvalState,
			ProfitState:        ClassifyProfitState(ret),
			HoldingPeriodState: ClassifyHoldingPeriodState(lot.HoldingDays, DefaultHoldingPeriodThresholds),
		})
		if lot.PriceSource == QuoteSourceTencent {
			a.row.QuoteSource = QuoteSourceTencent
			a.row.QuoteTime = lot.QuoteTime
		} else if a.row.QuoteSource == "" && lot.PriceSource != "" {
			a.row.QuoteSource = lot.PriceSource
			if a.row.QuoteTime == nil {
				a.row.QuoteTime = lot.QuoteTime
			}
		}
		a.vol += lot.Volume
		a.costBasis += lot.CostPrice * float64(lot.Volume)
		a.row.EvalState = worseEvalState(a.row.EvalState, lot.EvalState)
		bd := strings.TrimSpace(lot.BuyDate)
		if bd != "" && (a.firstBuyDate == "" || bd < a.firstBuyDate) {
			a.firstBuyDate = bd
		}
		if lot.CurrentPrice == nil || lot.MarketValue == nil || lot.UnrealizedPnL == nil {
			a.haveAllPrice = false
		} else {
			a.sumMV += *lot.MarketValue
			a.sumPnL += *lot.UnrealizedPnL
			if a.marketPrice == nil {
				px := *lot.CurrentPrice
				a.marketPrice = &px
			}
		}
	}

	holdings := make([]HoldingEvalStockRow, 0, len(order))
	for _, code := range order {
		a := byCode[code]
		a.row.TotalVolume = a.vol
		a.row.TrendState = TrendStateUnknown
		a.row.FirstBuyDate = a.firstBuyDate
		a.row.HoldingDays = calendarHoldingDays(a.firstBuyDate, flat.AsOf)
		if a.vol > 0 && a.costBasis > 0 {
			avg := a.costBasis / float64(a.vol)
			a.row.AvgCost = &avg
		}
		if a.haveAllPrice && len(a.row.Lots) > 0 {
			mv := a.sumMV
			pnl := a.sumPnL
			a.row.MarketValue = &mv
			a.row.UnrealizedPnL = &pnl
			a.row.MarketPrice = a.marketPrice
			a.row.CurrentPrice = a.marketPrice
			if a.costBasis > 0 {
				r := a.sumPnL / a.costBasis
				a.row.UnrealizedReturn = &r
			}
		}
		a.row.RiskState = ClassifyRiskState(a.row.UnrealizedReturn, DefaultRiskStateThresholds)
		a.row.ProfitState = ClassifyProfitState(a.row.UnrealizedReturn)
		a.row.HoldingPeriodState = ClassifyHoldingPeriodState(a.row.HoldingDays, DefaultHoldingPeriodThresholds)
		if a.row.QuoteSource == "" {
			a.row.QuoteSource = QuoteSourcePositionMark
		}
		a.row.DisplayName = ProjectSecurityDisplayName(a.row.StockName, "")
		holdings = append(holdings, a.row)
	}
	sort.SliceStable(holdings, func(i, j int) bool {
		return holdings[i].StockCode < holdings[j].StockCode
	})
	out.Holdings = holdings
	return out
}

func pctToRatio(pct *float64) *float64 {
	if pct == nil {
		return nil
	}
	r := *pct / 100
	return &r
}

func worseEvalState(a, b string) string {
	rank := func(s string) int {
		switch s {
		case EvalStateExitCandidate:
			return 3
		case EvalStateWatch:
			return 2
		case EvalStateNormal:
			return 1
		default:
			return 0
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	if a == "" {
		return EvalStateNormal
	}
	return a
}

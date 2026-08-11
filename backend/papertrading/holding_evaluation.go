// Holding Evaluation — read-only projection (Phase10-D.1.1).
//
// Builds lot-level performance metrics from Position Attribution + Market Price Provider.
// Does not alter positions/fills schema, Broker, Gateway, or Execution write paths.
// eval_state is NORMAL for all lots in this slice (WATCH / EXIT_CANDIDATE reserved).

package papertrading

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
)

// EvalState values for Holding Evaluation (not Exit Signal).
// D.1.4 keeps eval_state as a separate legacy label; risk uses RiskState*.
const (
	EvalStateNormal        = "NORMAL"
	EvalStateWatch         = "WATCH"          // reserved; unused in D.1.4 risk path
	EvalStateExitCandidate = "EXIT_CANDIDATE" // reserved; not an exit action
)

// RiskState is a read-only evaluation label from unrealized return (not sell / Exit Signal).
const (
	RiskStateNormal = "NORMAL"
	RiskStateWatch  = "WATCH"
	RiskStateDanger = "DANGER"
)

// TrendState — D.1.4 does not compute trend (no market fetch / MA20); always UNKNOWN.
const (
	TrendStateUnknown = "UNKNOWN"
	TrendStateUp      = "UP"      // reserved
	TrendStateSideway = "SIDEWAY" // reserved
	TrendStateDown    = "DOWN"    // reserved
)

// RiskStateThresholds classifies unrealized_return (ratio, not percent).
// Not wired into Broker / Gateway / Execution.
type RiskStateThresholds struct {
	// WatchBelow: returns strictly below this enter WATCH (default -0.05).
	WatchBelow float64
	// DangerBelow: returns strictly below this enter DANGER (default -0.10).
	DangerBelow float64
}

// DefaultRiskStateThresholds is the D.1.4 MVP policy (configurable; not trade-path globals).
var DefaultRiskStateThresholds = RiskStateThresholds{
	WatchBelow:  -0.05,
	DangerBelow: -0.10,
}

// ClassifyRiskState maps unrealized return ratio → risk_state.
// nil / missing return → NORMAL (no crash, no invented danger).
func ClassifyRiskState(unrealizedReturnRatio *float64, th RiskStateThresholds) string {
	if unrealizedReturnRatio == nil || math.IsNaN(*unrealizedReturnRatio) || math.IsInf(*unrealizedReturnRatio, 0) {
		return RiskStateNormal
	}
	r := *unrealizedReturnRatio
	if th.WatchBelow == 0 && th.DangerBelow == 0 {
		th = DefaultRiskStateThresholds
	}
	// return >= -5% → NORMAL; -5% > return >= -10% → WATCH; return < -10% → DANGER
	if r >= th.WatchBelow {
		return RiskStateNormal
	}
	if r >= th.DangerBelow {
		return RiskStateWatch
	}
	return RiskStateDanger
}

const holdingEvaluationDataSourceNote = "Holding Evaluation · read-only projection from attribution lots + mark prices; no sell side effects"

// MarkQuote is one mark from MarketPriceProvider.
type MarkQuote struct {
	Price   float64
	AsOf    time.Time
	Source  string
	Quality string // "ok" | "missing"
}

// MarketPriceProvider supplies read-only marks. Must not write positions/fills.
type MarketPriceProvider interface {
	MarkPrice(stockCode string, asOf time.Time) (MarkQuote, bool)
}

// MapPriceProvider is a static code→price map for tests and simple adapters.
type MapPriceProvider map[string]float64

// MarkPrice implements MarketPriceProvider.
func (m MapPriceProvider) MarkPrice(stockCode string, asOf time.Time) (MarkQuote, bool) {
	code := normalizeAttrCode(stockCode)
	if m == nil {
		return MarkQuote{Quality: "missing", AsOf: asOf, Source: "map"}, false
	}
	for k, p := range m {
		if normalizeAttrCode(k) == code {
			return MarkQuote{Price: p, AsOf: asOf, Source: "map", Quality: "ok"}, true
		}
	}
	return MarkQuote{Quality: "missing", AsOf: asOf, Source: "map"}, false
}

// AttributionMarkPriceProvider uses CurrentPrice from attribution rows.
type AttributionMarkPriceProvider struct {
	byCode map[string]MarkQuote
}

// NewAttributionMarkPriceProvider builds a provider from attribution position marks.
func NewAttributionMarkPriceProvider(attr *PositionAttributionView) *AttributionMarkPriceProvider {
	p := &AttributionMarkPriceProvider{byCode: map[string]MarkQuote{}}
	if attr == nil {
		return p
	}
	asOf := attr.AsOf
	for _, row := range attr.Positions {
		code := normalizeAttrCode(row.StockCode)
		if code == "" {
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

// MarkPrice implements MarketPriceProvider.
func (p *AttributionMarkPriceProvider) MarkPrice(stockCode string, asOf time.Time) (MarkQuote, bool) {
	if p == nil || p.byCode == nil {
		return MarkQuote{Quality: "missing", AsOf: asOf, Source: "position_mark"}, false
	}
	q, ok := p.byCode[normalizeAttrCode(stockCode)]
	if !ok {
		return MarkQuote{Quality: "missing", AsOf: asOf, Source: "position_mark"}, false
	}
	if asOf.IsZero() {
		return q, true
	}
	q.AsOf = asOf
	return q, true
}

// HoldingEvaluationOptions configures BuildHoldingEvaluation / ProjectHoldingEvaluation.
type HoldingEvaluationOptions struct {
	AsOf time.Time
	// StockCode optional filter (normalized match).
	StockCode string
	// NameByFillID optional stock_name_snapshot overrides (plan/item snapshot).
	NameByFillID map[uint]string
}

// HoldingEvaluationView is the account-level evaluation response.
type HoldingEvaluationView struct {
	Enabled             bool                   `json:"enabled"`
	AccountID           uint                   `json:"account_id,omitempty"`
	AsOf                time.Time              `json:"as_of"`
	ReconcileAllMatched bool                   `json:"reconcile_all_matched"`
	Holdings            []HoldingLotEvaluation `json:"holdings"`
	Unattributable      []UnattributedHolding  `json:"unattributable,omitempty"`
	Summary             HoldingEvalSummary     `json:"summary"`
	DataSourceNote      string                 `json:"data_source_note"`
	PriceProviderNote   string                 `json:"price_provider_note,omitempty"`
}

// HoldingLotEvaluation is one attributed buy lot with mark metrics.
type HoldingLotEvaluation struct {
	LotID               uint       `json:"lot_id"` // = fill_id
	PlanID              uint       `json:"plan_id"`
	PlanItemID          uint       `json:"plan_item_id"`
	StockCode           string     `json:"stock_code"`
	StockNameSnapshot   string     `json:"stock_name_snapshot"`
	Volume              int64      `json:"volume"`
	CostPrice           float64    `json:"cost_price"`
	BuyDate             string     `json:"buy_date,omitempty"`
	HoldingDays         int        `json:"holding_days"`
	CurrentPrice        *float64   `json:"current_price"`
	MarketValue         *float64   `json:"market_value"`
	UnrealizedPnL       *float64   `json:"unrealized_pnl"`
	UnrealizedReturnPct *float64   `json:"unrealized_return_pct"`
	EvalState           string     `json:"eval_state"`
	PriceSource         string     `json:"price_source,omitempty"`
	PriceQuality        string     `json:"price_quality,omitempty"`
	QuoteTime           *time.Time `json:"quote_time,omitempty"`
}

// UnattributedHolding mirrors attribution qty not covered by fills (not a lot).
type UnattributedHolding struct {
	StockCode  string `json:"stock_code"`
	StockName  string `json:"stock_name,omitempty"`
	Volume     int64  `json:"volume"`
	ReasonCode string `json:"reason_code"`
	Message    string `json:"message,omitempty"`
}

// HoldingEvalSummary is response-level aggregates.
type HoldingEvalSummary struct {
	LotCount            int            `json:"lot_count"`
	TotalVolume         int64          `json:"total_volume"`
	TotalMarketValue    *float64       `json:"total_market_value,omitempty"`
	TotalUnrealizedPnL  *float64       `json:"total_unrealized_pnl,omitempty"`
	ByEvalState         map[string]int `json:"by_eval_state"`
	UnattributableCount int            `json:"unattributable_count"`
}

// ProjectHoldingEvaluation projects evaluation lots from an attribution view + price provider.
// Pure function: no DB writes. Unattributed qty never becomes a forged lot.
func ProjectHoldingEvaluation(
	attr *PositionAttributionView,
	prices MarketPriceProvider,
	opts HoldingEvaluationOptions,
) *HoldingEvaluationView {
	asOf := opts.AsOf
	if asOf.IsZero() {
		if attr != nil && !attr.AsOf.IsZero() {
			asOf = attr.AsOf
		} else {
			asOf = time.Now()
		}
	}
	out := &HoldingEvaluationView{
		Enabled:        true,
		AsOf:           asOf,
		Holdings:       []HoldingLotEvaluation{},
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
	if attr == nil {
		out.ReconcileAllMatched = true
		return out
	}
	out.Enabled = attr.Enabled
	out.AccountID = attr.AccountID
	out.ReconcileAllMatched = attr.ReconcileAllMatched

	filter := normalizeAttrCode(opts.StockCode)
	var (
		totalVol int64
		sumMV    float64
		sumPnL   float64
		haveMV   bool
		havePnL  bool
	)

	for _, row := range attr.Positions {
		code := normalizeAttrCode(row.StockCode)
		if filter != "" && code != filter {
			continue
		}
		if row.Reconcile.Status != ReconcileStatusMatched {
			out.ReconcileAllMatched = false
		}
		if row.Unattributed != nil && row.Unattributed.Volume > 0 {
			out.Unattributable = append(out.Unattributable, UnattributedHolding{
				StockCode:  row.StockCode,
				StockName:  row.StockName,
				Volume:     row.Unattributed.Volume,
				ReasonCode: row.Unattributed.ReasonCode,
				Message:    row.Unattributed.Message,
			})
		} else if row.Reconcile.UnattributedVolume > 0 {
			out.Unattributable = append(out.Unattributable, UnattributedHolding{
				StockCode:  row.StockCode,
				StockName:  row.StockName,
				Volume:     row.Reconcile.UnattributedVolume,
				ReasonCode: "POSITION_GT_FILLS",
				Message:    fmt.Sprintf("position_volume=%d exceeds attributed buy fills=%d", row.Reconcile.PositionVolume, row.Reconcile.AttributedVolume),
			})
		}

		for _, lot := range row.Lots {
			if lot.FillID == 0 {
				continue // never forge
			}
			ev := evaluateLot(row, lot, prices, asOf, opts)
			out.Holdings = append(out.Holdings, ev)
			totalVol += ev.Volume
			out.Summary.ByEvalState[ev.EvalState]++
			if ev.MarketValue != nil {
				sumMV += *ev.MarketValue
				haveMV = true
			}
			if ev.UnrealizedPnL != nil {
				sumPnL += *ev.UnrealizedPnL
				havePnL = true
			}
		}
	}

	sort.SliceStable(out.Holdings, func(i, j int) bool {
		if out.Holdings[i].StockCode != out.Holdings[j].StockCode {
			return out.Holdings[i].StockCode < out.Holdings[j].StockCode
		}
		if out.Holdings[i].BuyDate != out.Holdings[j].BuyDate {
			return out.Holdings[i].BuyDate < out.Holdings[j].BuyDate
		}
		return out.Holdings[i].LotID < out.Holdings[j].LotID
	})

	out.Summary.LotCount = len(out.Holdings)
	out.Summary.TotalVolume = totalVol
	out.Summary.UnattributableCount = len(out.Unattributable)
	if haveMV {
		v := sumMV
		out.Summary.TotalMarketValue = &v
	}
	if havePnL {
		v := sumPnL
		out.Summary.TotalUnrealizedPnL = &v
	}
	return out
}

func evaluateLot(
	row PositionAttributionRow,
	lot PositionLotDTO,
	prices MarketPriceProvider,
	asOf time.Time,
	opts HoldingEvaluationOptions,
) HoldingLotEvaluation {
	name := row.StockName
	if opts.NameByFillID != nil {
		if n, ok := opts.NameByFillID[lot.FillID]; ok && strings.TrimSpace(n) != "" {
			name = n
		}
	}
	cost := lot.FillPrice
	if cost == 0 && lot.Volume > 0 && lot.CostAmount != 0 {
		cost = lot.CostAmount / float64(lot.Volume)
	}
	buyDate := strings.TrimSpace(lot.TradeDate)
	ev := HoldingLotEvaluation{
		LotID:             lot.FillID,
		PlanID:            lot.PlanID,
		PlanItemID:        lot.PlanItemID,
		StockCode:         row.StockCode,
		StockNameSnapshot: name,
		Volume:            lot.Volume,
		CostPrice:         cost,
		BuyDate:           buyDate,
		HoldingDays:       calendarHoldingDays(buyDate, asOf),
		EvalState:         EvalStateNormal, // D.1.1: no complex policy
	}

	var quote MarkQuote
	ok := false
	if prices != nil {
		quote, ok = prices.MarkPrice(row.StockCode, asOf)
	}
	if !ok || quote.Quality == "missing" {
		ev.PriceQuality = "missing"
		if quote.Source != "" {
			ev.PriceSource = quote.Source
		}
		return ev
	}
	px := quote.Price
	ev.CurrentPrice = &px
	ev.PriceSource = quote.Source
	ev.PriceQuality = "ok"
	if !quote.AsOf.IsZero() {
		t := quote.AsOf
		ev.QuoteTime = &t
	}
	mv := px * float64(lot.Volume)
	ev.MarketValue = &mv
	pnl := (px - cost) * float64(lot.Volume)
	ev.UnrealizedPnL = &pnl
	if cost > 0 && !math.IsNaN(cost) && !math.IsInf(cost, 0) {
		retPct := (px - cost) / cost * 100
		ev.UnrealizedReturnPct = &retPct
	}
	return ev
}

// calendarHoldingDays returns natural-day holding span (asOf date − buy_date).
// Missing/invalid buy_date → 0; future buy_date → 0.
func calendarHoldingDays(buyDate string, asOf time.Time) int {
	buyDate = strings.TrimSpace(buyDate)
	if buyDate == "" || asOf.IsZero() {
		return 0
	}
	buy, err := time.ParseInLocation("2006-01-02", buyDate, asOf.Location())
	if err != nil {
		buy, err = time.Parse("2006-01-02", buyDate)
		if err != nil {
			return 0
		}
		buy = time.Date(buy.Year(), buy.Month(), buy.Day(), 0, 0, 0, 0, asOf.Location())
	}
	asOfDay := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, asOf.Location())
	buyDay := time.Date(buy.Year(), buy.Month(), buy.Day(), 0, 0, 0, 0, asOf.Location())
	if asOfDay.Before(buyDay) {
		return 0
	}
	return int(asOfDay.Sub(buyDay).Hours() / 24)
}

// HoldingEvaluationBuildOptions filters the DB-backed builder.
type HoldingEvaluationBuildOptions struct {
	StockCode string
	AsOf      time.Time
}

// BuildHoldingEvaluation loads attribution then projects evaluation using position marks
// as the default MarketPriceProvider. Read-only; no schema/write-path changes.
func BuildHoldingEvaluation(opts HoldingEvaluationBuildOptions) (*HoldingEvaluationView, error) {
	attr, err := BuildPositionAttribution(AttributionOptions{StockCode: opts.StockCode})
	if err != nil {
		out := ProjectHoldingEvaluation(nil, nil, HoldingEvaluationOptions{AsOf: opts.AsOf, StockCode: opts.StockCode})
		out.Enabled = IsEnabled()
		return out, err
	}
	provider := NewAttributionMarkPriceProvider(attr)
	view := ProjectHoldingEvaluation(attr, provider, HoldingEvaluationOptions{
		AsOf:      opts.AsOf,
		StockCode: opts.StockCode,
	})
	view.PriceProviderNote = "attribution_position_mark"
	// Enrich stock_name_snapshot from plan items when available (read-only).
	enrichHoldingNamesFromItems(view)
	return view, nil
}

func enrichHoldingNamesFromItems(view *HoldingEvaluationView) {
	if view == nil || len(view.Holdings) == 0 || dbDaoUnavailable() {
		return
	}
	itemIDs := uniqueUint(len(view.Holdings), func(i int) uint { return view.Holdings[i].PlanItemID })
	if len(itemIDs) == 0 {
		return
	}
	type nameRow struct {
		ID        uint
		StockName string
	}
	var rows []nameRow
	if err := db.Dao.Table("trade_plan_items").Select("id, stock_name").Where("id IN ?", itemIDs).Scan(&rows).Error; err != nil {
		return
	}
	byID := map[uint]string{}
	for _, r := range rows {
		if strings.TrimSpace(r.StockName) != "" {
			byID[r.ID] = r.StockName
		}
	}
	for i := range view.Holdings {
		if n, ok := byID[view.Holdings[i].PlanItemID]; ok {
			view.Holdings[i].StockNameSnapshot = n
		}
	}
}

func dbDaoUnavailable() bool {
	return db.Dao == nil
}

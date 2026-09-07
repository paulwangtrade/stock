// Position Evaluation Explanation — Phase17 C2 read-only explainability layer.
//
// Sits between HoldingEval and ExitEval. Produces human-readable hold/risk tags only.
// Does NOT emit BUY/SELL, does NOT write Broker/Gateway/TradePlan execution.

package papertrading

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Explanation source_type (observation only).
const (
	ExplanationSourceStrategy  = "Strategy"
	ExplanationSourceWatchlist = "Watchlist"
	ExplanationSourceManual    = "Manual"
	ExplanationSourceUnknown   = "Unknown"
)

// Hold explanation tags (non-trading).
const (
	ExplainTagSignalActive     = "SIGNAL_ACTIVE"
	ExplainTagTrendSupport     = "TREND_SUPPORT"
	ExplainTagProfitProtection = "PROFIT_PROTECTION"
)

// Risk explanation tags (non-trading; not sell actions).
const (
	ExplainTagSignalExpired = "SIGNAL_EXPIRED"
	ExplainTagLossControl   = "LOSS_CONTROL"
	ExplainTagPriceStale    = "PRICE_STALE"
	ExplainTagNoSourceTrace = "NO_SOURCE_TRACE"
)

// Freshness status for evaluation inputs (does not mutate kline cache).
const (
	EvalFreshnessFresh   = "FRESH"
	EvalFreshnessStale   = "STALE"
	EvalFreshnessUnknown = "UNKNOWN"
)

const (
	defaultEvalPriceStaleAfter   = 24 * time.Hour
	defaultEvalKlineStaleAfter   = 36 * time.Hour
	defaultSignalActiveMaxDays   = 20
	positionExplanationNote      = "Position Evaluation Explanation · read-only; no BUY/SELL actions"
)

// EvaluationDataFreshness marks age of price / kline inputs for an evaluation.
type EvaluationDataFreshness struct {
	PriceAgeSec  *int64  `json:"price_age_seconds,omitempty"`
	KlineAgeSec  *int64  `json:"kline_age_seconds,omitempty"`
	PriceStatus  string  `json:"price_status"`
	KlineStatus  string  `json:"kline_status"`
	Status       string  `json:"status"` // overall: worst of price/kline
	PriceAge     string  `json:"price_age,omitempty"`
	KlineAge     string  `json:"kline_age,omitempty"`
}

// SignalContextSnapshot copies TradePlanOrigin / SignalPrice fields (no new SignalEvent table).
type SignalContextSnapshot struct {
	SignalSnapshotID uint    `json:"signal_snapshot_id,omitempty"`
	SignalTime       string  `json:"signal_time,omitempty"`
	SignalPrice      float64 `json:"signal_price,omitempty"`
	SignalTag        string  `json:"signal_tag,omitempty"`
	Present          bool    `json:"present"`
}

// PositionEvaluationExplanation is the C2 explainability DTO (observation only).
type PositionEvaluationExplanation struct {
	StockCode       string                   `json:"stock_code"`
	EvaluationTime  time.Time                `json:"evaluation_time"`
	PositionDays    int                      `json:"position_days"`
	CostPrice       *float64                 `json:"cost_price,omitempty"`
	CurrentPrice    *float64                 `json:"current_price,omitempty"`
	PnL             *float64                 `json:"pnl,omitempty"`
	SourceType      string                   `json:"source_type"`
	SignalContext   SignalContextSnapshot    `json:"signal_context"`
	HoldReasons     []string                 `json:"hold_reasons"`
	RiskHints       []string                 `json:"risk_hints"`
	Freshness       EvaluationDataFreshness  `json:"freshness"`
	DataSourceNote  string                   `json:"data_source_note"`
}

// ExplanationSourceHint is optional provenance input (from Origin / plan SourceSession).
type ExplanationSourceHint struct {
	SourceType       string
	SignalSnapshotID uint
	SignalTime       string
	SignalPrice      float64
	SignalTag        string
	SignalPresent    bool
	StrategyName     string
	PlanSourceSession string
	// Optional kline last-bar / fetch time for freshness (caller-supplied; never writes cache).
	KlineAsOf *time.Time
}

// ExplanationOptions tunes thresholds for tags / freshness (tests override).
type ExplanationOptions struct {
	PriceStaleAfter     time.Duration
	KlineStaleAfter     time.Duration
	SignalActiveMaxDays int
	AsOf                time.Time
}

func (o ExplanationOptions) normalized() ExplanationOptions {
	if o.PriceStaleAfter <= 0 {
		o.PriceStaleAfter = defaultEvalPriceStaleAfter
	}
	if o.KlineStaleAfter <= 0 {
		o.KlineStaleAfter = defaultEvalKlineStaleAfter
	}
	if o.SignalActiveMaxDays <= 0 {
		o.SignalActiveMaxDays = defaultSignalActiveMaxDays
	}
	if o.AsOf.IsZero() {
		o.AsOf = time.Now()
	}
	return o
}

// BuildPositionEvaluationExplanation projects hold/risk tags from HoldingEval + optional Origin hint.
// Pure over inputs: no DB, no trade actions.
func BuildPositionEvaluationExplanation(
	stock HoldingEvalStockRow,
	hint ExplanationSourceHint,
	opts ExplanationOptions,
) PositionEvaluationExplanation {
	opts = opts.normalized()
	code := strings.TrimSpace(stock.StockCode)
	src := ClassifyExplanationSourceType(hint.PlanSourceSession, hint.StrategyName, hint.SignalPresent || hint.SourceType != "")
	if hint.SourceType != "" {
		src = normalizeExplanationSourceType(hint.SourceType)
	}

	sig := SignalContextSnapshot{
		SignalSnapshotID: hint.SignalSnapshotID,
		SignalTime:       strings.TrimSpace(hint.SignalTime),
		SignalPrice:      hint.SignalPrice,
		SignalTag:        strings.TrimSpace(hint.SignalTag),
		Present:          hint.SignalPresent || hint.SignalSnapshotID > 0 ||
			strings.TrimSpace(hint.SignalTime) != "" || hint.SignalPrice > 0 ||
			strings.TrimSpace(hint.SignalTag) != "",
	}

	fresh := ClassifyEvaluationDataFreshness(stock.QuoteTime, hint.KlineAsOf, opts)

	ex := PositionEvaluationExplanation{
		StockCode:      code,
		EvaluationTime: opts.AsOf,
		PositionDays:   stock.HoldingDays,
		CostPrice:      stock.AvgCost,
		CurrentPrice:   firstNonNilFloat(stock.CurrentPrice, stock.MarketPrice),
		PnL:            stock.UnrealizedPnL,
		SourceType:     src,
		SignalContext:  sig,
		HoldReasons:    []string{},
		RiskHints:      []string{},
		Freshness:      fresh,
		DataSourceNote: positionExplanationNote,
	}

	hold, risk := classifyExplanationTags(stock, src, sig, fresh, opts)
	ex.HoldReasons = hold
	ex.RiskHints = risk
	return ex
}

func firstNonNilFloat(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}

// ClassifyExplanationSourceType maps TradePlan SourceSession (+ strategy/signal) → source_type.
func ClassifyExplanationSourceType(sourceSession, strategyName string, hasSignalOrStrategy bool) string {
	switch strings.ToLower(strings.TrimSpace(sourceSession)) {
	case models.TradePlanSourceWatchlist:
		return ExplanationSourceWatchlist
	case models.TradePlanSourceTSell, models.TradePlanSourceExitReview:
		return ExplanationSourceManual
	case models.TradePlanSourceAfterClose,
		models.TradePlanSourceMorningRebuild,
		models.TradePlanSourceCashRescale:
		return ExplanationSourceStrategy
	}
	if strings.TrimSpace(strategyName) != "" || hasSignalOrStrategy {
		return ExplanationSourceStrategy
	}
	return ExplanationSourceUnknown
}

func normalizeExplanationSourceType(raw string) string {
	switch strings.TrimSpace(raw) {
	case ExplanationSourceStrategy, ExplanationSourceWatchlist, ExplanationSourceManual, ExplanationSourceUnknown:
		return strings.TrimSpace(raw)
	case "strategy":
		return ExplanationSourceStrategy
	case "watchlist":
		return ExplanationSourceWatchlist
	case "manual":
		return ExplanationSourceManual
	default:
		return ExplanationSourceUnknown
	}
}

// ClassifyEvaluationDataFreshness derives FRESH/STALE/UNKNOWN without touching kline cache.
func ClassifyEvaluationDataFreshness(quoteTime *time.Time, klineAsOf *time.Time, opts ExplanationOptions) EvaluationDataFreshness {
	opts = opts.normalized()
	out := EvaluationDataFreshness{
		PriceStatus: EvalFreshnessUnknown,
		KlineStatus: EvalFreshnessUnknown,
		Status:      EvalFreshnessUnknown,
	}
	if quoteTime != nil && !quoteTime.IsZero() {
		age := opts.AsOf.Sub(*quoteTime)
		if age < 0 {
			age = 0
		}
		sec := int64(age / time.Second)
		out.PriceAgeSec = &sec
		out.PriceAge = formatDurationShort(age)
		if age > opts.PriceStaleAfter {
			out.PriceStatus = EvalFreshnessStale
		} else {
			out.PriceStatus = EvalFreshnessFresh
		}
	}
	if klineAsOf != nil && !klineAsOf.IsZero() {
		age := opts.AsOf.Sub(*klineAsOf)
		if age < 0 {
			age = 0
		}
		sec := int64(age / time.Second)
		out.KlineAgeSec = &sec
		out.KlineAge = formatDurationShort(age)
		if age > opts.KlineStaleAfter {
			out.KlineStatus = EvalFreshnessStale
		} else {
			out.KlineStatus = EvalFreshnessFresh
		}
	}
	out.Status = worstFreshness(out.PriceStatus, out.KlineStatus)
	return out
}

func formatDurationShort(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func worstFreshness(a, b string) string {
	rank := func(s string) int {
		switch s {
		case EvalFreshnessStale:
			return 3
		case EvalFreshnessUnknown:
			return 2
		case EvalFreshnessFresh:
			return 1
		default:
			return 0
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	if a == "" {
		return EvalFreshnessUnknown
	}
	return a
}

func classifyExplanationTags(
	stock HoldingEvalStockRow,
	src string,
	sig SignalContextSnapshot,
	fresh EvaluationDataFreshness,
	opts ExplanationOptions,
) (hold []string, risk []string) {
	holdSet := map[string]bool{}
	riskSet := map[string]bool{}

	if !sig.Present && src == ExplanationSourceUnknown {
		riskSet[ExplainTagNoSourceTrace] = true
	}

	if sig.Present {
		if signalExpired(sig.SignalTime, opts) {
			riskSet[ExplainTagSignalExpired] = true
		} else {
			holdSet[ExplainTagSignalActive] = true
		}
	}

	switch strings.ToUpper(strings.TrimSpace(stock.TrendState)) {
	case TrendStateUp, TrendStateSideway:
		holdSet[ExplainTagTrendSupport] = true
	}

	profit := stock.ProfitState
	if profit == "" {
		profit = ClassifyProfitState(stock.UnrealizedReturn)
	}
	if profit == ProfitStateProfit {
		holdSet[ExplainTagProfitProtection] = true
	}

	riskState := stock.RiskState
	if riskState == "" {
		riskState = ClassifyRiskState(stock.UnrealizedReturn, DefaultRiskStateThresholds)
	}
	if profit == ProfitStateLoss || riskState == RiskStateWatch || riskState == RiskStateDanger {
		riskSet[ExplainTagLossControl] = true
	}

	if fresh.PriceStatus == EvalFreshnessStale {
		riskSet[ExplainTagPriceStale] = true
	}

	return sortedExplainTags(holdSet, []string{
		ExplainTagSignalActive, ExplainTagTrendSupport, ExplainTagProfitProtection,
	}), sortedExplainTags(riskSet, []string{
		ExplainTagSignalExpired, ExplainTagLossControl, ExplainTagPriceStale, ExplainTagNoSourceTrace,
	})
}

func signalExpired(signalTime string, opts ExplanationOptions) bool {
	opts = opts.normalized()
	t, ok := parseExplainSignalTime(signalTime, opts.AsOf.Location())
	if !ok {
		// Present but unparseable/missing time → do not invent SIGNAL_EXPIRED.
		return false
	}
	asOfDay := time.Date(opts.AsOf.Year(), opts.AsOf.Month(), opts.AsOf.Day(), 0, 0, 0, 0, opts.AsOf.Location())
	sigDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, opts.AsOf.Location())
	days := int(asOfDay.Sub(sigDay).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days > opts.SignalActiveMaxDays
}

func parseExplainSignalTime(raw string, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "missing" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, true
		}
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func sortedExplainTags(set map[string]bool, order []string) []string {
	if len(set) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(set))
	for _, c := range order {
		if set[c] {
			out = append(out, c)
		}
	}
	extra := make([]string, 0)
	for c := range set {
		known := false
		for _, k := range order {
			if c == k {
				known = true
				break
			}
		}
		if !known {
			extra = append(extra, c)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// ExplainTagLabelZH is a display hint (not an action).
func ExplainTagLabelZH(code string) string {
	switch strings.TrimSpace(code) {
	case ExplainTagSignalActive:
		return "当前仍有有效信号"
	case ExplainTagTrendSupport:
		return "趋势结构未破坏"
	case ExplainTagProfitProtection:
		return "已盈利，保护利润"
	case ExplainTagSignalExpired:
		return "原始信号过期"
	case ExplainTagLossControl:
		return "当前亏损扩大"
	case ExplainTagPriceStale:
		return "当前价格数据过期"
	case ExplainTagNoSourceTrace:
		return "无来源信息"
	default:
		return code
	}
}

// --- ExitEval wiring helpers ---

// EnrichHoldingWithExplanations attaches PositionEvaluationExplanation on each holding stock row.
func EnrichHoldingWithExplanations(
	holding *HoldingEvalObservationView,
	hints map[string]ExplanationSourceHint,
	opts ExplanationOptions,
) {
	if holding == nil {
		return
	}
	opts = opts.normalized()
	if opts.AsOf.IsZero() && !holding.AsOf.IsZero() {
		opts.AsOf = holding.AsOf
	}
	for i := range holding.Holdings {
		code := normalizeAttrCode(holding.Holdings[i].StockCode)
		var hint ExplanationSourceHint
		if hints != nil {
			hint = hints[code]
			if hint.SourceType == "" && hint.SignalSnapshotID == 0 && !hint.SignalPresent {
				// also try raw code key
				hint = hints[holding.Holdings[i].StockCode]
			}
		}
		ex := BuildPositionEvaluationExplanation(holding.Holdings[i], hint, opts)
		holding.Holdings[i].Explanation = &ex
	}
}

// LoadExplanationSourceHints resolves plan SourceSession + SignalPrice fields for stocks in a holding view.
// Reuses TradePlan / CandidatePoolItem / SignalScanHit (SignalEvent embed) without tradeplanorigin package
// (avoids papertrading ↔ opportunity import cycle). Read-only; nil DB → empty map.
func LoadExplanationSourceHints(holding *HoldingEvalObservationView) map[string]ExplanationSourceHint {
	out := map[string]ExplanationSourceHint{}
	if holding == nil {
		return out
	}
	type ref struct {
		code       string
		planID     uint
		planItemID uint
	}
	refs := make([]ref, 0)
	planIDs := map[uint]struct{}{}
	itemIDs := map[uint]struct{}{}
	for _, h := range holding.Holdings {
		code := normalizeAttrCode(h.StockCode)
		var planID, itemID uint
		for _, lot := range h.Lots {
			if lot.PlanID > 0 && planID == 0 {
				planID = lot.PlanID
			}
			if lot.PlanItemID > 0 && itemID == 0 {
				itemID = lot.PlanItemID
			}
			if planID > 0 && itemID > 0 {
				break
			}
		}
		if code == "" {
			continue
		}
		refs = append(refs, ref{code: code, planID: planID, planItemID: itemID})
		if planID > 0 {
			planIDs[planID] = struct{}{}
		}
		if itemID > 0 {
			itemIDs[itemID] = struct{}{}
		}
	}
	if len(refs) == 0 {
		return out
	}

	sessionByPlan := map[uint]string{}
	poolByPlan := map[uint]uint{}
	itemsByID := map[uint]models.TradePlanItem{}
	if !dbDaoUnavailable() && db.Dao != nil {
		if len(planIDs) > 0 {
			ids := make([]uint, 0, len(planIDs))
			for id := range planIDs {
				ids = append(ids, id)
			}
			var plans []models.TradePlan
			if err := db.Dao.Select("id, source_session, pool_id").Where("id IN ?", ids).Find(&plans).Error; err == nil {
				for _, p := range plans {
					sessionByPlan[p.ID] = p.SourceSession
					poolByPlan[p.ID] = p.PoolID
				}
			}
		}
		if len(itemIDs) > 0 {
			ids := make([]uint, 0, len(itemIDs))
			for id := range itemIDs {
				ids = append(ids, id)
			}
			var items []models.TradePlanItem
			if err := db.Dao.Where("id IN ?", ids).Find(&items).Error; err == nil {
				for _, it := range items {
					itemsByID[it.ID] = it
				}
			}
		}
	}

	snapshotCache := map[uint][]models.SignalScanHit{}
	for _, r := range refs {
		hint := ExplanationSourceHint{
			PlanSourceSession: sessionByPlan[r.planID],
		}
		if it, ok := itemsByID[r.planItemID]; ok {
			hint.StrategyName = strings.TrimSpace(it.StrategyName)
		}
		if !dbDaoUnavailable() && db.Dao != nil {
			enrichHintFromPoolAndSnapshot(&hint, r.code, poolByPlan[r.planID], snapshotCache)
		}
		hint.SourceType = ClassifyExplanationSourceType(hint.PlanSourceSession, hint.StrategyName, hint.SignalPresent)
		out[r.code] = hint
	}
	return out
}

func enrichHintFromPoolAndSnapshot(
	hint *ExplanationSourceHint,
	code string,
	poolID uint,
	snapshotCache map[uint][]models.SignalScanHit,
) {
	if hint == nil || poolID == 0 || db.Dao == nil {
		return
	}
	var poolItem models.CandidatePoolItem
	err := db.Dao.Where("pool_id = ? AND stock_code = ?", poolID, code).First(&poolItem).Error
	if err != nil {
		// try loose match on trailing digits
		var items []models.CandidatePoolItem
		if db.Dao.Where("pool_id = ?", poolID).Find(&items).Error != nil {
			return
		}
		found := false
		for _, it := range items {
			if normalizeAttrCode(it.StockCode) == code {
				poolItem = it
				found = true
				break
			}
		}
		if !found {
			return
		}
	}
	if hint.StrategyName == "" {
		hint.StrategyName = strings.TrimSpace(poolItem.StrategyName)
	}
	hint.SignalTag = strings.TrimSpace(poolItem.SignalTag)
	hint.SignalSnapshotID = poolItem.SignalSnapshotID
	if poolItem.SignalSnapshotID == 0 {
		hint.SignalPresent = hint.SignalTag != ""
		return
	}
	hits, ok := snapshotCache[poolItem.SignalSnapshotID]
	if !ok {
		hits = loadSignalHitsForExplanation(poolItem.SignalSnapshotID)
		snapshotCache[poolItem.SignalSnapshotID] = hits
	}
	for _, h := range hits {
		if normalizeAttrCode(h.SECURITY_CODE) == code || normalizeAttrCode(h.SECUCODE) == code {
			if hint.SignalTag == "" {
				hint.SignalTag = strings.TrimSpace(h.Tag)
			}
			hint.SignalTime = strings.TrimSpace(h.SignalTime)
			if h.SignalPrice > 0 {
				hint.SignalPrice = h.SignalPrice
			}
			break
		}
	}
	hint.SignalPresent = hint.SignalSnapshotID > 0 ||
		hint.SignalTime != "" || hint.SignalPrice > 0 || hint.SignalTag != ""
}

func loadSignalHitsForExplanation(snapshotID uint) []models.SignalScanHit {
	if snapshotID == 0 || db.Dao == nil {
		return nil
	}
	var snap models.SignalScanSnapshot
	if err := db.Dao.Select("id, result_json").Where("id = ?", snapshotID).First(&snap).Error; err != nil {
		return nil
	}
	raw := strings.TrimSpace(snap.ResultJSON)
	if raw == "" {
		return nil
	}
	var payload models.SignalScanResultPayload
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return nil
	}
	return payload.Items
}

// Holding T-Suitability — Phase17.1 read-only "做 T" suitability projection.
//
// Pipeline: HoldingEval → Explanation → HealthScore → HoldingTSuitability.
// Does NOT emit BUY/SELL, does NOT change ExitEval state, Broker, Settlement, or mark_price.

package papertrading

import (
	"strings"
	"time"

	"go-stock/backend/db"
)

// T-suitability levels (observation only — not trade signals).
const (
	TSuitabilitySuitable    = "suitable"
	TSuitabilityCaution     = "caution"
	TSuitabilityUnsuitable  = "unsuitable"
)

// VolatilityStatus from simple 5m amplitude (no MACD/RSI).
const (
	VolatilityActive  = "ACTIVE"
	VolatilityNormal  = "NORMAL"
	VolatilityLow     = "LOW"
	VolatilityUnknown = "UNKNOWN"
)

// Reason codes (explain-only; no BUY/SELL wording).
const (
	TSuitReasonLockedPosition    = "LOCKED_POSITION"
	TSuitReasonNoSellable        = "NO_SELLABLE"
	TSuitReasonPriceStale        = "PRICE_STALE"
	TSuitReasonLowVolatility     = "LOW_VOLATILITY"
	TSuitReasonVolatilityUnknown = "VOLATILITY_UNKNOWN"
	TSuitReasonHealthWatch       = "HEALTH_WATCH"
	TSuitReasonHealthRisk        = "HEALTH_RISK"
	TSuitReasonHealthUnknown     = "HEALTH_UNKNOWN"
	TSuitReasonPartialLocked     = "PARTIAL_LOCKED"
)

const (
	tSuitVolWindowBars     = 12
	tSuitVolActiveMin      = 0.010 // ≥ 1.0%
	tSuitVolNormalMin      = 0.004 // ≥ 0.4%
	holdingTSuitabilityNote = "Holding T-Suitability · position operability only; not a trade signal"
)

// HoldingTSuitability is the read-only 做 T suitability DTO.
type HoldingTSuitability struct {
	StockCode        string    `json:"stock_code"`
	Level            string    `json:"level"` // suitable | caution | unsuitable
	Reasons          []string  `json:"reasons"`
	CanSell          bool      `json:"can_sell"`
	Freshness        string    `json:"freshness"`         // FRESH | STALE | UNKNOWN
	VolatilityStatus string    `json:"volatility_status"` // ACTIVE | NORMAL | LOW | UNKNOWN
	HealthGrade      string    `json:"health_grade"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	DataSourceNote   string    `json:"data_source_note,omitempty"`
}

// HoldingTSuitabilityInput is the pure-function input (no DB required for unit tests).
type HoldingTSuitabilityInput struct {
	StockCode        string
	CanSell          bool
	AvailableQty     int64
	LockedQty        int64
	Freshness        string
	HasPriceStaleTag bool
	VolatilityStatus string
	HealthGrade      string
	AsOf             time.Time
}

// AmplitudeBar is a minimal OHLC bar for 5m volatility (high/low/close only).
type AmplitudeBar struct {
	High  float64
	Low   float64
	Close float64
}

// BuildHoldingTSuitability projects Level + Reasons. Nil-safe; never panics on zero input.
func BuildHoldingTSuitability(in HoldingTSuitabilityInput) HoldingTSuitability {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	code := strings.TrimSpace(in.StockCode)
	fresh := normalizeTSuitFreshness(in.Freshness)
	vol := normalizeVolatilityStatus(in.VolatilityStatus)
	grade := strings.TrimSpace(strings.ToUpper(in.HealthGrade))

	canSell := in.CanSell
	if in.AvailableQty <= 0 {
		canSell = false
	}

	out := HoldingTSuitability{
		StockCode:        code,
		Level:            TSuitabilitySuitable,
		Reasons:          []string{},
		CanSell:          canSell,
		Freshness:        fresh,
		VolatilityStatus: vol,
		HealthGrade:      grade,
		EvaluatedAt:      asOf,
		DataSourceNote:   holdingTSuitabilityNote,
	}

	reasons := make([]string, 0, 6)
	level := TSuitabilitySuitable

	// --- Layer 1: operability (hard) ---
	hardUnsuitable := false
	if !canSell {
		hardUnsuitable = true
		if in.LockedQty > 0 {
			reasons = append(reasons, TSuitReasonLockedPosition)
		} else {
			reasons = append(reasons, TSuitReasonNoSellable)
		}
	}
	if fresh == EvalFreshnessStale || in.HasPriceStaleTag {
		hardUnsuitable = true
		reasons = appendUniqueReason(reasons, TSuitReasonPriceStale)
	}
	if hardUnsuitable {
		level = TSuitabilityUnsuitable
	}

	// --- Layer 2: volatility ---
	switch vol {
	case VolatilityLow:
		level = worseTSuitLevel(level, TSuitabilityCaution)
		reasons = appendUniqueReason(reasons, TSuitReasonLowVolatility)
	case VolatilityUnknown:
		level = worseTSuitLevel(level, TSuitabilityCaution)
		reasons = appendUniqueReason(reasons, TSuitReasonVolatilityUnknown)
	}

	// --- Layer 3: health (soft — never hard unsuitable) ---
	switch grade {
	case HealthGradeC:
		level = worseTSuitLevel(level, TSuitabilityCaution)
		reasons = appendUniqueReason(reasons, TSuitReasonHealthWatch)
	case HealthGradeD:
		level = worseTSuitLevel(level, TSuitabilityCaution)
		reasons = appendUniqueReason(reasons, TSuitReasonHealthRisk)
	case "":
		if !hardUnsuitable {
			// Missing health is informational; mild caution only when still suitable path.
			level = worseTSuitLevel(level, TSuitabilityCaution)
			reasons = appendUniqueReason(reasons, TSuitReasonHealthUnknown)
		}
	}

	// Optional note: partial lock but still sellable.
	if canSell && in.LockedQty > 0 && level != TSuitabilityUnsuitable {
		reasons = appendUniqueReason(reasons, TSuitReasonPartialLocked)
		level = worseTSuitLevel(level, TSuitabilityCaution)
	}

	out.Level = level
	out.Reasons = reasons
	return out
}

func worseTSuitLevel(cur, candidate string) string {
	rank := func(l string) int {
		switch l {
		case TSuitabilityUnsuitable:
			return 3
		case TSuitabilityCaution:
			return 2
		case TSuitabilitySuitable:
			return 1
		default:
			return 0
		}
	}
	if rank(candidate) > rank(cur) {
		return candidate
	}
	return cur
}

func appendUniqueReason(dst []string, code string) []string {
	code = strings.TrimSpace(code)
	if code == "" {
		return dst
	}
	for _, r := range dst {
		if r == code {
			return dst
		}
	}
	return append(dst, code)
}

func normalizeTSuitFreshness(v string) string {
	s := strings.TrimSpace(strings.ToUpper(v))
	switch s {
	case EvalFreshnessFresh, EvalFreshnessStale, EvalFreshnessUnknown:
		return s
	case "":
		return EvalFreshnessUnknown
	default:
		return EvalFreshnessUnknown
	}
}

func normalizeVolatilityStatus(v string) string {
	s := strings.TrimSpace(strings.ToUpper(v))
	switch s {
	case VolatilityActive, VolatilityNormal, VolatilityLow, VolatilityUnknown:
		return s
	case "HIGH": // alias
		return VolatilityActive
	case "":
		return VolatilityUnknown
	default:
		return VolatilityUnknown
	}
}

// ClassifyVolatilityFromBars computes mean (high-low)/close over the last N 5m bars.
// Missing/insufficient bars → UNKNOWN. No MACD/RSI.
func ClassifyVolatilityFromBars(bars []AmplitudeBar) string {
	if len(bars) == 0 {
		return VolatilityUnknown
	}
	n := tSuitVolWindowBars
	if len(bars) < n {
		n = len(bars)
	}
	window := bars[len(bars)-n:]
	var sum float64
	var count int
	for _, b := range window {
		if b.Close <= 0 {
			continue
		}
		high, low := b.High, b.Low
		if high <= 0 {
			high = b.Close
		}
		if low <= 0 {
			low = b.Close
		}
		if high < low {
			high, low = low, high
		}
		sum += (high - low) / b.Close
		count++
	}
	if count == 0 {
		return VolatilityUnknown
	}
	amp := sum / float64(count)
	if amp >= tSuitVolActiveMin {
		return VolatilityActive
	}
	if amp >= tSuitVolNormalMin {
		return VolatilityNormal
	}
	return VolatilityLow
}

// BuildHoldingTSuitabilityFromStock projects from an enriched HoldingEval stock row + position gate.
func BuildHoldingTSuitabilityFromStock(
	stock *HoldingEvalStockRow,
	gate HoldingTSuitabilityPositionGate,
	volatilityStatus string,
	asOf time.Time,
) HoldingTSuitability {
	in := HoldingTSuitabilityInput{
		CanSell:          gate.CanSell,
		AvailableQty:     gate.AvailableQty,
		LockedQty:        gate.LockedQty,
		VolatilityStatus: volatilityStatus,
		AsOf:             asOf,
	}
	if stock != nil {
		in.StockCode = stock.StockCode
		if stock.Explanation != nil {
			in.Freshness = stock.Explanation.Freshness.Status
			if in.Freshness == "" {
				in.Freshness = stock.Explanation.Freshness.PriceStatus
			}
			for _, tag := range stock.Explanation.RiskHints {
				if tag == ExplainTagPriceStale {
					in.HasPriceStaleTag = true
					break
				}
			}
		}
		if stock.HealthScore != nil {
			in.HealthGrade = stock.HealthScore.Grade
		}
	}
	return BuildHoldingTSuitability(in)
}

// HoldingTSuitabilityPositionGate carries PositionState sellability (paper_sim).
type HoldingTSuitabilityPositionGate struct {
	CanSell      bool
	AvailableQty int64
	LockedQty    int64
}

// loadPositionGatesHook is overridable in tests.
var loadPositionGatesHook = loadPositionGatesFromDB

// SetLoadPositionGatesHookForTest swaps the position-gate loader; restore with returned func.
func SetLoadPositionGatesHookForTest(fn func(accountID uint) map[string]HoldingTSuitabilityPositionGate) func() {
	prev := loadPositionGatesHook
	if fn == nil {
		loadPositionGatesHook = loadPositionGatesFromDB
	} else {
		loadPositionGatesHook = fn
	}
	return func() { loadPositionGatesHook = prev }
}
func loadPositionGatesFromDB(accountID uint) map[string]HoldingTSuitabilityPositionGate {
	out := map[string]HoldingTSuitabilityPositionGate{}
	if db.Dao == nil {
		return out
	}
	q := db.Dao.Model(&PaperSimPosition{})
	if accountID > 0 {
		q = q.Where("account_id = ?", accountID)
	}
	var rows []PaperSimPosition
	if err := q.Find(&rows).Error; err != nil {
		return out
	}
	for _, p := range rows {
		code := normalizeAttrCode(p.StockCode)
		if code == "" {
			continue
		}
		avail := p.AvailableVolume
		out[code] = HoldingTSuitabilityPositionGate{
			CanSell:      avail > 0,
			AvailableQty: avail,
			LockedQty:    p.LockedVolume,
		}
	}
	return out
}

// EnrichHoldingWithTSuitability attaches HoldingTSuitability after HealthScore is present.
// volatilityByCode is optional (5m-derived); missing → UNKNOWN → caution.
// Never mutates ExitEval thresholds.
func EnrichHoldingWithTSuitability(holding *HoldingEvalObservationView, volatilityByCode map[string]string) {
	if holding == nil {
		return
	}
	gates := loadPositionGatesHook(holding.AccountID)
	asOf := holding.AsOf
	for i := range holding.Holdings {
		code := normalizeAttrCode(holding.Holdings[i].StockCode)
		gate := gates[code]
		// If no gate row, treat as not sellable when we have no position volume info.
		if _, ok := gates[code]; !ok {
			gate = HoldingTSuitabilityPositionGate{CanSell: false, AvailableQty: 0}
		}
		vol := ""
		if volatilityByCode != nil {
			vol = volatilityByCode[code]
			if vol == "" {
				vol = volatilityByCode[holding.Holdings[i].StockCode]
			}
		}
		ts := BuildHoldingTSuitabilityFromStock(&holding.Holdings[i], gate, vol, asOf)
		holding.Holdings[i].TSuitability = &ts
	}
}

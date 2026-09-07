// Holding T Signal — Phase17.6 read-only 做 T observation signals (v0).
//
// Does NOT emit trade orders, does NOT change ExitEval state, Broker, Settlement, or TradePlan.
// Distinct from HoldingTSuitability (environment gate) and from daily ice SignalEvent.

package papertrading

import (
	"math"
	"strings"
	"time"
)

// Observation signal types (WATCH only — not executable orders).
const (
	TSignalBuyWatch  = "T_BUY_WATCH"
	TSignalSellWatch = "T_SELL_WATCH"
)

// Signal observation strength (not suitability level).
const (
	TSignalLevelActive = "active"
	TSignalLevelSoft   = "soft"
)

// Reason codes (observation explain; no order wording).
const (
	TSignalReasonNoPosition       = "NO_POSITION"
	TSignalReasonCannotSell       = "CANNOT_SELL"
	TSignalReasonPriceStale       = "PRICE_STALE"
	TSignalReasonNoBars           = "NO_5M_BARS"
	TSignalReasonInvalidCost      = "INVALID_COST"
	TSignalReasonPullback         = "SHORT_PULLBACK"
	TSignalReasonBounce           = "BOUNCE_HINT"
	TSignalReasonNearCost         = "NEAR_COST"
	TSignalReasonRise             = "SHORT_RISE"
	TSignalReasonAwayCost         = "AWAY_FROM_COST"
	TSignalReasonMomentumFade     = "MOMENTUM_FADE"
	TSignalReasonSuitUnsuitable   = "SUITABILITY_UNSUITABLE"
	TSignalReasonHealthCaution    = "HEALTH_CAUTION"
)

const (
	holdingTSignalNote = "Holding T Signal v0 · observation WATCH only; not a trade order"

	tSignalMinBars       = 6
	tSignalNearCostMax   = 0.012 // |px-cost|/cost ≤ 1.2% → near cost
	tSignalAwayCostMin   = 0.008 // (px-cost)/cost ≥ 0.8% → away above cost (sell watch)
	tSignalPullbackMin   = 0.004 // ≥ 0.4% off local high
	tSignalRiseMin       = 0.005 // ≥ 0.5% off local low / prior
	tSignalBounceMin     = 0.002 // ≥ 0.2% off trough
	tSignalLookback      = 8
)

// HoldingTSignal is a read-only 做 T observation signal DTO.
type HoldingTSignal struct {
	StockCode    string    `json:"stock_code"`
	PositionID   string    `json:"position_id,omitempty"`
	SignalType   string    `json:"signal_type"` // T_BUY_WATCH | T_SELL_WATCH
	SignalTime   string    `json:"signal_time"`
	SignalPrice  float64   `json:"signal_price"`
	Level        string    `json:"level"` // active | soft
	Reasons      []string  `json:"reasons"`
	Confidence   float64   `json:"confidence"` // 0..1
	DataSourceNote string  `json:"data_source_note,omitempty"`
	EvaluatedAt  time.Time `json:"evaluated_at,omitempty"`
}

// HoldingTSignalBar is a minimal 5m OHLC bar (+ optional time label).
type HoldingTSignalBar struct {
	Time  string
	High  float64
	Low   float64
	Close float64
}

// HoldingTSignalInput is the pure-function input (no DB / Broker).
type HoldingTSignalInput struct {
	StockCode        string
	PositionID       string
	HasPosition      bool
	CanSell          bool
	AvailableQty     int64
	CostPrice        float64
	Freshness        string // FRESH | STALE | UNKNOWN
	SuitabilityLevel string // suitable | caution | unsuitable
	HealthGrade      string // A|B|C|D|""
	Bars             []HoldingTSignalBar
	AsOf             time.Time
}

// HoldingTSignalResult wraps zero-or-more observation signals plus gate notes.
type HoldingTSignalResult struct {
	Signals []HoldingTSignal `json:"signals"`
	Notes   []string         `json:"notes,omitempty"` // why empty / suppressed
}

// BuildHoldingTSignal projects observation WATCH signals (v0 heuristics).
// Nil-safe; never panics; never mutates trading state.
func BuildHoldingTSignal(in HoldingTSignalInput) HoldingTSignalResult {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	code := strings.TrimSpace(in.StockCode)
	posID := strings.TrimSpace(in.PositionID)
	out := HoldingTSignalResult{Signals: []HoldingTSignal{}, Notes: []string{}}

	hasPos := in.HasPosition || in.AvailableQty > 0
	if !hasPos {
		out.Notes = append(out.Notes, TSignalReasonNoPosition)
		return out
	}
	fresh := normalizeTSuitFreshness(in.Freshness)
	if fresh == EvalFreshnessStale {
		out.Notes = append(out.Notes, TSignalReasonPriceStale)
		return out
	}
	if len(in.Bars) < tSignalMinBars {
		out.Notes = append(out.Notes, TSignalReasonNoBars)
		return out
	}
	cost := in.CostPrice
	if cost <= 0 || math.IsNaN(cost) {
		out.Notes = append(out.Notes, TSignalReasonInvalidCost)
		return out
	}

	canSell := in.CanSell
	if in.AvailableQty <= 0 && !in.CanSell {
		canSell = false
	}

	bars := in.Bars
	last := bars[len(bars)-1]
	px := last.Close
	if px <= 0 {
		out.Notes = append(out.Notes, TSignalReasonNoBars)
		return out
	}
	sigTime := strings.TrimSpace(last.Time)
	if sigTime == "" {
		sigTime = asOf.Format(time.RFC3339)
	}

	suit := strings.TrimSpace(strings.ToLower(in.SuitabilityLevel))
	grade := strings.TrimSpace(strings.ToUpper(in.HealthGrade))

	baseConf := 0.55
	if fresh == EvalFreshnessUnknown {
		baseConf -= 0.08
	}
	if suit == TSuitabilityCaution {
		baseConf -= 0.08
	}
	if suit == TSuitabilityUnsuitable {
		baseConf -= 0.20
	}
	switch grade {
	case HealthGradeC, HealthGradeD:
		baseConf -= 0.08
	case "":
		baseConf -= 0.04
	}

	// --- Feature extraction (last lookback window) ---
	win := bars
	if len(win) > tSignalLookback {
		win = win[len(win)-tSignalLookback:]
	}
	localHigh, localLow := win[0].High, win[0].Low
	for _, b := range win {
		if b.High > localHigh {
			localHigh = b.High
		}
		if b.Low < localLow || localLow <= 0 {
			localLow = b.Low
		}
	}
	pullback := 0.0
	if localHigh > 0 {
		pullback = (localHigh - px) / localHigh
	}
	riseFromLow := 0.0
	if localLow > 0 {
		riseFromLow = (px - localLow) / localLow
	}
	distCost := (px - cost) / cost
	absDistCost := math.Abs(distCost)

	// Bounce: last close above prior bar low and above prior close (or off trough).
	prev := bars[len(bars)-2]
	bounce := px > prev.Low && (px >= prev.Close || (prev.Low > 0 && (px-prev.Low)/prev.Low >= tSignalBounceMin))

	// Momentum fade after rise: last close below prior close, or below mid of last bar range.
	momFade := false
	if riseFromLow >= tSignalRiseMin*0.8 {
		if px < prev.Close {
			momFade = true
		}
		if prev.High > prev.Low && px < (prev.High+prev.Low)/2 {
			momFade = true
		}
	}

	// --- T_BUY_WATCH ---
	if canSell {
		buyReasons := make([]string, 0, 4)
		buyScore := 0
		if pullback >= tSignalPullbackMin {
			buyReasons = append(buyReasons, TSignalReasonPullback)
			buyScore++
		}
		if bounce {
			buyReasons = append(buyReasons, TSignalReasonBounce)
			buyScore++
		}
		if absDistCost <= tSignalNearCostMax {
			buyReasons = append(buyReasons, TSignalReasonNearCost)
			buyScore++
		}
		// Prefer buy when not strongly extended above cost.
		if buyScore >= 2 && distCost <= tSignalAwayCostMin+0.004 {
			if suit == TSuitabilityUnsuitable {
				buyReasons = append(buyReasons, TSignalReasonSuitUnsuitable)
			}
			if grade == HealthGradeC || grade == HealthGradeD {
				buyReasons = append(buyReasons, TSignalReasonHealthCaution)
			}
			conf := clamp01(baseConf + 0.12*float64(buyScore-1))
			level := TSignalLevelSoft
			if buyScore >= 3 && conf >= 0.55 {
				level = TSignalLevelActive
			}
			out.Signals = append(out.Signals, HoldingTSignal{
				StockCode:      code,
				PositionID:     posID,
				SignalType:     TSignalBuyWatch,
				SignalTime:     sigTime,
				SignalPrice:    px,
				Level:          level,
				Reasons:        buyReasons,
				Confidence:     conf,
				DataSourceNote: holdingTSignalNote,
				EvaluatedAt:    asOf,
			})
		}
	} else {
		out.Notes = append(out.Notes, TSignalReasonCannotSell)
	}

	// --- T_SELL_WATCH ---
	sellReasons := make([]string, 0, 4)
	sellScore := 0
	if riseFromLow >= tSignalRiseMin || (prev.Close > 0 && (px-prev.Close)/prev.Close >= tSignalRiseMin*0.6) {
		sellReasons = append(sellReasons, TSignalReasonRise)
		sellScore++
	}
	if distCost >= tSignalAwayCostMin {
		sellReasons = append(sellReasons, TSignalReasonAwayCost)
		sellScore++
	}
	if momFade {
		sellReasons = append(sellReasons, TSignalReasonMomentumFade)
		sellScore++
	}
	if sellScore >= 2 {
		if suit == TSuitabilityUnsuitable {
			sellReasons = append(sellReasons, TSignalReasonSuitUnsuitable)
		}
		if grade == HealthGradeC || grade == HealthGradeD {
			sellReasons = append(sellReasons, TSignalReasonHealthCaution)
		}
		conf := clamp01(baseConf + 0.12*float64(sellScore-1))
		level := TSignalLevelSoft
		if sellScore >= 3 && conf >= 0.55 {
			level = TSignalLevelActive
		}
		out.Signals = append(out.Signals, HoldingTSignal{
			StockCode:      code,
			PositionID:     posID,
			SignalType:     TSignalSellWatch,
			SignalTime:     sigTime,
			SignalPrice:    px,
			Level:          level,
			Reasons:        sellReasons,
			Confidence:     conf,
			DataSourceNote: holdingTSignalNote,
			EvaluatedAt:    asOf,
		})
	}

	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return math.Round(v*100) / 100
}

// AmplitudeBarsToTSignalBars adapts volatility bars (no time) for signal builder tests/helpers.
func AmplitudeBarsToTSignalBars(bars []AmplitudeBar) []HoldingTSignalBar {
	out := make([]HoldingTSignalBar, 0, len(bars))
	for i, b := range bars {
		out = append(out, HoldingTSignalBar{
			Time:  "",
			High:  b.High,
			Low:   b.Low,
			Close: b.Close,
		})
		_ = i
	}
	return out
}

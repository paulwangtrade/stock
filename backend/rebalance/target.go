package rebalance

import (
	"math"
	"strings"
	"time"
)

const (
	SourceCurrentKeep       = "current_keep"
	SourceStrategyCandidate = "strategy_candidate"
	defaultCashBuffer       = 0.15 // aligns with ~0.85 gross headroom for observation target
	defaultGrossCap         = 0.85
)

// ObservationTargetOptions builds a read-only Target for Diff observation (not TradePlan).
type ObservationTargetOptions struct {
	// EnterSymbols are optional new names (e.g. from a pool). Empty → no ADD from enter.
	EnterSymbols []string
	// DropSymbols force symbols out of target (observation/tests). Empty → keep all current.
	DropSymbols []string
	// MaxNames caps target size; 0 → no cap beyond current+enter.
	MaxNames int
	// CashBufferWeight overrides default cash buffer (0 → use defaultCashBuffer).
	CashBufferWeight float64
	// GrossCap overrides default gross (0 → use defaultGrossCap).
	GrossCap float64
	// Identity when true: target weights = current weights (all KEEP if same set).
	Identity bool
}

// BuildObservationTarget constructs Target from Current (+ optional enter/drop).
// Equal-weight among keep∪enter by default; Identity copies current weights.
// Does not read DB, CandidatePool, or TradePlan.
func BuildObservationTarget(current *CurrentView, opts ObservationTargetOptions) *TargetPortfolio {
	out := &TargetPortfolio{
		Positions:        []TargetPosition{},
		ConstructionNote: "observation target · equal-weight keep∪enter; not a trade plan",
	}
	if current != nil {
		out.AsOf = current.AsOf
		out.AccountID = current.AccountID
		out.EquityRef = current.Equity
	}
	if out.AsOf.IsZero() {
		out.AsOf = time.Now()
	}
	if opts.Identity {
		out.ConstructionNote = "observation target · identity weights (expect KEEP)"
		if current == nil {
			return out
		}
		var sumW float64
		for _, p := range current.Positions {
			out.Positions = append(out.Positions, TargetPosition{
				Symbol:       p.Symbol,
				TargetWeight: p.Weight,
				TargetAmount: p.MarketValue,
				Source:       SourceCurrentKeep,
				Reason:       "identity current weight",
				Priority:     0,
			})
			sumW += p.Weight
		}
		out.CashBufferWeight = math.Max(0, 1-sumW)
		return out
	}

	buffer := opts.CashBufferWeight
	if buffer <= 0 {
		buffer = defaultCashBuffer
	}
	gross := opts.GrossCap
	if gross <= 0 {
		gross = defaultGrossCap
	}
	budget := math.Min(1-buffer, gross)
	out.CashBufferWeight = 1 - budget

	drop := map[string]struct{}{}
	for _, s := range opts.DropSymbols {
		drop[strings.TrimSpace(s)] = struct{}{}
	}

	type row struct {
		symbol string
		source string
		reason string
		prio   int
	}
	rows := []row{}
	if current != nil {
		for _, p := range current.Positions {
			code := strings.TrimSpace(p.Symbol)
			if code == "" {
				continue
			}
			if _, skip := drop[code]; skip {
				continue
			}
			rows = append(rows, row{symbol: code, source: SourceCurrentKeep, reason: "current keep", prio: 0})
		}
	}
	seen := map[string]struct{}{}
	for _, r := range rows {
		seen[r.symbol] = struct{}{}
	}
	prio := 1
	for _, s := range opts.EnterSymbols {
		code := strings.TrimSpace(s)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		if _, skip := drop[code]; skip {
			continue
		}
		rows = append(rows, row{symbol: code, source: SourceStrategyCandidate, reason: "observation enter", prio: prio})
		seen[code] = struct{}{}
		prio++
	}
	if opts.MaxNames > 0 && len(rows) > opts.MaxNames {
		// Prefer keeps (prio 0) then enters by order.
		rows = rows[:opts.MaxNames]
	}
	n := len(rows)
	if n == 0 || budget <= 0 || out.EquityRef < 0 {
		return out
	}
	w := budget / float64(n)
	for _, r := range rows {
		out.Positions = append(out.Positions, TargetPosition{
			Symbol:       r.symbol,
			TargetWeight: w,
			TargetAmount: w * out.EquityRef,
			Source:       r.source,
			Reason:       r.reason,
			Priority:     r.prio,
		})
	}
	return out
}

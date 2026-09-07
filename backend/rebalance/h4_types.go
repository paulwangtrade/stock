package rebalance

import (
	"time"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfoliorisk"
)

// H4 action deltas (observation suggestion — not Diff KEEP/ADD labels).
const (
	DeltaBuy    = "BUY"
	DeltaReduce = "REDUCE"
	DeltaExit   = "EXIT"

	SuggestSchemaVersion = "rebalance.h4-suggest-v1"
	PhaseSuggestOnly     = "suggest_only"

	ReasonUnderweight   = "UNDERWEIGHT"
	ReasonOverweight    = "OVERWEIGHT"
	ReasonNewName       = "NEW_NAME"
	ReasonTargetZero    = "TARGET_ZERO"
	ReasonOrphan        = "ORPHAN"
	ReasonBelowMinDelta = "BELOW_MIN_DELTA"
	ReasonBlockNew      = "BLOCK_NEW"
	ReasonCashBound     = "CASH_BOUND"
	ReasonGrossBound    = "GROSS_BOUND"
	ReasonSingleCap     = "SINGLE_CAP"
	ReasonRiskTighten   = "RISK_TIGHTEN"
	ReasonDataMissing   = "DATA_MISSING"
	ReasonAlreadyAtTarget = "ALREADY_AT_TARGET"

	OrphanExit = "exit_candidate"
	OrphanHold = "hold"

	SourcePortfolioAllocation = "portfolio_allocation"
	SourceSnapshot            = "snapshot"

	suggestDataNote = "Rebalance Calculate H4 · suggest_only; Current vs Target deltas; no TradePlan; no Execution; no Holdings mutation"
)

// CurrentPortfolio is the H4 book side (from Snapshot).
type CurrentPortfolio struct {
	Found     bool                     `json:"found"`
	AccountID uint                     `json:"account_id,omitempty"`
	Equity    float64                  `json:"equity"`
	Cash      float64                  `json:"cash"`
	Exposure  float64                  `json:"exposure"`
	Positions []CurrentPortfolioPos    `json:"positions"`
	Source    string                   `json:"source,omitempty"`
}

// CurrentPortfolioPos is one holding row.
type CurrentPortfolioPos struct {
	Symbol      string  `json:"symbol"`
	Qty         int64   `json:"qty"`
	MarketValue float64 `json:"market_value"`
	Weight      float64 `json:"weight"`
}

// CalculateOptions controls H4 suggest-only behaviour.
type CalculateOptions struct {
	// SuggestOnly is forced true inside Calculate regardless of input.
	SuggestOnly          bool
	MinDeltaWeight       float64 // default 0.005
	ReduceVsExitEpsilon  float64 // target weight ≤ ε → EXIT; default 1e-6
	OrphanPolicy         string  // exit_candidate | hold；default exit_candidate
	RiskTightenBuys      bool    // apply Resolved + PortfolioRisk to clip BUY
	MaxBuyNotional       float64 // optional extra cap; 0 = none
}

// DefaultCalculateOptions is production-safe suggest-only policy.
func DefaultCalculateOptions() CalculateOptions {
	return CalculateOptions{
		SuggestOnly:         true,
		MinDeltaWeight:      0.005,
		ReduceVsExitEpsilon: 1e-6,
		OrphanPolicy:        OrphanExit,
		RiskTightenBuys:     true,
	}
}

// RebalanceInput is the H4 engine input bundle.
type RebalanceInput struct {
	AsOf          time.Time
	TradeDate     string
	Current       CurrentPortfolio
	Target        TargetPortfolio // desired weights/amounts (e.g. from Allocation)
	PortfolioRisk *portfoliorisk.PortfolioRiskSnapshot
	Resolved      allocationengine.ResolvedConstraints
	Options       CalculateOptions
}

// BuyDelta is a suggested buy (not an order).
type BuyDelta struct {
	Symbol         string   `json:"symbol"`
	DeltaWeight    float64  `json:"delta_weight"`
	DeltaNotional  float64  `json:"delta_notional"`
	CurrentWeight  float64  `json:"current_weight"`
	TargetWeight   float64  `json:"target_weight"`
	Reason         string   `json:"reason"`
	ReasonCodes    []string `json:"reason_codes"`
	RiskBinding    string   `json:"risk_binding,omitempty"`
}

// ReduceDelta is a suggested partial trim (not an order).
type ReduceDelta struct {
	Symbol        string   `json:"symbol"`
	DeltaWeight   float64  `json:"delta_weight"` // positive = weight to shed
	DeltaNotional float64  `json:"delta_notional"`
	CurrentWeight float64  `json:"current_weight"`
	TargetWeight  float64  `json:"target_weight"`
	Reason        string   `json:"reason"`
	ReasonCodes   []string `json:"reason_codes"`
}

// ExitDelta is a suggested flatten-to-zero intent (not an order).
type ExitDelta struct {
	Symbol        string   `json:"symbol"`
	DeltaWeight   float64  `json:"delta_weight"`
	DeltaNotional float64  `json:"delta_notional"`
	CurrentWeight float64  `json:"current_weight"`
	Intent        string   `json:"intent"` // flatten_sellable
	Reason        string   `json:"reason"`
	ReasonCodes   []string `json:"reason_codes"`
}

// SkippedDelta records a symbol with no actionable delta.
type SkippedDelta struct {
	Symbol      string   `json:"symbol"`
	ReasonCodes []string `json:"reason_codes"`
}

// RiskImpact summarizes how risk/constraints changed suggestion size.
type RiskImpact struct {
	Applied              bool     `json:"applied"`
	BuyNotionalBeforeCut float64  `json:"buy_notional_before_cut"`
	BuyNotionalAfterCut  float64  `json:"buy_notional_after_cut"`
	BuyNotionalCut       float64  `json:"buy_notional_cut"`
	NamesBuyClipped      int      `json:"names_buy_clipped"`
	NamesBuyBlocked      int      `json:"names_buy_blocked"`
	EffectiveSingleCap   float64  `json:"effective_single_cap,omitempty"`
	GrossHeadroomUsed    *float64 `json:"gross_headroom_used,omitempty"`
	BlockNewEntries      bool     `json:"block_new_entries"`
	Notes                []string `json:"notes,omitempty"`
}

// RebalanceSuggestion is the H4 suggest-only output.
type RebalanceSuggestion struct {
	SchemaVersion      string         `json:"schema_version"`
	Phase              string         `json:"phase"`
	AsOf               string         `json:"as_of,omitempty"`
	TradeDate          string         `json:"trade_date,omitempty"`
	SuggestOnly        bool           `json:"suggest_only"`
	RecordOnly         bool           `json:"record_only"`
	NotTradePlan       bool           `json:"not_trade_plan"`
	NotExecution       bool           `json:"not_execution"`
	NotBuyChain        bool           `json:"not_buy_chain"`
	NotHoldingsMutate  bool           `json:"not_holdings_mutate"`
	Buys               []BuyDelta     `json:"buys"`
	Reduces            []ReduceDelta  `json:"reduces"`
	Exits              []ExitDelta    `json:"exits"`
	Skipped            []SkippedDelta `json:"skipped,omitempty"`
	Reason             string         `json:"reason"` // portfolio-level summary reason
	RiskImpact         RiskImpact     `json:"risk_impact"`
	Summary            SuggestSummary `json:"summary"`
	InputsFingerprint  string         `json:"inputs_fingerprint"`
	DataSourceNote     string         `json:"data_source_note"`
	Notes              []string       `json:"notes,omitempty"`
}

// SuggestSummary aggregates notionals / counts.
type SuggestSummary struct {
	BuyNotional    float64 `json:"buy_notional"`
	ReduceNotional float64 `json:"reduce_notional"`
	ExitNotional   float64 `json:"exit_notional"`
	BuyCount       int     `json:"buy_count"`
	ReduceCount    int     `json:"reduce_count"`
	ExitCount      int     `json:"exit_count"`
}

// rawBuy is an internal pre-risk-cut buy candidate.
type rawBuy struct {
	sym    string
	dw     float64
	notion float64
	cw, tw float64
	codes  []string
	reason string
	isNew  bool
}

// Package portfolioselectorshadow compares Legacy Selection vs PortfolioSelector
// in a read-only Shadow pipeline (Phase13).
//
// DefaultEnabled is false. It never writes TradePlan, never imports Execution /
// Broker / Risk PlanFilter, and never mutates Controlled adoption policy.
package portfolioselectorshadow

import "time"

const (
	SchemaVersion = "portfolio_selector.shadow.v1"

	SkipReasonDisabled = "portfolio_selector_shadow_disabled"

	IncomparableSelectorFailed = "selector_failed"
	IncomparableLegacyFailed   = "legacy_failed"
	IncomparableBothFailed     = "both_failed"
	IncomparableMissingSnap    = "missing_snapshot"

	PresenceBoth          = "both"
	PresenceOnlyLegacy    = "only_legacy"
	PresenceOnlyPortfolio = "only_portfolio"

	AmountEpsilon = 0.01
)

// DefaultEnabled is the compile-time default. Production stays false.
const DefaultEnabled = false

// PoolItemView is a read-only pool row (Shadow input; not a TradePlan item).
type PoolItemView struct {
	StockCode string  `json:"stock_code"`
	StockName string  `json:"stock_name"`
	Industry  string  `json:"industry"`
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
}

// PositionView is an injected holding.
type PositionView struct {
	StockCode   string  `json:"stock_code"`
	MarketValue float64 `json:"market_value"`
	Industry    string  `json:"industry"`
}

// SnapshotView is injected book state (Shadow does not query DB).
type SnapshotView struct {
	Found       bool           `json:"found"`
	Cash        float64        `json:"cash"`
	Equity      float64        `json:"equity"`
	MarketValue float64        `json:"market_value"`
	Positions   []PositionView `json:"positions"`
}

// Input feeds one Shadow observation.
type Input struct {
	TradeDate      string
	PoolID         uint
	PoolItems      []PoolItemView
	Snapshot       *SnapshotView
	LegacyAmount   float64
	LegacyMaxNames int
	// Optional: precomputed Legacy Selection (PrimaryPicks face). If nil, Shadow runs selection.Select.
	LegacySelection any // reserved; unused — always built from PoolItems via selection.Select
	Trigger         string
	RunID           string
	DecisionTime    time.Time
}

// Config constructs a Runtime. Enabled defaults to DefaultEnabled (false).
type Config struct {
	Enabled bool
	Now     func() time.Time
}

// LegacySelectedName is one PrimaryPicks row with fixed-amount projection.
type LegacySelectedName struct {
	StockCode string  `json:"stock_code"`
	StockName string  `json:"stock_name"`
	Industry  string  `json:"industry"`
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	Amount    float64 `json:"amount"`
}

// LegacyAnnotate is an E.6 annotate reject (not PlanFilter).
type LegacyAnnotate struct {
	StockCode string `json:"stock_code"`
	Reason    string `json:"reason"`
}

// LegacySelectionView is the Legacy observation face.
type LegacySelectionView struct {
	RankedCount      int                  `json:"ranked_count"`
	SelectionLimit   int                  `json:"selection_limit"`
	Selected         []LegacySelectedName `json:"selected"`
	Waitlist         []string             `json:"waitlist"`
	RejectedAnnotate []LegacyAnnotate     `json:"rejected_annotate"`
	UniformAmount    float64              `json:"uniform_amount"`
	AllocatedTotal   float64              `json:"allocated_total"`
}

// PortfolioSelectedRef is a compact portfolio face line.
type PortfolioSelectedRef struct {
	StockCode        string  `json:"stock_code"`
	Amount           float64 `json:"amount"`
	Weight           float64 `json:"weight"`
	Industry         string  `json:"industry"`
	AllocationReason string  `json:"allocation_reason"`
}

// PortfolioRejectedRef is a construction reject.
type PortfolioRejectedRef struct {
	StockCode    string `json:"stock_code"`
	RejectReason string `json:"reject_reason"`
}

// PortfolioFaceRef summarizes PortfolioSelector output.
type PortfolioFaceRef struct {
	SelectedCount int                    `json:"selected_count"`
	RejectedCount int                    `json:"rejected_count"`
	TotalAmount   float64                `json:"total_amount"`
	Method        string                 `json:"method"`
	Selected      []PortfolioSelectedRef `json:"selected"`
	Rejected      []PortfolioRejectedRef `json:"rejected"`
	SectorWeights map[string]float64     `json:"sector_weights"`
	Binding       string                 `json:"binding"`
}

// AllocationNameDiff is one symbol allocation comparison.
type AllocationNameDiff struct {
	StockCode                 string  `json:"stock_code"`
	Presence                  string  `json:"presence"`
	LegacyAmount              float64 `json:"legacy_amount"`
	PortfolioAmount           float64 `json:"portfolio_amount"`
	AmountDelta               float64 `json:"amount_delta"`
	LegacyWeight              float64 `json:"legacy_weight"`
	PortfolioWeight           float64 `json:"portfolio_weight"`
	WeightDelta               float64 `json:"weight_delta"`
	PortfolioAllocationReason string  `json:"portfolio_allocation_reason,omitempty"`
}

// AllocationDiff compares total and per-name amounts.
type AllocationDiff struct {
	LegacyMethod         string               `json:"legacy_method"`
	PortfolioMethod      string               `json:"portfolio_method"`
	LegacyTotalAmount    float64              `json:"legacy_total_amount"`
	PortfolioTotalAmount float64              `json:"portfolio_total_amount"`
	TotalAmountDelta     float64              `json:"total_amount_delta"`
	PerName              []AllocationNameDiff `json:"per_name"`
	AmountDeltaNameCount int                  `json:"amount_delta_name_count"`
}

// ReasonCount is a histogram bucket.
type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// RejectedReasonSummary aggregates construction / annotate rejects.
type RejectedReasonSummary struct {
	PortfolioRejectHistogram []ReasonCount `json:"portfolio_reject_histogram"`
	LegacyAnnotateHistogram  []ReasonCount `json:"legacy_annotate_histogram"`
	ConstructionRejectNote   string        `json:"construction_reject_note"`
}

// SectorConcentrationDiff is a read-only sector weight projection diff.
type SectorConcentrationDiff struct {
	EquityBase                 float64            `json:"equity_base"`
	BaselineSectorWeights      map[string]float64 `json:"baseline_sector_weights"`
	LegacyPostSectorWeights    map[string]float64 `json:"legacy_post_sector_weights"`
	PortfolioPostSectorWeights map[string]float64 `json:"portfolio_post_sector_weights"`
	LegacyMaxSectorWeight      float64            `json:"legacy_max_sector_weight"`
	PortfolioMaxSectorWeight   float64            `json:"portfolio_max_sector_weight"`
	MaxSectorDelta             float64            `json:"max_sector_delta"`
	Degraded                   bool               `json:"degraded"`
	DegradedReason             string             `json:"degraded_reason,omitempty"`
}

// ShadowFailure records a closed observation failure.
type ShadowFailure struct {
	Side        string `json:"side,omitempty"`
	ErrorReason string `json:"error_reason,omitempty"`
}

// Report is the Shadow output (not a TradePlan).
type Report struct {
	SchemaVersion      string    `json:"schema_version"`
	Enabled            bool      `json:"enabled"`
	Skipped            bool      `json:"skipped"`
	SkipReason         string    `json:"skip_reason,omitempty"`
	RunID              string    `json:"run_id,omitempty"`
	RecordedAt         time.Time `json:"recorded_at"`
	TradeDate          string    `json:"trade_date,omitempty"`
	PoolID             uint      `json:"pool_id,omitempty"`
	Trigger            string    `json:"trigger,omitempty"`
	Comparable         bool      `json:"comparable"`
	IncomparableReason string    `json:"incomparable_reason,omitempty"`

	LegacySelectedCount    int `json:"legacy_selected_count"`
	PortfolioSelectedCount int `json:"portfolio_selected_count"`

	AllocationDiff          AllocationDiff          `json:"allocation_diff"`
	SectorConcentrationDiff SectorConcentrationDiff `json:"sector_concentration_diff"`
	RejectedReasonSummary   RejectedReasonSummary   `json:"rejected_reason_summary"`
	DataGaps                []string                `json:"data_gaps"`

	Legacy    LegacySelectionView `json:"legacy"`
	Portfolio PortfolioFaceRef    `json:"portfolio"`
	Failure   ShadowFailure       `json:"failure"`
}

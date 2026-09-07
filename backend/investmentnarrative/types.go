// Package investmentnarrative projects read-only Investment Narrative views (Phase14-G0.5a).
// No new fact tables; joins existing signal snapshots, trade plans, fills, positions, exit eval.
package investmentnarrative

import "time"

const (
	SchemaVersion      = "investment_narrative.v1"
	SectionStatusFull    = "full"
	SectionStatusPartial = "partial"
	SectionStatusMissing = "missing"
	SectionStatusNA      = "not_applicable"

	dataSourceNote = "Investment Narrative · read-only projection; no new facts; not investment advice"
)

// InvestmentNarrative is the top-level read model for one stock.
type InvestmentNarrative struct {
	StockCode      string                `json:"stock_code"`
	Discovery      DiscoveryNarrative    `json:"discovery"`
	PlanOrigin     PlanOriginNarrative   `json:"plan_origin"`
	HoldingBasis   HoldingBasisNarrative `json:"holding_basis"`
	ExitReview     ExitReviewNarrative   `json:"exit_review"`
	PriceStory     PriceStoryNarrative   `json:"price_story"`
	AsOf           time.Time             `json:"as_of"`
	SchemaVersion  string                `json:"schema_version"`
	DataSourceNote string                `json:"data_source_note,omitempty"`
}

type DiscoveryNarrative struct {
	Exists      bool     `json:"exists"`
	Status      string   `json:"status"`
	SignalTime  string   `json:"signal_time,omitempty"`
	SignalPrice *float64 `json:"signal_price,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	SnapshotID  uint     `json:"snapshot_id,omitempty"`
}

type PlanOriginNarrative struct {
	Exists   bool   `json:"exists"`
	Status   string `json:"status"`
	PlanID   uint   `json:"plan_id,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type HoldingBasisNarrative struct {
	Exists   bool    `json:"exists"`
	Status   string  `json:"status"`
	Quantity int64   `json:"quantity,omitempty"`
	AvgCost  float64 `json:"avg_cost,omitempty"`
}

type ExitReviewOutcomeNarrative struct {
	Decision           string     `json:"decision,omitempty"`
	ReviewTime         *time.Time `json:"review_time,omitempty"`
	Reason             string     `json:"reason,omitempty"`
	RelatedTradePlanID *uint      `json:"related_trade_plan_id,omitempty"`
}

type ExitReviewNarrative struct {
	Exists        bool                        `json:"exists"`
	Status        string                      `json:"status"`
	LatestOutcome *ExitReviewOutcomeNarrative `json:"latest_outcome,omitempty"`
	ReasonCodes   []string                    `json:"reason_codes,omitempty"`
	Summary       string                      `json:"summary,omitempty"`
}

type PriceStoryNarrative struct {
	Status        string   `json:"status"`
	SignalPrice   *float64 `json:"signal_price,omitempty"`
	CurrentPrice  *float64 `json:"current_price,omitempty"`
	VsSignalPct   *float64 `json:"vs_signal_pct,omitempty"`
}

// BuildOptions configures the narrative projector.
type BuildOptions struct {
	StockCode string
	AccountID uint
	AsOf      time.Time
}

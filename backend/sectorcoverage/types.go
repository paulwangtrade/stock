// Package sectorcoverage is the Sector Coverage Audit (Phase13).
//
// It measures stock-pool and holdings classification coverage via sectorprovider,
// optionally cross-checks portfoliorisk sector availability, and fail-closes
// AllowSectorConstraint when coverage is insufficient.
//
// Read-only. Does not wire TradePlan / Execution / buy-sell write chains.
package sectorcoverage

import (
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/sectorprovider"
)

const (
	SchemaVersion = "sector_coverage_audit.p13-v1"

	// DefaultMinHoldingsCoverage requires full holdings classification (fail-closed).
	DefaultMinHoldingsCoverage = sectorprovider.DefaultMinHoldingsCoverage

	OutcomeResolved = "resolved"
	OutcomeMissing  = "missing"
	OutcomeUnknown  = "unknown"

	NoteOK                    = "holdings_fully_classified"
	NoteIncomplete            = "holdings_coverage_incomplete"
	NoteUnknownPresent        = "unknown_labels_present"
	NoteEmptyHoldings         = "empty_holdings_cannot_enable_sector_constraint"
	NoteProviderNil           = "provider_nil"
	NoteRiskSectorUnavailable = "portfoliorisk_sector_unavailable"
)

// AuditInput scopes the audit universe.
type AuditInput struct {
	AsOf time.Time

	// Holdings symbols (gate for AllowSectorConstraint).
	Holdings []string
	// Pool / Candidates symbols (股票池；informational coverage).
	Pool []string

	// MinHoldingsCoverage defaults to 1.0. Coverage below → constraint off.
	MinHoldingsCoverage float64

	// Optional: when set, also verify portfoliorisk.Build sector.available.
	Snapshot    *portfolio.Snapshot
	Constraints *portfoliolayer.ConstraintSet
	// AttachPortfolioRisk runs Build and ANDs sector.available into the gate.
	AttachPortfolioRisk bool
}

// ClassGap is one symbol lacking a usable industry/sector label.
type ClassGap struct {
	Symbol  string `json:"symbol"`
	Outcome string `json:"outcome"` // missing | unknown
	Bucket  string `json:"bucket"`  // holdings | pool
}

// SectorCoverageReport is the audit output.
type SectorCoverageReport struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of,omitempty"`
	Taxonomy      string    `json:"taxonomy,omitempty"`
	Source        string    `json:"source,omitempty"`
	Version       string    `json:"version,omitempty"`

	// 股票池覆盖率
	PoolRequested int      `json:"pool_requested"`
	PoolResolved  int      `json:"pool_resolved"`
	PoolCoverage  float64  `json:"pool_coverage"` // resolved/requested; 0 if requested=0
	PoolMissing   []string `json:"pool_missing,omitempty"`

	// 持仓覆盖率
	HoldingsRequested int      `json:"holdings_requested"`
	HoldingsResolved  int      `json:"holdings_resolved"`
	HoldingsCoverage  float64  `json:"holdings_coverage"`
	HoldingsMissing   []string `json:"holdings_missing,omitempty"`
	HoldingsComplete  bool     `json:"holdings_complete"`

	// 行业分类缺失（持仓∪池，去重）
	IndustryClassificationMissing []string   `json:"industry_classification_missing,omitempty"`
	ClassificationGaps            []ClassGap `json:"classification_gaps,omitempty"`

	// unknown 数量（sentinel 标签：unknown / 0 / …）
	UnknownCount int `json:"unknown_count"`

	MinCoverageRequired   float64 `json:"min_coverage_required"`
	AllowSectorConstraint bool    `json:"allow_sector_constraint"`
	Note                  string  `json:"note,omitempty"`

	// Optional portfoliorisk cross-check
	PortfolioRiskAttached        bool   `json:"portfolio_risk_attached,omitempty"`
	PortfolioRiskSectorAvailable *bool  `json:"portfolio_risk_sector_available,omitempty"`
	PortfolioRiskSectorNote      string `json:"portfolio_risk_sector_note,omitempty"`

	RecordOnly       bool `json:"record_only"`
	NotTradePlan     bool `json:"not_trade_plan"`
	NotExecution     bool `json:"not_execution"`
	NotBuyChainWrite bool `json:"not_buy_chain_write"`
}

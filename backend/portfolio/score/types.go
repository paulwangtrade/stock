// Package score builds a read-only Investment Quality Score (Phase11-F.2.1).
//
// Maps CandidatePool + holdings into a shared 0–100 scale. Missing factors stay nil
// (no invented neutral scores). Does not write DB or change trading paths.
package score

import "time"

const (
	SubjectCandidate = "CANDIDATE"
	SubjectHolding   = "HOLDING"
	SubjectBoth      = "BOTH"

	QualityOK       = "OK"
	QualityDegraded = "DEGRADED"
	QualityNotFound = "NOT_FOUND"

	dataSourceNote = "Investment Score F.2.1 · CandidatePool + Snapshot/Intelligence projection; no fake fills; read-only"
	disclaimer     = "投资质量分（规则映射）。非投资建议，不触发买卖，不修改交易状态。"
)

// InvestmentScoreView is the HTTP / service read model.
type InvestmentScoreView struct {
	StockCode          string   `json:"stock_code"`
	StockName          string   `json:"stock_name,omitempty"`
	SubjectType        string   `json:"subject_type"`
	TradeDate          string   `json:"trade_date,omitempty"`
	AsOf               time.Time `json:"as_of"`
	TotalScore         *float64 `json:"total_score"`
	StrategyScore      *float64 `json:"strategy_score"`
	RiskScore          *float64 `json:"risk_score"`
	MomentumScore      *float64 `json:"momentum_score"`
	PortfolioFitScore  *float64 `json:"portfolio_fit_score"`
	Quality            string   `json:"quality"`
	MissingFactors     []string `json:"missing_factors,omitempty"`
	Found              bool     `json:"found"`
	DataSourceNote     string   `json:"data_source_note"`
	Disclaimer         string   `json:"disclaimer"`
}

// CandidateInput is one pool row (optional).
type CandidateInput struct {
	Present          bool
	StockName        string
	Score            float64 // typically 0–1 composite
	SignalScore      float64
	HasSignal        bool // true when signal fields indicate a real observation
	Rank             int
	PoolItemCount    int
	TradePlanRiskCode string // optional; non-empty → real risk signal
}

// HoldingInput is one book row + intelligence labels (optional).
type HoldingInput struct {
	Present         bool
	StockName       string
	Weight          float64 // 0–1
	UnrealizedReturn *float64 // (mark-cost)/cost when cost>0
	RiskLevel       string
	PositionStatus  string
	StrategyStatus  string
}

// Inputs for pure Build.
type Inputs struct {
	StockCode string
	TradeDate string
	AsOf      time.Time
	Candidate CandidateInput
	Holding   HoldingInput
}

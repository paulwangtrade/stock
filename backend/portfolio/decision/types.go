// Package decision builds a read-only Portfolio Decision Summary (Phase11-G.3).
//
// Aggregates health / opportunity / capital efficiency / risk into HOLD|REVIEW|WATCH.
// Never emits BUY/SELL. Does not write DB or touch trading paths.
// Position new/old labels come from portfolio/positionstate (consumers must not re-derive from available_qty==0).
package decision

import "time"

const (
	AttentionHold   = "HOLD"
	AttentionReview = "REVIEW"
	AttentionWatch  = "WATCH"

	QualityOK       = "OK"
	QualityDegraded = "DEGRADED"

	EffLow    = "LOW"
	EffMedium = "MEDIUM"
	EffHigh   = "HIGH"

	dataSourceNote = "Portfolio Decision Summary G.3 · Dashboard/Snapshot + Intelligence + Score + PreTrade + inline opportunity/efficiency heuristics; read-only"
	disclaimer     = "组合决策摘要（只读关注提示）。禁止视为买卖或自动调仓指令。系统仅提示关注，不自动买卖或调仓。"
)

// PortfolioDecisionSummary is the G.3 read model.
type PortfolioDecisionSummary struct {
	TradeDate                   string                      `json:"trade_date"`
	AsOf                        time.Time                   `json:"as_of"`
	OverallAttention            string                      `json:"overall_attention"`
	PortfolioHealth             PortfolioHealth             `json:"portfolio_health"`
	OpportunityAttention        OpportunityAttention        `json:"opportunity_attention"`
	CapitalEfficiencyAttention  CapitalEfficiencyAttention  `json:"capital_efficiency_attention"`
	RiskAttention               RiskAttention               `json:"risk_attention"`
	Explanation                 string                      `json:"explanation"`
	Quality                     string                      `json:"quality"`
	MissingInputs               []string                    `json:"missing_inputs,omitempty"`
	DataSourceNote              string                      `json:"data_source_note"`
	Disclaimer                  string                      `json:"disclaimer"`
}

// PortfolioHealth is overall book health.
type PortfolioHealth struct {
	Status        string   `json:"status"`
	Equity        float64  `json:"equity"`
	Cash          float64  `json:"cash"`
	PositionCount int      `json:"position_count"`
	Found         bool     `json:"found"`
	Notes         []string `json:"notes,omitempty"`
}

// OpportunityAttention compares candidate vs holdings (G.1-style heuristic).
type OpportunityAttention struct {
	Status         string                `json:"status"`
	CandidateCode  string                `json:"candidate_code,omitempty"`
	CandidateName  string                `json:"candidate_name,omitempty"`
	CandidateScore *float64              `json:"candidate_score"`
	Highlights     []OpportunityHighlight `json:"highlights"`
	SourceAction   string                `json:"source_action,omitempty"` // HOLD|REVIEW|POSSIBLE_REPLACE (audit only)
	UserAction     string                `json:"user_action"`             // HOLD|REVIEW|WATCH only
}

// OpportunityHighlight is one holding compared to candidate.
type OpportunityHighlight struct {
	HoldingCode  string   `json:"holding_code"`
	HoldingScore *float64 `json:"holding_score"`
	Reason       string   `json:"reason"`
}

// CapitalEfficiencyAttention lists low-efficiency names (G.2-style heuristic).
type CapitalEfficiencyAttention struct {
	Status             string             `json:"status"`
	LowEfficiencyCount int                `json:"low_efficiency_count"`
	Items              []EfficiencyItem   `json:"items"`
}

// EfficiencyItem is one holding efficiency row.
type EfficiencyItem struct {
	StockCode        string   `json:"stock_code"`
	CapitalUsed      float64  `json:"capital_used"`
	PortfolioWeight  float64  `json:"portfolio_weight,omitempty"`
	InvestmentScore  *float64 `json:"investment_score"`
	EfficiencyScore  *float64 `json:"efficiency_score"`
	EfficiencyLevel  string   `json:"efficiency_level,omitempty"`
}

// RiskAttention merges PreTrade + Intelligence.
type RiskAttention struct {
	Status             string              `json:"status"`
	PretradeLevel      string              `json:"pretrade_level,omitempty"`
	CashEnough         *bool               `json:"cash_enough,omitempty"`
	Concentration      string              `json:"concentration,omitempty"`
	IntelligenceItems  []RiskIntelItem     `json:"intelligence_items"`
}

// RiskIntelItem is one intelligence attention row.
type RiskIntelItem struct {
	StockCode        string `json:"stock_code"`
	PositionStatus   string `json:"position_status,omitempty"`
	RiskLevel        string `json:"risk_level,omitempty"`
	AttentionReason  string `json:"attention_reason,omitempty"`
}

// --- Build inputs ---

// HoldingRow is one book name with optional scores.
type HoldingRow struct {
	StockCode        string
	StockName        string
	CapitalUsed      float64
	Weight           float64
	InvestmentScore  *float64
	PositionStatus   string
	RiskLevel        string
	AttentionReason  string
}

// CandidateRow is top opportunity (optional).
type CandidateRow struct {
	Present bool
	Code    string
	Name    string
	Score   *float64 // 0–100 investment total when known
}

// PreTradeInput is optional F.1 snapshot.
type PreTradeInput struct {
	Present       bool
	Level         string // PASS|WARNING|BLOCKED
	CashEnough    bool
	Concentration string
}

// Inputs for pure Build (tests inject; Service loads).
type Inputs struct {
	TradeDate   string
	AsOf        time.Time
	AccountFound bool
	Equity      float64
	Cash        float64
	Holdings    []HoldingRow
	Candidate   CandidateRow
	PreTrade    PreTradeInput
	// MissingOpportunityPackage / MissingEfficiencyPackage mark design-only deps.
	MissingOpportunityPackage bool
	MissingEfficiencyPackage  bool
}

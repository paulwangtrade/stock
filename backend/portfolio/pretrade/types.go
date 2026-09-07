// Package pretrade builds a read-only PreTrade Risk Report (Phase11-F.1).
//
// It explains cash / existing position / concentration before execution.
// It does NOT block Gateway, write TradePlan, or change Risk engine rules.
package pretrade

import "time"

// Overall level (observation only — F.1 does not gate execution).
const (
	LevelPass    = "PASS"
	LevelWarning = "WARNING"
	LevelBlocked = "BLOCKED"
)

// Check item status values.
const (
	StatusPass    = "PASS"
	StatusWarning = "WARNING"
	StatusBlocked = "BLOCKED"
)

// Position action labels for planned BUY lines (not trade instructions).
const (
	ActionNew  = "NEW"
	ActionAdd  = "ADD"
	ActionHold = "HOLD"
)

// Concentration labels (aligned with tradingplan/readiness).
const (
	ConcentrationNormal   = "NORMAL"
	ConcentrationElevated = "ELEVATED"
	ConcentrationHigh     = "HIGH"
)

const (
	dataSourceNote = "PreTrade Risk Check F.1 · read-only plan + portfolio snapshot; observation only; does not block Gateway"
	disclaimer     = "执行前风险解释（只读）。不阻断成交，不修改交易状态，非投资建议。"
)

// PreTradeRiskResult is the F.1 read model.
type PreTradeRiskResult struct {
	PlanID                 uint                     `json:"plan_id"`
	TradeDate              string                   `json:"trade_date"`
	AsOf                   time.Time                `json:"as_of"`
	Allowed                bool                     `json:"allowed"` // informational; F.1 never enforces
	Level                  string                   `json:"level"`
	RequiresConfirm        bool                     `json:"requires_confirm"`
	Summary                string                   `json:"summary"`
	CashCheck              CashCheck                `json:"cash_check"`
	ExistingPositionCheck  ExistingPositionCheck    `json:"existing_position_check"`
	ConcentrationCheck     ConcentrationCheck       `json:"concentration_check"`
	RiskSnapshot           RiskSnapshot             `json:"risk_snapshot"`
	PositionActions        []PositionAction         `json:"position_action"`
	DataSourceNote         string                   `json:"data_source_note"`
	Disclaimer             string                   `json:"disclaimer"`
}

// CashCheck is plan notional vs available cash.
type CashCheck struct {
	Status        string  `json:"status"`
	Reason        string  `json:"reason,omitempty"`
	AvailableCash float64 `json:"available_cash"`
	RequiredCash  float64 `json:"required_cash"`
	CashEnough    bool    `json:"cash_enough"`
}

// ExistingPositionCheck summarizes NEW vs ADD across plan lines.
type ExistingPositionCheck struct {
	Status       string   `json:"status"` // PASS | WARNING
	Reason       string   `json:"reason,omitempty"`
	AddCount     int      `json:"add_count"`
	NewCount     int      `json:"new_count"`
	ConflictCodes []string `json:"conflict_codes,omitempty"`
}

// ConcentrationCheck projects post-buy single-name weight.
type ConcentrationCheck struct {
	Status        string  `json:"status"` // PASS | WARNING
	Reason        string  `json:"reason,omitempty"`
	Level         string  `json:"level"` // NORMAL | ELEVATED | HIGH
	MaxWeightPct  float64 `json:"max_weight_pct"`
	IndustryNote  string  `json:"industry_note,omitempty"`
}

// RiskSnapshot is plan + config observation (not a live RiskDecision).
type RiskSnapshot struct {
	PlanRiskStatus   string  `json:"plan_risk_status,omitempty"`
	MarketLevel      int     `json:"market_level,omitempty"`
	RiskSummary      string  `json:"risk_summary,omitempty"`
	MaxSingleNamePct float64 `json:"max_single_name_pct,omitempty"`
	MaxGrossExposurePct float64 `json:"max_gross_exposure_pct,omitempty"`
	RiskFilterEnabled bool   `json:"risk_filter_enabled,omitempty"`
}

// PositionAction is one planned name vs current book.
type PositionAction struct {
	StockCode       string  `json:"stock_code"`
	StockName       string  `json:"stock_name,omitempty"`
	Action          string  `json:"action"` // NEW | ADD | HOLD
	ExistingVolume  int64   `json:"existing_volume,omitempty"`
	PlannedAmount   float64 `json:"planned_amount,omitempty"`
	PlannedVolume   int64   `json:"planned_volume,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}

// PlanLine is a minimal executable BUY line for evaluation.
type PlanLine struct {
	StockCode    string
	StockName    string
	Side         string
	TargetAmount float64
	TargetVolume int64
	LimitPrice   float64
	RefPrice     float64
	OpenRefPrice float64
	Status       string
	IntentStatus string
}

// AccountInput is cash/equity context.
type AccountInput struct {
	Cash   float64
	Equity float64
}

// PositionInput is an existing holding.
type PositionInput struct {
	StockCode   string
	StockName   string
	TotalVolume int64
}

// Inputs for pure Build (tests inject; Service loads).
type Inputs struct {
	PlanID       uint
	TradeDate    string
	AsOf         time.Time
	Lines        []PlanLine
	Account      AccountInput
	Positions    []PositionInput
	RiskSnapshot RiskSnapshot
}

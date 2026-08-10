package qualitygate

import "time"

const (
	RuleE1 = "QG-E1"
	RuleI1 = "QG-I1"
	RuleG1 = "QG-G1"
	RuleM1 = "QG-M1"
	RuleP1 = "QG-P1"

	CodeEntryPriceMissing   = "ENTRY_PRICE_MISSING"
	CodePositionConflict    = "POSITION_CONFLICT"
	CodeSectorConcentration = "SECTOR_CONCENTRATION"
	CodeOpenGapPending      = "OPEN_GAP_PENDING"
	CodePlanCompleteness    = "PLAN_COMPLETENESS"

	SeverityPASS = "PASS"
	SeverityWARN = "WARN"
	SeverityFAIL = "FAIL"

	BehaviorNone    = "none"
	BehaviorWarning = "warning"
	BehaviorBlock   = "block"
)

// AccountPosition is a read-only holding snapshot for conflict checks.
type AccountPosition struct {
	StockCode string  `json:"stockCode"`
	Volume    float64 `json:"volume"`
	StockName string  `json:"stockName,omitempty"`
}

// MarketDataSnapshot supplies optional enrich + open prices (injected; no live fetch).
type MarketDataSnapshot struct {
	IndustryByCode    map[string]string  `json:"industryByCode,omitempty"`
	NameByCode        map[string]string  `json:"nameByCode,omitempty"`
	AnchorPriceByCode map[string]float64 `json:"anchorPriceByCode,omitempty"`
	OpenPriceByCode   map[string]float64 `json:"openPriceByCode,omitempty"`
	SkipGapEval       bool               `json:"skipGapEval,omitempty"`
}

// Input is the MVP evaluation input.
type Input struct {
	Plan       PlanView
	Positions  []AccountPosition
	MarketData MarketDataSnapshot
	Config     Config
}

// PlanView is a minimal TradePlan projection (avoids DB coupling in unit tests).
type PlanView struct {
	ID             uint
	TradeDate      string
	PlanVersion    int
	Status         string
	AmountPerStock float64
	Items          []ItemView
}

// ItemView is a minimal TradePlanItem projection.
type ItemView struct {
	StockCode    string
	StockName    string
	Side         string
	Priority     int
	TargetAmount float64
	LimitPrice   float64
	TargetVolume int64
	Industry     string
}

// Config holds MVP thresholds.
type Config struct {
	SectorWarnAmountShare float64 // default 0.40
	SectorWarnNameCount   int     // default 2; warn when count > this
	MinCompletenessRatio  float64 // default 0.80
	RequireEntryPrice     bool
}

// DefaultConfig returns MVP defaults from PHASE6_5_6_2 plan.
func DefaultConfig() Config {
	return Config{
		SectorWarnAmountShare: 0.40,
		SectorWarnNameCount:   2,
		MinCompletenessRatio:  0.80,
		RequireEntryPrice:     true,
	}
}

// Finding is one rule result line.
type Finding struct {
	Passed   bool           `json:"passed"`
	Severity string         `json:"severity"`
	RuleCode string         `json:"rule_code"`
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

// Result is the aggregate QualityGate outcome.
type Result struct {
	Passed    bool      `json:"passed"`
	Severity  string    `json:"severity"`
	PlanID    uint      `json:"plan_id,omitempty"`
	TradeDate string    `json:"trade_date,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	Findings  []Finding `json:"findings"`
	Blockers  []Finding `json:"blockers,omitempty"`
	Warnings  []Finding `json:"warnings,omitempty"`
}

// QualityGateResult is the public name for Result (PHASE6_5_6_3 contract).
type QualityGateResult = Result

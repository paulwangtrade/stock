// Package riskreport provides a read-only Advanced Risk Report aggregator.
//
// Phase13-B MVP: Portfolio / Position / Execution / Market → RiskReport.
// Does not alter trading execution (Broker / Gateway / Fill / Settlement / TradePlan writes).
package riskreport

import "time"

// SchemaVersion is the RiskReport contract (Phase12-B).
const SchemaVersion = "riskrpt-1"

const (
	StatusOK       = "ok"
	StatusGated    = "gated"
	StatusDegraded = "degraded"
	StatusFailed   = "failed"
)

const (
	DimPortfolio  = "portfolio"
	DimPosition   = "position"
	DimExecution  = "execution"
	DimMarket     = "market"
)

const (
	SeverityInfo = "info"
	SeverityWarn = "warn"
	SeverityHigh = "high"
)

const (
	BandLow     = "low"
	BandMedium  = "medium"
	BandHigh    = "high"
	BandUnknown = "unknown"
)

const (
	SuggestReview  = "review"
	SuggestMonitor = "monitor"
	SuggestInform  = "inform"
)

// BuildRequest configures a report build.
type BuildRequest struct {
	UserID    string
	Tier      string
	TradeDate string
	Now       time.Time
	// Sources when set skips live fetch (tests / preloaded Observation).
	Sources *ReportSources
}

// ReportSources is the read-only input bundle (no trading writes).
type ReportSources struct {
	PortfolioEnabled bool
	PortfolioQuality string // OK | UNKNOWN
	Cash             *float64
	PositionRatio    *float64
	Concentration    *float64 // max single-name weight
	PositionSymbols  []string

	HoldingRiskStates map[string]int // NORMAL|WATCH|DANGER → count
	HoldingSymbols    map[string][]string // state → symbols

	ExitReasonCounts map[string]int // TIME_REVIEW|LOSS_REVIEW|PLAN_REVIEW → count
	ExitReviewStates map[string]int // NORMAL|WATCH|REVIEW_REQUIRED → count

	ExecEnabled     bool
	ExecTotalOrders int
	ExecFilled      int
	ExecFailed      int
	ExecFillRate    float64
	ExecDataNote    string
	ExecDataSource  string // paper_sim | paper_orders_fallback | …

	MarketLevel   int
	MarketState   string // PRE_OPEN|OPEN|…|CLOSED
	MarketTrading bool
}

// RiskReport is the aggregated read-only advanced risk view.
type RiskReport struct {
	ReportID      string    `json:"report_id"`
	SchemaVersion string    `json:"schema_version"`
	UserID        string    `json:"user_id,omitempty"`
	TradeDate     string    `json:"trade_date,omitempty"`
	AccountScope  string    `json:"account_scope,omitempty"`
	GeneratedAt   time.Time `json:"generated_at"`
	Status        string    `json:"status"`
	GateReason    string    `json:"gate_reason,omitempty"`

	Score       RiskScore         `json:"score"`
	Factors     []RiskFactor      `json:"factors"`
	Warnings    []RiskWarning     `json:"warnings"`
	Suggestions []RiskSuggestion  `json:"suggestions"`

	Dimensions  DimensionsView `json:"dimensions"`
	Sources     []SourceRef    `json:"sources,omitempty"`
	DataQuality string         `json:"data_quality"` // OK | PARTIAL | UNKNOWN
	Disclaimers []string       `json:"disclaimers"`
}

// RiskScore is 0–100 (higher = more risk).
type RiskScore struct {
	Overall      int            `json:"overall"`
	Band         string         `json:"band"`
	ByDimension  map[string]int `json:"by_dimension"`
}

// RiskFactor is one machine-stable risk signal.
type RiskFactor struct {
	Code            string   `json:"code"`
	Dimension       string   `json:"dimension"`
	Severity        string   `json:"severity"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	MetricValue     *float64 `json:"metric_value,omitempty"`
	MetricUnit      string   `json:"metric_unit,omitempty"`
	RelatedSymbols  []string `json:"related_symbols,omitempty"`
}

// RiskWarning is a short alert.
type RiskWarning struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	FactorCodes []string `json:"factor_codes,omitempty"`
}

// RiskSuggestion is a review/monitor hint — never execute/sell/buy.
type RiskSuggestion struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Kind    string `json:"kind"` // review | monitor | inform
}

// DimensionsView summarizes per-dimension scores.
type DimensionsView struct {
	Portfolio DimensionSlice `json:"portfolio"`
	Position  DimensionSlice `json:"position"`
	Execution DimensionSlice `json:"execution"`
	Market    DimensionSlice `json:"market"`
}

// DimensionSlice is one dimension's score + factor count.
type DimensionSlice struct {
	Score       int `json:"score"`
	FactorCount int `json:"factor_count"`
	Available   bool `json:"available"`
}

// SourceRef cites an upstream observation.
type SourceRef struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

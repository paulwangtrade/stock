// Package obsfeedback is the read-only quantitative observation feedback loop (Phase10-E.4).
//
// Records Holding Decision snapshots, evaluates future market outcomes, and aggregates
// performance metrics. Does not trade, write paper_sim_*, or change Decision rules.
package obsfeedback

const Disclaimer = "历史判断统计，不代表未来收益。不是交易建议。不生成买卖单。"

const (
	SourcePortfolioObservation = "portfolio_observation"
	ActionNone                 = "none"
)

const (
	HorizonT1       = 1
	HorizonT5       = 5
	HorizonT10      = 10
	DefaultHorizon  = HorizonT5
	BenchmarkCSI300 = "csi300"
	BenchmarkCSI500 = "csi500"
	BenchmarkNone   = "none"
)

const (
	BenchmarkCodeCSI300 = "000300.SH"
	BenchmarkCodeCSI500 = "000905.SH"
)

const (
	OutcomeEvaluated     = "EVALUATED"
	OutcomeInsufficient  = "INSUFFICIENT_DATA"
	OutcomePending       = "PENDING"
)

const (
	StateHoldNormal    = "HOLD_NORMAL"
	StateHoldWatch     = "HOLD_WATCH"
	StateHoldReview    = "HOLD_REVIEW"
	StateExitCandidate = "EXIT_CANDIDATE"
)

// Record is what the system saw at observation time (not a trade signal).
type Record struct {
	ObservationID     string   `json:"observation_id"`
	Symbol            string   `json:"symbol"`
	Market            string   `json:"market"`
	ObservationTime   string   `json:"observation_time"`
	ObservationDate   string   `json:"observation_date"`
	DecisionState     string   `json:"decision_state"`
	DecisionReason    string   `json:"decision_reason,omitempty"`
	HealthScore       *float64 `json:"health_score,omitempty"`
	HoldingDays       int      `json:"holding_days"`
	CostPrice         *float64 `json:"cost_price,omitempty"`
	MarketPrice       *float64 `json:"market_price,omitempty"`
	UnrealizedReturn  *float64 `json:"unrealized_return,omitempty"`
	PortfolioWeight   *float64 `json:"portfolio_weight,omitempty"`
	Source            string   `json:"source"`
}

// Bar is one daily OHLC used for outcome evaluation.
type Bar struct {
	Date  string
	Close float64
	High  float64
	Low   float64
}

// SeriesProvider supplies daily bars after an observation date (tests inject fakes).
type SeriesProvider interface {
	DailyBars(symbol string, fromDate string, limit int) ([]Bar, error)
}

// Outcome is the future-market evaluation of one Record at one horizon.
type Outcome struct {
	ObservationID    string   `json:"observation_id"`
	Horizon          int      `json:"horizon"`
	Status           string   `json:"status"`
	ObservationDate  string   `json:"observation_date"`
	FutureDate       string   `json:"future_date,omitempty"`
	FuturePrice      *float64 `json:"future_price,omitempty"`
	FutureReturn     *float64 `json:"future_return,omitempty"`
	MaxDrawdown      *float64 `json:"max_drawdown,omitempty"`
	MaxProfit        *float64 `json:"max_profit,omitempty"`
	BenchmarkID      string   `json:"benchmark_id,omitempty"`
	BenchmarkReturn  *float64 `json:"benchmark_return,omitempty"`
	Alpha            *float64 `json:"alpha,omitempty"`
	Note             string   `json:"note,omitempty"`
}

// StateMetrics aggregates EVALUATED outcomes for one decision_state.
type StateMetrics struct {
	DecisionState     string   `json:"decision_state"`
	Samples           int      `json:"samples"`
	Evaluated         int      `json:"evaluated"`
	Pending           int      `json:"pending"`
	Insufficient      int      `json:"insufficient"`
	WinRate           *float64 `json:"win_rate,omitempty"`
	AvgReturn         *float64 `json:"avg_return,omitempty"`
	AvgAlpha          *float64 `json:"avg_alpha,omitempty"`
	AvgMaxDrawdown    *float64 `json:"avg_max_drawdown,omitempty"`
	RiskCaptureRate   *float64 `json:"risk_capture_rate,omitempty"`
}

// PerformanceView is the read-only metrics payload.
type PerformanceView struct {
	Horizon            int                     `json:"horizon"`
	Benchmark          string                  `json:"benchmark"`
	Samples            int                     `json:"samples"`
	Evaluated          int                     `json:"evaluated"`
	DecisionAccuracy   *float64                `json:"decision_accuracy"`
	WinRate            *float64                `json:"win_rate"`
	AvgReturn          *float64                `json:"avg_return"`
	AvgAlpha           *float64                `json:"avg_alpha"`
	ByState            map[string]StateMetrics `json:"by_state"`
	IndustryBenchmark  IndustryBenchmarkView   `json:"industry_benchmark"`
	Disclaimer         string                  `json:"disclaimer"`
	Action             string                  `json:"action"`
}

// IndustryBenchmarkView is an E.4 stub (no industry index map yet).
type IndustryBenchmarkView struct {
	Available bool   `json:"available"`
	Note      string `json:"note"`
}

const industryBenchmarkNote = "行业指数映射未启用；E.4 仅支持沪深300/中证500 全市场基准"

func fptr(v float64) *float64 { return &v }

func safeRatio(n, d int) *float64 {
	if d <= 0 {
		return nil
	}
	v := float64(n) / float64(d)
	return &v
}

func safeMean(sum float64, n int) *float64 {
	if n <= 0 {
		return nil
	}
	v := sum / float64(n)
	return &v
}

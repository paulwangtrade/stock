package mobileprojection

import (
	"time"

	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolioobservation"
)

const (
	SchemaVersion = "mobile_observation_projection.p13-v1"
	DefaultEnabled  = false

	DisclaimerKey = "mobile_read.disclaimer.v1"
	DisclaimerZH  = "组合观察与文字说明，不构成投资建议，不会自动买卖。"
	ExportNote    = "no trade writes; no holdings detail; no account_id; no broker credentials"

	dataSourceNote = "mobileprojection · Desktop read-only projection; default OFF; not Cloud; not Execution"
)

// Input aggregates read-only sources for one projection build.
type Input struct {
	// Enabled must be true to emit filled summary/risk/attention/insight blocks.
	Enabled bool

	DeviceIDHash8 string
	UploadedAt    time.Time

	Observation *portfolioobservation.PortfolioObservationView
	AIInsight   *AIInsight
	Attention   *attention.DailyAttentionView
}

// AIInsight is the Desktop-side AI insight report input (aiinsight.p13-v1 shape).
// Optional; when nil insight is derived from Observation explain factors only.
type AIInsight struct {
	SchemaVersion       string
	AsOf                time.Time
	TradeDate           string
	NarrativeMode       string // rules_only | rules_plus_llm | unavailable
	Available           bool
	WhyBuyLimited       ExplainSection
	WhyReducePosition   ExplainSection
	WhyRiskElevated     ExplainSection
	Sources             AIInsightSources
	DataGaps            []string
	EvidenceFingerprint string
	DataSourceNote      string
}

// ExplainSection is one narrative block (shared input/output shape).
type ExplainSection struct {
	Available         bool
	Headline          string
	Paragraphs        []string
	Bullets           []ExplainBullet
	UnavailableReason string
	Caution           string
}

// ExplainBullet is one evidence bullet without executable payload.
type ExplainBullet struct {
	Code      string
	PlainText string
	Source    string
}

// AIInsightSources records upstream presence flags.
type AIInsightSources struct {
	Observation       bool
	Risk              bool
	HoldingsDecision  bool
	AllocationShadow  bool
}

// MobileObservationProjection is the Desktop → Cloud upload DTO (built locally only).
type MobileObservationProjection struct {
	SchemaVersion   string    `json:"schema_version"`
	DeviceIDHash8   string    `json:"device_id_hash8,omitempty"`
	TradeDate       string    `json:"trade_date"`
	UploadedAt      time.Time `json:"uploaded_at"`

	Enabled    bool   `json:"enabled"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`

	Summary   *MobilePortfolioSummary `json:"summary,omitempty"`
	Risk      *MobileRiskSummary      `json:"risk,omitempty"`
	Attention *MobileAlertsView       `json:"attention,omitempty"`
	Insight   *MobileAIInsightView    `json:"insight,omitempty"`

	RecordOnly bool   `json:"record_only"`
	ExportNote string `json:"export_note"`
}

// MobilePortfolioSummary is the mobile portfolio shape card (no holdings list).
type MobilePortfolioSummary struct {
	SchemaVersion      string                    `json:"schema_version"`
	TradeDate          string                    `json:"trade_date"`
	AsOf               time.Time                 `json:"as_of"`
	Found              bool                      `json:"found"`
	NameCount          int                       `json:"name_count,omitempty"`
	CashRatio          *float64                  `json:"cash_ratio,omitempty"`
	GrossExposure      *float64                  `json:"gross_exposure,omitempty"`
	Top1Weight         *float64                  `json:"top1_weight,omitempty"`
	HeadroomVsCap      *float64                  `json:"headroom_vs_cap,omitempty"`
	DecisionAttention  string                    `json:"decision_attention"`
	TradingChain       MobileTradingChainObserve `json:"trading_chain"`
	Strategy           *MobileStrategyRef        `json:"strategy,omitempty"`
	Quality            string                    `json:"quality"`
	DataGaps           []string                  `json:"data_gaps"`
	DataSourceNote     string                    `json:"data_source_note"`
}

// MobileTradingChainObserve is phase/block observation only (not execution API).
type MobileTradingChainObserve struct {
	Phase         string   `json:"phase"`
	BlockReasons  []string `json:"block_reasons"`
	MessageSafe   string   `json:"message_safe"`
}

// MobileStrategyRef is strategy display metadata (no parameters).
type MobileStrategyRef struct {
	Name          string `json:"name,omitempty"`
	VersionHash8  string `json:"version_hash8,omitempty"`
}

// MobileRiskSummary is risk observation for Mobile.
type MobileRiskSummary struct {
	SchemaVersion   string                  `json:"schema_version"`
	TradeDate       string                  `json:"trade_date"`
	AsOf            time.Time               `json:"as_of"`
	RiskLevel       string                  `json:"risk_level"`
	RiskLevelLabel  string                  `json:"risk_level_label"`
	Exposure        MobileRiskExposure      `json:"exposure"`
	Concentration   MobileRiskConcentration `json:"concentration"`
	Sector          MobileRiskSector        `json:"sector"`
	ExplainCodes    []string                `json:"explain_codes"`
	Quality         string                  `json:"quality"`
	DataGaps        []string                `json:"data_gaps"`
	Caution         string                  `json:"caution"`
}

type MobileRiskExposure struct {
	Available      bool     `json:"available"`
	GrossExposure  *float64 `json:"gross_exposure,omitempty"`
	HeadroomVsCap  *float64 `json:"headroom_vs_cap,omitempty"`
	Note           string   `json:"note,omitempty"`
}

type MobileRiskConcentration struct {
	Available   bool     `json:"available"`
	Top1Weight  *float64 `json:"top1_weight,omitempty"`
	NameCount   *int     `json:"name_count,omitempty"`
}

type MobileRiskSector struct {
	Available             bool     `json:"available"`
	AllowSectorConstraint bool     `json:"allow_sector_constraint"`
	MaxSectorWeight       *float64 `json:"max_sector_weight,omitempty"`
	CoverageNote          string   `json:"coverage_note,omitempty"`
}

// MobileAIInsightView is the mobile AI insight block.
type MobileAIInsightView struct {
	SchemaVersion        string          `json:"schema_version"`
	TradeDate            string          `json:"trade_date"`
	AsOf                 time.Time       `json:"as_of"`
	NarrativeMode        string          `json:"narrative_mode"`
	Available            bool            `json:"available"`
	WhyBuyLimited        ExplainSection  `json:"why_buy_limited"`
	WhyReducePosition    ExplainSection  `json:"why_reduce_position"`
	WhyRiskElevated      ExplainSection  `json:"why_risk_elevated"`
	Sources              AIInsightSources `json:"sources"`
	DataGaps             []string        `json:"data_gaps"`
	EvidenceFingerprint  string          `json:"evidence_fingerprint,omitempty"`
	DataSourceNote       string          `json:"data_source_note"`
}

// MobileAlertsView is Daily Attention for Mobile (no plan_id).
type MobileAlertsView struct {
	SchemaVersion  string            `json:"schema_version"`
	TradeDate      string            `json:"trade_date"`
	AsOf           time.Time         `json:"as_of"`
	OverallAction  string            `json:"overall_action"`
	Headline       string            `json:"headline,omitempty"`
	Items          []MobileAlertItem `json:"items"`
	Counts         MobileAlertCounts `json:"counts"`
	Quality        string            `json:"quality"`
	MissingInputs  []string          `json:"missing_inputs,omitempty"`
	Disclaimer     string            `json:"disclaimer"`
}

type MobileAlertItem struct {
	ID               string `json:"id"`
	ItemType         string `json:"item_type"`
	Priority         int    `json:"priority"`
	Title            string `json:"title"`
	Reason           string `json:"reason"`
	Source           string `json:"source"`
	Severity         string `json:"severity"`
	SuggestedAction  string `json:"suggested_action"`
	StockCode        string `json:"stock_code,omitempty"`
}

type MobileAlertCounts struct {
	Review int `json:"review"`
	Watch  int `json:"watch"`
	Hold   int `json:"hold"`
	Total  int `json:"total"`
}

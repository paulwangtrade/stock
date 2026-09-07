package tradingconfig

// Source identifies which legacy or future store supplied a resolved value.
// Phase6.5-A: only legacy_* sources are active; trading_config is reserved.
type Source string

const (
	SourceLegacyPaperOpenBuy Source = "legacy_paper_open_buy"
	SourceLegacyPaperConfig  Source = "legacy_paper_config" // Risk slice from paper_open_buy.json
	SourceLegacyAfterClose   Source = "legacy_after_close"
	SourceLegacyAutomation   Source = "legacy_automation"
	SourceLegacyPaperMVP     Source = "legacy_paper_trading_mvp"
	SourceTradingConfig      Source = "trading_config"  // future; unused in Phase6.5-A/B
	SourceCompileDefault     Source = "compile_default" // hardcoded slice defaults (e.g. ProviderShadow)
)

const (
	SizingMethodFixedAmount         = "fixed_amount"
	SizingMethodPortfolioAware      = "portfolio_aware"
	PositionSizerModeFixedAmount    = SizingMethodFixedAmount
	PositionSizerModePortfolioAware = SizingMethodPortfolioAware
	DefaultFixedAmount              = 100_000
	DefaultPaperInitialCash         = 1_000_000
	DefaultFillMode                 = "A"
	DefaultPositionSizerMode        = PositionSizerModeFixedAmount
)

// PositionView is the Position Sizing slice exposed to callers.
type PositionView struct {
	SizingMethod      string  `json:"sizing_method"`
	MaxPositionAmount float64 `json:"max_position_amount"`
	Source            Source  `json:"source"`
}

// WorkflowView is the workflow switch slice.
type WorkflowView struct {
	AfterCloseEnabled bool   `json:"after_close_enabled"`
	Source            Source `json:"source"`
}

// ExecutionSwitchView is Track-A open-buy / plan EnableExecute switch (paper_open_buy.json).
type ExecutionSwitchView struct {
	EnablePaperOpenBuy bool   `json:"enable_paper_open_buy"`
	Source             Source `json:"source"`
}

// PaperMVPView is Track-B Paper Trading MVP switches (paper_trading_mvp.json).
type PaperMVPView struct {
	EnablePaperTrading bool    `json:"enable_paper_trading"`
	InitialCash        float64 `json:"initial_cash"`
	FillMode           string  `json:"fill_mode"`
	// PositionSizerMode selects Draft buy sizing: fixed_amount (default) | portfolio_aware.
	// Missing / unknown JSON values normalize to fixed_amount (existing production behavior).
	PositionSizerMode string             `json:"position_sizer_mode,omitempty"`
	PortfolioAware    PortfolioAwareView `json:"portfolio_aware,omitempty"`
	Source            Source             `json:"source"`
}

// PortfolioAwareView is the Track-B nested policy for portfolio_aware sizing.
// Zero values mean "use Sizer defaults"; ignored when mode is fixed_amount.
type PortfolioAwareView struct {
	MaxExposure             float64 `json:"max_exposure,omitempty"`
	MaxSinglePositionWeight float64 `json:"max_single_position_weight,omitempty"`
	ReserveCashRatio        float64 `json:"reserve_cash_ratio,omitempty"`
	MinOrderAmount          float64 `json:"min_order_amount,omitempty"`
}

// AutomationView is a read-only observability snapshot of legacy UI automation.
type AutomationView struct {
	Enabled              bool    `json:"enabled"`
	AccountEquity        float64 `json:"account_equity,omitempty"`
	RiskPerTradePct      float64 `json:"risk_per_trade_pct,omitempty"`
	MaxPositionPct       float64 `json:"max_position_pct,omitempty"`
	MaxTotalExposurePct  float64 `json:"max_total_exposure_pct,omitempty"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes,omitempty"`
	AlertCooldownMinutes int     `json:"alert_cooldown_minutes,omitempty"`
	Source               Source  `json:"source"`
	Present              bool    `json:"present"`
}

// RiskView is the Plan Risk / Morning exposure slice (Phase6.5-B).
type RiskView struct {
	Enabled                  bool    `json:"enabled"`
	MarketLevel              int     `json:"market_level"`
	BlockNewEntriesOnDefense bool    `json:"block_new_entries_on_defense"`
	MaxGrossExposurePct      float64 `json:"max_gross_exposure_pct"`
	MaxSingleNamePct         float64 `json:"max_single_name_pct"`
	MaxDailyLossPct          float64 `json:"max_daily_loss_pct"`
	CurrentDailyPnlPct       float64 `json:"current_daily_pnl_pct"`
	Source                   Source  `json:"source"`
}

// ProviderShadowView is the G.12 Portfolio Provider Shadow switch.
// Production default is Enabled=false (compile-time; not loaded from trade JSON).
type ProviderShadowView struct {
	Enabled bool   `json:"enabled"`
	Source  Source `json:"source"`
}

// Resolved is the compatibility-layer snapshot returned by the Provider.
type Resolved struct {
	Position       PositionView        `json:"position"`
	Workflow       WorkflowView        `json:"workflow"`
	Execution      ExecutionSwitchView `json:"execution"`
	PaperMVP       PaperMVPView        `json:"paper_mvp"`
	Risk           RiskView            `json:"risk"`
	Automation     AutomationView      `json:"automation"`
	ProviderShadow ProviderShadowView  `json:"provider_shadow"`
}

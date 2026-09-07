package tradingdaymonitor

import "time"

// Status codes aligned with TradingEvent status + PENDING/UNKNOWN for gaps.
const (
	StatusPass    = "PASS"
	StatusFail    = "FAIL"
	StatusSkip    = "SKIP"
	StatusPending = "PENDING"
	StatusUnknown = "UNKNOWN"
)

const (
	SourceTradingEvent = "trading_event"
	SourceReadiness    = "readiness"
)

// TradingDayMonitorView is the read-only Day Monitor projection (Phase11-C).
type TradingDayMonitorView struct {
	TradeDate      string           `json:"trade_date"`
	AsOf           time.Time        `json:"as_of"`
	Morning        MorningSection   `json:"morning"`
	Execution      ExecutionSection `json:"execution"`
	Settlement     SettlementSection `json:"settlement"`
	// PositionStates: Phase11-K unified holding states (read-only; not TradingEvent).
	PositionStates []PositionStateRow `json:"position_states,omitempty"`
	DataSourceNote string             `json:"data_source_note"`
	Disclaimer     string             `json:"disclaimer"`
}

// PositionStateRow is a thin Monitor projection of PositionStateView.
type PositionStateRow struct {
	Symbol        string `json:"symbol"`
	State         string `json:"state"`
	TotalQty      int64  `json:"total_qty"`
	AvailableQty  int64  `json:"available_qty"`
	LockedQty     int64  `json:"locked_qty"`
	IsNewPosition bool   `json:"is_new_position"`
	CanSell       bool   `json:"can_sell"`
	HoldingDays   int    `json:"holding_days,omitempty"`
	RiskTag       string `json:"risk_tag,omitempty"`
}

// MorningSection covers materialize / approve / freeze observation.
type MorningSection struct {
	Materialize StepStatus `json:"materialize"`
	Approve     StepStatus `json:"approve"`
	Freeze      StepStatus `json:"freeze"`
}

// StepStatus is one morning workflow step.
type StepStatus struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	PlanID    uint   `json:"plan_id,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Source    string `json:"source,omitempty"`
}

// ExecutionSection is Gateway / fill-window observation.
type ExecutionSection struct {
	Session   string `json:"session,omitempty"`
	Status    string `json:"status"`
	PlanID    uint   `json:"plan_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Source    string `json:"source,omitempty"`
}

// SettlementSection is EOD settlement observation.
type SettlementSection struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Source    string `json:"source,omitempty"`
}

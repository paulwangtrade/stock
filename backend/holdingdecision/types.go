// Package holdingdecision is a read-only Holding Decision Engine (Phase10-D.8).
//
// Maps Holding Evaluation facts → decision_state. Pure functions: no DB, Broker, Execution, or sell orders.
package holdingdecision

// Decision states (not sell instructions).
const (
	StateHoldNormal     = "HOLD_NORMAL"
	StateHoldWatch      = "HOLD_WATCH"
	StateHoldReview     = "HOLD_REVIEW"
	StateExitCandidate  = "EXIT_CANDIDATE" // produced only when Policy.ExitCandidateEnabled
)

// Reason codes (no SELL / EXIT_NOW / FORCE_CLOSE).
const (
	ReasonNone           = "NONE"
	ReasonDataMissing    = "DATA_MISSING"
	ReasonProfitWeakness = "PROFIT_WEAKNESS"
	ReasonRiskIncrease   = "RISK_INCREASE"
	ReasonRiskMaterial   = "RISK_MATERIAL"
)

const (
	HintContinueHold    = "CONTINUE_HOLD"
	HintKeepWatching    = "KEEP_WATCHING"
	HintReassessThesis  = "REASSESS_THESIS"
	HintConsiderExitEval = "CONSIDER_EXIT_EVAL" // only with ExitCandidateEnabled; not a sell
)

// Policy controls conservative v1 rules. ExitCandidateEnabled default false.
type Policy struct {
	ExitCandidateEnabled bool
}

// DefaultPolicy is production-safe: EXIT_CANDIDATE stays closed.
func DefaultPolicy() Policy {
	return Policy{ExitCandidateEnabled: false}
}

// Evidence is a fact snapshot copied from evaluation (not a trade instruction).
type Evidence struct {
	CurrentPrice *float64 `json:"current_price,omitempty"`
	ReturnRate   *float64 `json:"return_rate,omitempty"`
	HoldingDays  int      `json:"holding_days"`
	RiskState    string   `json:"risk_state,omitempty"`
	ProfitState  string   `json:"profit_state,omitempty"`
	PeriodState  string   `json:"period_state,omitempty"`
	QuoteSource  string   `json:"quote_source,omitempty"`
}

// HoldingDecision is one lot- or stock-level conclusion.
type HoldingDecision struct {
	Symbol        string   `json:"symbol"`
	FillID        uint     `json:"fill_id,omitempty"`
	PlanID        uint     `json:"plan_id,omitempty"`
	State         string   `json:"state"`
	Reason        string   `json:"reason"` // primary reason
	ReasonCodes   []string `json:"reason_codes"`
	NextHint      string   `json:"next_hint"`
	Summary       string   `json:"summary"`
	Evidence      Evidence `json:"evidence"`
	Action        string   `json:"action"` // always "none" in D.8
}

// StockDecision aggregates lots for one symbol (worst state).
type StockDecision struct {
	Symbol      string            `json:"symbol"`
	State       string            `json:"state"`
	Reason      string            `json:"reason"`
	ReasonCodes []string          `json:"reason_codes"`
	NextHint    string            `json:"next_hint"`
	Summary     string            `json:"summary"`
	Action      string            `json:"action"`
	Lots        []HoldingDecision `json:"lots,omitempty"`
}

// View is the observation payload (holding_decision).
type View struct {
	AsOf               string          `json:"as_of,omitempty"`
	ExitCandidateEnabled bool          `json:"exit_candidate_enabled"`
	ByState            map[string]int  `json:"by_state"`
	Holdings           []StockDecision `json:"holdings"`
	DataSourceNote     string          `json:"data_source_note"`
}

const dataSourceNote = "Holding Decision · read-only from Holding Evaluation facts; not a sell recommendation; EXIT_CANDIDATE default off"

// Engine is the pure Evaluate entry.
type Engine interface {
	Evaluate(eval any, policy Policy) *View
}

type engine struct{}

// NewEngine returns the v1 conservative engine.
func NewEngine() Engine {
	return engine{}
}

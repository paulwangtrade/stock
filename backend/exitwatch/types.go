// Package exitwatch is a shared read-only observation alert for paper_sim and
// external_mirror holdings. It never sells, never persists trailing/HWM, and
// never opens a ledger.
package exitwatch

import "time"

const (
	SourcePaperSim       = "paper_sim"
	SourceExternalMirror = "external_mirror"

	ClassHold         = "HOLD_OBSERVE"
	ClassReduce       = "REDUCE_OBSERVE"
	ClassFlatten      = "FLATTEN_OBSERVE"
	ClassInsufficient = "DATA_INSUFFICIENT"

	ReasonTime     = "TIME"
	ReasonLoss     = "LOSS"
	ReasonSignal   = "SIGNAL"
	ReasonPlan     = "PLAN"
	ReasonStale    = "STALE"
	ReasonNoSource = "NO_SOURCE"
	ReasonT1Locked = "T1_LOCKED"

	RuleHold       = "HOLD"
	RuleIncomplete = "INCOMPLETE"
)

// Disclaimer is the user-facing copy. Observation is not an order.
const Disclaimer = "观察≠卖出指令"

const dataSourceNote = "ExitWatch · read-only observation alerts; not an order; no trailing or HWM persistence; no new ledger"

// QuoteStaleAfter matches holding-evaluation price freshness (24h).
const QuoteStaleAfter = 24 * time.Hour

// Policy is the observation threshold snapshot carried in the payload.
// Numbers match the default exit-evaluation policy.
type Policy struct {
	ID                  string
	Version             int
	MaxHoldingDays      int
	LossWatchThreshold  float64
	LossReviewThreshold float64
}

// DefaultPolicy is default_v1 (days > 20, watch -5%, review -10%).
var DefaultPolicy = Policy{
	ID:                  "default_v1",
	Version:             1,
	MaxHoldingDays:      20,
	LossWatchThreshold:  -0.05,
	LossReviewThreshold: -0.10,
}

func (p Policy) normalized() Policy {
	out := p
	if out.ID == "" {
		out.ID = DefaultPolicy.ID
	}
	if out.Version <= 0 {
		out.Version = DefaultPolicy.Version
	}
	if out.MaxHoldingDays <= 0 {
		out.MaxHoldingDays = DefaultPolicy.MaxHoldingDays
	}
	if out.LossWatchThreshold == 0 && out.LossReviewThreshold == 0 {
		out.LossWatchThreshold = DefaultPolicy.LossWatchThreshold
		out.LossReviewThreshold = DefaultPolicy.LossReviewThreshold
	} else {
		if out.LossWatchThreshold == 0 {
			out.LossWatchThreshold = DefaultPolicy.LossWatchThreshold
		}
		if out.LossReviewThreshold == 0 {
			out.LossReviewThreshold = DefaultPolicy.LossReviewThreshold
		}
	}
	if out.LossReviewThreshold > out.LossWatchThreshold {
		out.LossReviewThreshold = out.LossWatchThreshold
	}
	return out
}

// Ref is policy id and version, e.g. default_v1@v1.
func (p Policy) Ref() string {
	n := p.normalized()
	return n.ID + "@v" + itoa(n.Version)
}

// Item is one observation alert. DedupKey is source|position_id|rule|bar.
type Item struct {
	Source                string    `json:"source"`
	PositionID            string    `json:"position_id"`
	StockCode             string    `json:"stock_code"`
	StockName             string    `json:"stock_name"`
	Class                 string    `json:"class"`
	Label                 string    `json:"label"`
	ReasonCodes           []string  `json:"reason_codes"`
	Rule                  string    `json:"rule"`
	DedupKey              string    `json:"dedup_key"`
	Bar                   string    `json:"bar"`
	PolicyID              string    `json:"policy_id"`
	PolicyVersion         int       `json:"policy_version"`
	PolicyRef             string    `json:"policy_ref"`
	AsOf                  time.Time `json:"as_of"`
	Summary               string    `json:"summary"`
	SellIntentAllowed     bool      `json:"sell_intent_allowed"`
	RequiresManualConfirm bool      `json:"requires_manual_confirm"`
	ManualSellDraftRef    string    `json:"manual_sell_draft_ref,omitempty"`
	NotAnOrder            bool      `json:"not_an_order"`
	Disclaimer            string    `json:"disclaimer"`
}

// SourceStatus reports whether a book could be read. A failure does not escalate.
type SourceStatus struct {
	Source  string `json:"source"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// View is the shared payload.
type View struct {
	Items           []Item         `json:"items"`
	Sources         []SourceStatus `json:"sources"`
	AsOf            time.Time      `json:"as_of"`
	PolicyID        string         `json:"policy_id"`
	PolicyVersion   int            `json:"policy_version"`
	PolicyRef       string         `json:"policy_ref"`
	Disclaimer      string         `json:"disclaimer"`
	AutoSell        bool           `json:"auto_sell"`
	PersistTrailing bool           `json:"persist_trailing"`
	PersistHWM      bool           `json:"persist_hwm"`
	NewLedger       bool           `json:"new_ledger"`
	WritesTradePlan bool           `json:"writes_trade_plan"`
	WritesPaperSim  bool           `json:"writes_paper_sim"`
	DataSourceNote  string         `json:"data_source_note"`
}

// Facts is the source-agnostic input to Project. Adapters fill it.
// Incomplete price, cost, identity, or source must not be upgraded into an exit class.
type Facts struct {
	Source             string
	PositionID         string
	StockCode          string
	StockName          string
	Quantity           int64
	QuantityKnown      bool
	RequireQuantity    bool
	CostKnown          bool
	PriceKnown         bool
	PriceStale         bool
	UnrealizedReturn   *float64
	HoldingDays        int
	HoldingDaysKnown   bool
	RequireHoldingDays bool
	UseExitEvaluation  bool
	ExitState          string
	ExitReasonCodes    []string
	SignalReview       bool
	T1Locked           bool
	ManualDraftRef     string
	Bar                string
	AsOf               time.Time
	Policy             Policy
}

// QuoteSnap is a price snapshot. The mirror adapter reads only this plus the mirror row.
type QuoteSnap struct {
	Code      string
	Price     float64
	FetchedAt time.Time
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

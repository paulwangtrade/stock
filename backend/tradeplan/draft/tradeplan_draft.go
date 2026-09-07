package draft

import "time"

// DraftStatus Phase4-B lifecycle enum (frozen).
type DraftStatus string

const (
	DraftCreated   DraftStatus = "CREATED"
	DraftValidated DraftStatus = "VALIDATED"
	DraftRejected  DraftStatus = "REJECTED"
	DraftExpired   DraftStatus = "EXPIRED"
)

// Validation reason codes (frozen).
const (
	CodeSourceMismatch        = "source_mismatch"
	CodeSnapshotChanged       = "snapshot_changed"
	CodeExecuteForbidden      = "execute_forbidden"
	CodeDraftMutationDetected = "draft_mutation_detected"
	CodeAuthorityDenied       = "authority_denied"
)

// DraftPayload projected Decision facts (mutable detection via DraftHash).
type DraftPayload struct {
	TradeDate      string  `json:"tradeDate"`
	StockCode      string  `json:"stockCode"`
	StockName      string  `json:"stockName"`
	Side           string  `json:"side"`
	ActionCode     string  `json:"actionCode"`
	ActionLabel    string  `json:"actionLabel,omitempty"`
	AllowDraft     bool    `json:"allowDraft"`
	TargetShares   int64   `json:"targetShares"`
	AddShares      int64   `json:"addShares"`
	TargetAmount   float64 `json:"targetAmount"`
	PositionPct    float64 `json:"positionPct,omitempty"`
	EntryPriceHint float64 `json:"entryPriceHint,omitempty"`
	StopPrice      float64 `json:"stopPrice,omitempty"`
	MarketLevel    int     `json:"marketLevel,omitempty"`
	GateReady      bool    `json:"gateReady"`
}

// DraftValidation last validation outcome attached to the draft.
type DraftValidation struct {
	OK      bool     `json:"ok"`
	Codes   []string `json:"codes,omitempty"`
	Message string   `json:"message,omitempty"`
}

// TradePlanDraft lifecycle-aware projection (not a DB TradePlan / not an Order).
type TradePlanDraft struct {
	DraftID            string          `json:"draftId"`
	SourceDecisionID   string          `json:"sourceDecisionId"`
	SourceSnapshotHash string          `json:"sourceSnapshotHash"`
	SnapshotHash       string          `json:"snapshotHash"` // Phase4-A alias (= SourceSnapshotHash)
	Status             DraftStatus     `json:"status"`
	CreatedAt          time.Time       `json:"createdAt"`
	EnableExecute      bool            `json:"enableExecute"` // always false; never execution auth
	Validation         DraftValidation `json:"validation"`
	Payload            DraftPayload    `json:"payload"`
	DraftHash          string          `json:"draftHash"`
	ConsumerRole       string          `json:"consumerRole"`
	Message            string          `json:"message,omitempty"`
}

// Phase4-A field accessors (compat for older tests / golden readers).
func (d *TradePlanDraft) ActionCode() string {
	if d == nil {
		return ""
	}
	return d.Payload.ActionCode
}

func (d *TradePlanDraft) TargetShares() int64 {
	if d == nil {
		return 0
	}
	return d.Payload.TargetShares
}

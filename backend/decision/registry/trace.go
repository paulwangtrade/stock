package registry

import (
	"encoding/json"
	"strings"

	"go-stock/backend/models"
)

// Stale reason codes (traceability).
const (
	StaleReasonNone             = ""
	StaleReasonMissingRegistry  = "missing_from_registry"
	StaleReasonCodeMismatch     = "code_mismatch"
	StaleReasonTradeDateMismatch = "trade_date_mismatch"
)

// TraceResult CandidatePoolItem → decisionId → Registry → Decision.
// Read-only: never mutates the input item's Score / Rank.
type TraceResult struct {
	DecisionID    string                 `json:"decisionId"`
	Bound         bool                   `json:"bound"`
	Found         bool                   `json:"found"`
	Stale         bool                   `json:"stale"`
	StaleReason   string                 `json:"staleReason,omitempty"`
	ItemCode      string                 `json:"itemCode"`
	ItemTradeDate string                 `json:"itemTradeDate"`
	ItemRank      int                    `json:"itemRank"`
	ItemScore     float64                `json:"itemScore"`
	Decision      *models.QuantDecision  `json:"decision,omitempty"`
	Summary       string                 `json:"summary"`
}

// TraceItem resolves Item.DecisionID against the registry without mutating Score/Rank.
func (r *Registry) TraceItem(item models.CandidatePoolItem) TraceResult {
	tr := TraceResult{
		DecisionID:    strings.TrimSpace(item.DecisionID),
		ItemCode:      item.StockCode,
		ItemTradeDate: item.TradeDate,
		ItemRank:      item.Rank,
		ItemScore:     item.Score,
	}

	if tr.DecisionID == "" {
		tr.Bound = false
		tr.Found = false
		tr.Stale = false
		tr.Summary = "UNBOUND: empty decisionId"
		return tr
	}
	tr.Bound = true

	d, ok := r.Get(tr.DecisionID)
	if !ok || d == nil {
		tr.Found = false
		tr.Stale = true
		tr.StaleReason = StaleReasonMissingRegistry
		tr.Summary = "STALE: decisionId missing from registry"
		return tr
	}
	tr.Found = true
	tr.Decision = d

	itemCode := normalizeCode(item.StockCode)
	decCode := normalizeCode(d.Instrument.StockCode)
	if itemCode != "" && decCode != "" && itemCode != decCode {
		tr.Stale = true
		tr.StaleReason = StaleReasonCodeMismatch
		tr.Summary = "STALE: stockCode mismatch"
		return tr
	}

	itemDate := strings.TrimSpace(item.TradeDate)
	decDate := strings.TrimSpace(d.TradeDate)
	if itemDate != "" && decDate != "" && itemDate != decDate {
		tr.Stale = true
		tr.StaleReason = StaleReasonTradeDateMismatch
		tr.Summary = "STALE: tradeDate mismatch"
		return tr
	}

	tr.Stale = false
	tr.StaleReason = StaleReasonNone
	tr.Summary = "OK: traced to decision"
	return tr
}

func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func cloneDecision(d *models.QuantDecision) *models.QuantDecision {
	if d == nil {
		return nil
	}
	b, err := json.Marshal(d)
	if err != nil {
		cp := *d
		return &cp
	}
	var out models.QuantDecision
	if err := json.Unmarshal(b, &out); err != nil {
		cp := *d
		return &cp
	}
	return &out
}

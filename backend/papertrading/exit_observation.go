// Exit observation gate — read-only projection of Exit Evaluation (Track-B paper_sim).
//
// Maps the existing NORMAL / WATCH / REVIEW_REQUIRED labels onto observation classes:
// 持有观察 / 减仓观察 / 清仓观察 / 数据不足.
// Missing return, stale price, or an unknown state fail closed to 数据不足.
// Health score does not change the class (same rule as Exit Evaluation).
// Never writes TradePlan, Broker orders, or paper_sim fills.

package papertrading

import (
	"math"
	"strings"
)

const (
	ExitObserveClassHold         = "HOLD_OBSERVE"
	ExitObserveClassReduce       = "REDUCE_OBSERVE"
	ExitObserveClassFlatten      = "FLATTEN_OBSERVE"
	ExitObserveClassInsufficient = "DATA_INSUFFICIENT"
)

// ExitObservationDisclaimer is the user-facing gate copy (not an order).
const ExitObservationDisclaimer = "非交易指令 · 不进入实盘 · 需人工确认才可走模拟卖出"

const exitObservationNote = "Exit observation gate · projection of Exit Evaluation state; not a broker order; persist_sell_plans=false; does not write TradePlan"

// ExitObservation is the per-position observation shown on paper-sim holdings.
type ExitObservation struct {
	Class                 string `json:"class"`
	Label                 string `json:"label"`
	Reason                string `json:"reason"`
	NotAnOrder            bool   `json:"not_an_order"`
	NotLive               bool   `json:"not_live"`
	RequiresManualConfirm bool   `json:"requires_manual_confirm"`
	// SellIntentAllowed is a UI hint only. The client must still confirm before
	// opening a manual sell draft. This projector never creates that draft.
	SellIntentAllowed bool   `json:"sell_intent_allowed"`
	PersistSellPlans  bool   `json:"persist_sell_plans"`
	WritesTradePlan   bool   `json:"writes_trade_plan"`
	Disclaimer        string `json:"disclaimer"`
	DataSourceNote    string `json:"data_source_note"`
}

// ExitObservationInput is the already-computed Exit Evaluation facts.
// Thresholds are not re-derived here.
type ExitObservationInput struct {
	State            string
	ReasonCodes      []string
	Summary          string
	UnrealizedReturn *float64
	HoldingDays      int
	PriceStale       bool
}

// ProjectExitObservation projects one observation. Pure: no DB, no orders.
func ProjectExitObservation(in ExitObservationInput) ExitObservation {
	out := ExitObservation{
		NotAnOrder:            true,
		NotLive:               true,
		RequiresManualConfirm: true,
		SellIntentAllowed:     false,
		PersistSellPlans:      false,
		WritesTradePlan:       false,
		Disclaimer:            ExitObservationDisclaimer,
		DataSourceNote:        exitObservationNote,
	}

	state := strings.TrimSpace(in.State)
	known := state == ExitEvalStateNormal || state == ExitEvalStateWatch || state == ExitEvalStateReviewRequired
	if in.PriceStale || returnMissing(in.UnrealizedReturn) || !known {
		out.Class = ExitObserveClassInsufficient
		out.Label = "数据不足"
		out.Reason = insufficientObservationReason(in, state, known)
		return out
	}

	switch state {
	case ExitEvalStateReviewRequired:
		out.Class = ExitObserveClassFlatten
		out.Label = "清仓观察"
		out.SellIntentAllowed = true
	case ExitEvalStateWatch:
		out.Class = ExitObserveClassReduce
		out.Label = "减仓观察"
		out.SellIntentAllowed = true
	default:
		out.Class = ExitObserveClassHold
		out.Label = "持有观察"
	}
	out.Reason = observationReason(in, state)
	return out
}

func returnMissing(r *float64) bool {
	if r == nil {
		return true
	}
	return math.IsNaN(*r) || math.IsInf(*r, 0)
}

func observationReason(in ExitObservationInput, state string) string {
	if s := strings.TrimSpace(in.Summary); s != "" {
		return s
	}
	return BuildExitReviewSummary(in.HoldingDays, in.UnrealizedReturn, state, in.ReasonCodes)
}

func insufficientObservationReason(in ExitObservationInput, state string, known bool) string {
	if in.PriceStale {
		return "数据过期，退出观察失败关闭"
	}
	if returnMissing(in.UnrealizedReturn) {
		if s := strings.TrimSpace(in.Summary); strings.Contains(s, "现价缺失") {
			return s
		}
		return "现价与收益率缺失，退出观察失败关闭"
	}
	if !known {
		if state == "" {
			return "退出评估状态缺失，失败关闭为数据不足"
		}
		return "退出评估状态未知，失败关闭为数据不足"
	}
	return "退出观察失败关闭为数据不足"
}

func exitObservationPriceStale(h HoldingEvalStockRow) bool {
	if h.Explanation != nil && h.Explanation.Freshness.PriceStatus == EvalFreshnessStale {
		return true
	}
	if h.HealthScore != nil {
		for _, code := range h.HealthScore.RiskFactors {
			if code == ExplainTagPriceStale {
				return true
			}
		}
	}
	return false
}

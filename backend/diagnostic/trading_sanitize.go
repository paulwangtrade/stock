package diagnostic

import (
	"strings"

	"go-stock/backend/data"
)

// knownBlockReasons mirrors deriveDailyBlockReasons output codes (allowlist only).
var knownBlockReasons = map[string]struct{}{
	"non_weekday":                        {},
	"enable_paper_open_buy_disabled":     {},
	"candidate_pool_empty":               {},
	"trade_plan_missing":                 {},
	"plan_stuck_executing":               {},
	"plan_reconcile_recommended":         {},
	"execution_finished_without_fills":   {},
	"execution_failed":                   {},
	"execution_skipped":                  {},
	"trade_plan_not_ready":               {},
	"all_plan_items_filtered_or_skipped": {},
	"plan_item_executor_not_configured":  {},
}

// TradingStatusSummary is a sanitized daily trading chain projection (no holdings/cash/IDs).
type TradingStatusSummary struct {
	TradeDate           string                    `json:"trade_date"`
	IsWeekday           bool                      `json:"is_weekday"`
	PaperOpenBuyEnabled bool                      `json:"paper_open_buy_enabled"`
	Candidate           TradingCandidateSummary   `json:"candidate"`
	Plan                TradingPlanSummary        `json:"plan"`
	Risk                TradingRiskSummary        `json:"risk"`
	Execution           TradingExecutionSummary   `json:"execution"`
	Paper               TradingPaperSummary       `json:"paper"`
	BlockReasons        []string                  `json:"block_reasons,omitempty"`
	MessageSafe         string                    `json:"message_safe,omitempty"`
}

type TradingCandidateSummary struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type TradingPlanSummary struct {
	Status                string `json:"status"`
	ItemCount             int    `json:"item_count"`
	PendingCount          int    `json:"pending_count"`
	FilledCount           int    `json:"filled_count"`
	SkippedCount          int    `json:"skipped_count"`
	ErrorCount            int    `json:"error_count"`
	ReconcileRecommended  bool   `json:"reconcile_recommended"`
}

type TradingRiskSummary struct {
	Status        string `json:"status"`
	AcceptedCount int    `json:"accepted_count"`
	FilteredCount int    `json:"filtered_count"`
	MarketLevel   int    `json:"market_level"`
}

type TradingExecutionSummary struct {
	Phase              string `json:"phase"`
	Ready              bool   `json:"ready"`
	ExecutorConfigured bool   `json:"executor_configured"`
}

type TradingPaperSummary struct {
	HasAccount       bool `json:"has_account"`
	OrderCount       int  `json:"order_count"`
	FillCount        int  `json:"fill_count"`
	FilledOrderCount int  `json:"filled_order_count"`
}

// SanitizeTradingStatus projects DailyTradingStatus into a diagnostic-safe summary.
func SanitizeTradingStatus(st data.DailyTradingStatus) TradingStatusSummary {
	out := TradingStatusSummary{
		TradeDate:           strings.TrimSpace(st.TradeDate),
		IsWeekday:           st.IsWeekday,
		PaperOpenBuyEnabled: st.EnablePaperOpenBuy,
		Candidate: TradingCandidateSummary{
			Status: strings.TrimSpace(st.Candidate.Status),
			Count:  st.Candidate.Count,
		},
		Plan: TradingPlanSummary{
			Status:               strings.TrimSpace(st.Plan.Status),
			ItemCount:            st.Plan.ItemCount,
			PendingCount:         st.Plan.PendingCount,
			FilledCount:          st.Plan.FilledCount,
			SkippedCount:         st.Plan.SkippedCount,
			ErrorCount:           st.Plan.ErrorCount,
			ReconcileRecommended: st.Plan.ReconcileRecommended,
		},
		Risk: TradingRiskSummary{
			Status:        strings.TrimSpace(st.Risk.Status),
			AcceptedCount: st.Risk.AcceptedCount,
			FilteredCount: st.Risk.FilteredCount,
			MarketLevel:   st.Risk.MarketLevel,
		},
		Execution: TradingExecutionSummary{
			Phase:              strings.TrimSpace(st.Execution.Phase),
			Ready:              st.Execution.Ready,
			ExecutorConfigured: st.Execution.ExecutorConfigured,
		},
		Paper: TradingPaperSummary{
			HasAccount:       st.Paper.HasAccount,
			OrderCount:       st.Paper.OrderCount,
			FillCount:        st.Paper.FillCount,
			FilledOrderCount: st.Paper.FilledOrderCount,
		},
		BlockReasons: filterKnownBlockReasons(st.BlockReasons),
	}
	out.MessageSafe = buildTradingMessageSafe(out)
	return out
}

func filterKnownBlockReasons(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, code := range in {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := knownBlockReasons[code]; !ok {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func buildTradingMessageSafe(st TradingStatusSummary) string {
	if len(st.BlockReasons) > 0 {
		return "blocked: " + st.BlockReasons[0]
	}
	if st.Execution.Ready {
		return "execution=ready"
	}
	if st.Execution.Phase != "" {
		return "execution=" + st.Execution.Phase
	}
	return "observation only"
}

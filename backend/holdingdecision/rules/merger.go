package rules

import (
	"fmt"
	"sort"
	"strings"
)

// MergeHits collapses RuleHits into a HoldingDecisionResult.
// Priority: EXIT > REDUCE > HOLD. Gates ReduceEnabled / ExitEnabled.
// Keeps all reason codes and records conflict_resolution steps.
func MergeHits(symbol string, hits []RuleHit, pol Policy, executableHint bool) HoldingDecisionResult {
	out := HoldingDecisionResult{
		SchemaVersion:    SchemaVersion,
		Symbol:           symbol,
		FinalAction:      ActionHold,
		Action:           ActionHold,
		RuleHits:         append([]RuleHit(nil), hits...),
		ReasonCodes:      nil,
		SuggestOnly:      true,
		ExecutableHint:   executableHint,
		RecordOnly:       true,
		NotAnOrder:       true,
		NotSellTradePlan: true,
		NotExecution:     true,
		NotBuyChain:      true,
		PersistSellPlans: false,
	}

	if len(hits) == 0 {
		out.Explanation = "无规则命中；观察动作 HOLD（suggest_only）"
		out.ReasonCodes = []string{ReasonDefaultHold}
		out.Evidence = map[string]any{"rule_hit_count": 0}
		out.ConflictResolution = []string{"no_hits → HOLD"}
		return out
	}

	sort.SliceStable(out.RuleHits, func(i, j int) bool {
		if out.RuleHits[i].Family != out.RuleHits[j].Family {
			return out.RuleHits[i].Family < out.RuleHits[j].Family
		}
		return out.RuleHits[i].RuleID < out.RuleHits[j].RuleID
	})

	best := ActionHold
	codes := make([]string, 0, len(hits))
	seen := map[string]struct{}{}
	gated := false
	var steps []string
	steps = append(steps, fmt.Sprintf("collected_hits=%d", len(hits)))

	for _, h := range hits {
		cand := normalizeAction(h.ActionCandidate)
		gatedCand, wasGated := applyGates(cand, pol)
		if wasGated {
			gated = true
			steps = append(steps, fmt.Sprintf("%s candidate=%s gated→%s", h.RuleID, cand, gatedCand))
		} else {
			steps = append(steps, fmt.Sprintf("%s candidate=%s", h.RuleID, cand))
		}
		prev := best
		best = maxAction(best, gatedCand)
		if best != prev {
			steps = append(steps, fmt.Sprintf("priority %s → %s (via %s)", prev, best, h.RuleID))
		}
		if c := strings.TrimSpace(h.ReasonCode); c != "" {
			if _, ok := seen[c]; !ok {
				seen[c] = struct{}{}
				codes = append(codes, c)
			}
		}
	}
	if gated {
		if _, ok := seen[ReasonRuleGated]; !ok {
			codes = append(codes, ReasonRuleGated)
		}
		steps = append(steps, "reduce/exit master gates applied")
	}
	if len(codes) == 0 {
		codes = []string{ReasonDefaultHold}
	}
	steps = append(steps, fmt.Sprintf("final_action=%s; reasons_kept=%d", best, len(codes)))

	out.FinalAction = best
	out.Action = best
	out.SuggestOnly = true
	out.ReasonCodes = codes
	out.ConflictResolution = steps
	out.Explanation = explain(best, out.RuleHits, gated)
	if !executableHint && (best == ActionReduce || best == ActionExit) {
		out.Explanation += "；T+1/不可卖 → executable_hint=false"
	}
	return out
}

func normalizeAction(a string) string {
	switch strings.ToUpper(strings.TrimSpace(a)) {
	case ActionExit:
		return ActionExit
	case ActionReduce:
		return ActionReduce
	default:
		return ActionHold
	}
}

func applyGates(cand string, pol Policy) (string, bool) {
	switch cand {
	case ActionExit:
		if pol.ExitEnabled {
			return ActionExit, false
		}
		if pol.ReduceEnabled {
			return ActionReduce, true
		}
		return ActionHold, true
	case ActionReduce:
		if pol.ReduceEnabled {
			return ActionReduce, false
		}
		return ActionHold, true
	default:
		return ActionHold, false
	}
}

func maxAction(a, b string) string {
	rank := func(x string) int {
		switch x {
		case ActionExit:
			return 2
		case ActionReduce:
			return 1
		default:
			return 0
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func explain(final string, hits []RuleHit, gated bool) string {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.RuleID)
	}
	msg := fmt.Sprintf("观察动作 %s；命中规则 [%s]", final, strings.Join(ids, ", "))
	if gated {
		msg += "（部分候选受 Reduce/Exit 总闸降级）"
	}
	msg += "；suggest_only；非下单、不 persist"
	return msg
}

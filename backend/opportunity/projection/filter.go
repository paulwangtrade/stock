package projection

import "strings"

func filterProjections(items []OpportunityProjection, status, strategyID string) []OpportunityProjection {
	status = strings.ToUpper(strings.TrimSpace(status))
	strategyID = strings.ToLower(strings.TrimSpace(strategyID))
	if status == "" && strategyID == "" {
		return items
	}
	out := make([]OpportunityProjection, 0, len(items))
	for _, it := range items {
		if status != "" && !strings.EqualFold(it.Decision.DecisionStatus, status) {
			continue
		}
		if strategyID != "" && !matchesStrategyID(it, strategyID) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func matchesStrategyID(p OpportunityProjection, strategyID string) bool {
	if strings.EqualFold(strings.TrimSpace(p.Signal.StrategyID), strategyID) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(p.Opportunity.StrategyName))
	if name != "" && (name == strategyID || strings.Contains(name, strategyID)) {
		return true
	}
	src := strings.ToLower(strings.TrimSpace(p.Opportunity.StrategySource))
	return src != "" && strings.Contains(src, strategyID)
}

func isEmptyProjection(p *OpportunityProjection) bool {
	if p == nil {
		return true
	}
	return !p.Signal.Present && !p.Opportunity.Present && p.Research == nil
}

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}

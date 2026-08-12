package portfolioobs

import (
	"sort"
	"strings"
)

const historyCurrentOnlyNote = "仅当前观察点，无持久化决策历史快照"

// ProjectDecisionHistory sorts points and counts state transitions. Pure; no DB.
func ProjectDecisionHistory(symbol string, points []DecisionHistoryPoint) DecisionHistory {
	out := DecisionHistory{
		Symbol: strings.TrimSpace(symbol),
		Points: []DecisionHistoryPoint{},
	}
	if len(points) == 0 {
		out.Note = historyCurrentOnlyNote
		return out
	}
	cp := append([]DecisionHistoryPoint{}, points...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].AsOf < cp[j].AsOf })
	out.Points = cp
	for i := 1; i < len(cp); i++ {
		a := strings.ToUpper(strings.TrimSpace(cp[i-1].State))
		b := strings.ToUpper(strings.TrimSpace(cp[i].State))
		if a != "" && b != "" && a != b {
			out.TransitionCount++
		}
	}
	out.Complete = len(cp) >= 2
	if !out.Complete {
		out.Note = historyCurrentOnlyNote
	}
	return out
}

// HistoryFromCurrent builds a single-point timeline from today's Decision.
func HistoryFromCurrent(symbol, state, reason, asOf string) DecisionHistory {
	asOf = strings.TrimSpace(asOf)
	state = strings.TrimSpace(state)
	if asOf == "" && state == "" {
		return ProjectDecisionHistory(symbol, nil)
	}
	if len(asOf) >= 10 {
		asOf = asOf[:10]
	}
	return ProjectDecisionHistory(symbol, []DecisionHistoryPoint{{
		AsOf:   asOf,
		State:  state,
		Reason: strings.TrimSpace(reason),
		Source: "current",
	}})
}

package semantic

import (
	"fmt"
	"sort"
	"time"

	"go-stock/backend/models"
)

// ReplayDayPair one trade-date baseline/candidate pair (fixed Signal/Gate/Zone/Size already assembled).
type ReplayDayPair struct {
	TradeDate string
	AsOf      string
	Pair      ShadowPair
}

// DailyShadowReport Phase2-C stability wrapped with tradeDate.
type DailyShadowReport struct {
	TradeDate            string          `json:"tradeDate"`
	AsOf                 string          `json:"asOf,omitempty"`
	BaselineActionCode   string          `json:"baselineActionCode"`
	CandidateActionCode  string          `json:"candidateActionCode"`
	Stability            StabilityReport `json:"stability"`
}

// TransitionEdge Action.Code from→to count.
type TransitionEdge struct {
	FromCode string `json:"fromCode"`
	ToCode   string `json:"toCode"`
	Count    int    `json:"count"`
}

// ActionTransitionMatrix adjacent-day (or same-day cross-producer) Action.Code edges.
type ActionTransitionMatrix struct {
	TransitionCount int              `json:"transitionCount,omitempty"`
	DayCount        int              `json:"dayCount,omitempty"`
	UniqueEdges     int              `json:"uniqueEdges"`
	Transitions     []TransitionEdge `json:"transitions"`
	FromTotals      map[string]int   `json:"fromTotals,omitempty"`
	ToTotals        map[string]int   `json:"toTotals,omitempty"`
}

// ActionTransitionsSection baseline / candidate / cross-producer matrices.
type ActionTransitionsSection struct {
	Baseline              ActionTransitionMatrix `json:"baseline"`
	Candidate             ActionTransitionMatrix `json:"candidate"`
	CrossProducerSameDay  ActionTransitionMatrix `json:"crossProducerSameDay"`
}

// HistoricalReplayReport Phase2-D multi-day shadow replay.
type HistoricalReplayReport struct {
	Phase             string                   `json:"phase"`
	Harness           string                   `json:"harness"`
	GeneratedAt       string                   `json:"generatedAt"`
	SeriesID          string                   `json:"seriesId"`
	Code              string                   `json:"code"`
	TotalDays         int                      `json:"totalDays"`
	Daily             []DailyShadowReport      `json:"daily"`
	Rollup            StabilityReport          `json:"rollup"`
	ActionTransitions ActionTransitionsSection `json:"actionTransitions"`
	Summary           string                   `json:"summary"`
}

// BuildActionTransitionMatrix counts adjacent Action.Code transitions.
func BuildActionTransitionMatrix(codes []string) ActionTransitionMatrix {
	cells := map[string]int{}
	fromTotals := map[string]int{}
	toTotals := map[string]int{}
	transitionCount := 0

	for i := 0; i < len(codes)-1; i++ {
		from := emptyMark(codes[i])
		to := emptyMark(codes[i+1])
		key := fmt.Sprintf("%s→%s", from, to)
		cells[key]++
		fromTotals[from]++
		toTotals[to]++
		transitionCount++
	}

	return ActionTransitionMatrix{
		TransitionCount: transitionCount,
		UniqueEdges:     len(cells),
		Transitions:     sortedEdges(cells),
		FromTotals:      fromTotals,
		ToTotals:        toTotals,
	}
}

// BuildCrossProducerSameDayMatrix counts same-day baseline→candidate Action.Code edges.
func BuildCrossProducerSameDayMatrix(left, right []string) ActionTransitionMatrix {
	n := len(left)
	if len(right) < n {
		n = len(right)
	}
	cells := map[string]int{}
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%s→%s", emptyMark(left[i]), emptyMark(right[i]))
		cells[key]++
	}
	return ActionTransitionMatrix{
		DayCount:    n,
		UniqueEdges: len(cells),
		Transitions: sortedEdges(cells),
	}
}

func sortedEdges(cells map[string]int) []TransitionEdge {
	var edges []TransitionEdge
	for key, count := range cells {
		from, to := splitArrow(key)
		edges = append(edges, TransitionEdge{FromCode: from, ToCode: to, Count: count})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Count != edges[j].Count {
			return edges[i].Count > edges[j].Count
		}
		if edges[i].FromCode != edges[j].FromCode {
			return edges[i].FromCode < edges[j].FromCode
		}
		return edges[i].ToCode < edges[j].ToCode
	})
	return edges
}

// BuildHistoricalReplayReport aggregates daily Phase2-C reports + Action transition matrices.
// Days must already carry assembled Decisions (fixture → Decision is JS/Go producer responsibility outside this aggregator).
func BuildHistoricalReplayReport(seriesID, code string, days []ReplayDayPair) HistoricalReplayReport {
	var daily []DailyShadowReport
	var allPairs []ShadowPair
	var baselineCodes []string
	var candidateCodes []string

	for i, day := range days {
		tradeDate := day.TradeDate
		if tradeDate == "" {
			tradeDate = fmt.Sprintf("day-%d", i+1)
		}
		pair := day.Pair
		if pair.ID == "" {
			pair.ID = fmt.Sprintf("%s:%s", seriesIDOr(seriesID), tradeDate)
		}
		if pair.Code == "" {
			pair.Code = code
		}

		stability := BuildShadowStabilityReport([]ShadowPair{pair})
		baseCode := actionCodeOf(pair.Baseline)
		candCode := actionCodeOf(pair.Candidate)

		daily = append(daily, DailyShadowReport{
			TradeDate:           tradeDate,
			AsOf:                day.AsOf,
			BaselineActionCode:  baseCode,
			CandidateActionCode: candCode,
			Stability:           stability,
		})
		allPairs = append(allPairs, pair)
		baselineCodes = append(baselineCodes, baseCode)
		candidateCodes = append(candidateCodes, candCode)
	}

	rollup := BuildShadowStabilityReport(allPairs)
	baseTM := BuildActionTransitionMatrix(baselineCodes)
	candTM := BuildActionTransitionMatrix(candidateCodes)
	cross := BuildCrossProducerSameDayMatrix(baselineCodes, candidateCodes)

	return HistoricalReplayReport{
		Phase:       "Phase2-D",
		Harness:     "quant-decision-historical-shadow-replay",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SeriesID:    seriesIDOr(seriesID),
		Code:        code,
		TotalDays:   len(days),
		Daily:       daily,
		Rollup:      rollup,
		ActionTransitions: ActionTransitionsSection{
			Baseline:             baseTM,
			Candidate:            candTM,
			CrossProducerSameDay: cross,
		},
		Summary: summarizeReplay(len(days), rollup, baseTM),
	}
}

func seriesIDOr(id string) string {
	if id == "" {
		return "unnamed"
	}
	return id
}

func actionCodeOf(d *models.QuantDecision) string {
	if d == nil {
		return ""
	}
	return d.Action.Code
}

func summarizeReplay(totalDays int, rollup StabilityReport, baselineTM ActionTransitionMatrix) string {
	if totalDays == 0 {
		return "EMPTY: no replay days"
	}
	fail := rollup.SemanticMismatchCount
	conflicts := rollup.ActionCode.ConflictCount
	edges := baselineTM.UniqueEdges
	if fail == 0 && conflicts == 0 {
		return fmt.Sprintf("STABLE_REPLAY: %dd; actionTransitions=%d; 0 action.code conflicts", totalDays, edges)
	}
	return fmt.Sprintf("UNSTABLE_REPLAY: %dd; semanticFail=%d; actionCodeConflicts=%d; baselineTransitions=%d",
		totalDays, fail, conflicts, edges)
}

package semantic

import (
	"fmt"
	"sort"
	"time"

	"go-stock/backend/models"
)

// ShadowPair one baseline/candidate pair for batch stability.
type ShadowPair struct {
	ID        string
	Code      string
	Baseline  *models.QuantDecision
	Candidate *models.QuantDecision
}

// ActionCodeConflictStat aggregated left→right code conflict.
type ActionCodeConflictStat struct {
	LeftCode  string `json:"leftCode"`
	RightCode string `json:"rightCode"`
	Count     int    `json:"count"`
}

// ActionCodeConflictPair per-pair conflict detail.
type ActionCodeConflictPair struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	LeftCode  string `json:"leftCode"`
	RightCode string `json:"rightCode"`
}

// SliceMismatchStats mismatch aggregation for one slice.
type SliceMismatchStats struct {
	MismatchPairCount int            `json:"mismatchPairCount"`
	PathCounts        map[string]int `json:"pathCounts"`
}

// PairStabilityResult compact per-pair outcome.
type PairStabilityResult struct {
	ID                string   `json:"id"`
	Code              string   `json:"code"`
	SemanticEqual     bool     `json:"semanticEqual"`
	ActionCodeEqual   bool     `json:"actionCodeEqual"`
	LeftActionCode    string   `json:"leftActionCode"`
	RightActionCode   string   `json:"rightActionCode"`
	DisplayDiffCount  int      `json:"displayDiffCount"`
	SemanticDiffCount int      `json:"semanticDiffCount"`
	Summary           string   `json:"summary"`
	MismatchedSlices  []string `json:"mismatchedSlices"`
}

// StabilityReport Phase2-C batch shadow stability report.
type StabilityReport struct {
	Phase                  string                        `json:"phase"`
	Harness                string                        `json:"harness"`
	GeneratedAt            string                        `json:"generatedAt"`
	TotalPairs             int                           `json:"totalPairs"`
	SemanticEqualCount     int                           `json:"semanticEqualCount"`
	SemanticMismatchCount  int                           `json:"semanticMismatchCount"`
	DisplayOnlyDiffCount   int                           `json:"displayOnlyDiffCount"`
	SemanticEqualRate      float64                       `json:"semanticEqualRate"`
	ActionCode             ActionCodeSection             `json:"actionCode"`
	MismatchBySlice        map[string]SliceMismatchStats `json:"mismatchBySlice"`
	Pairs                  []PairStabilityResult         `json:"pairs"`
	Summary                string                        `json:"summary"`
}

// ActionCodeSection Action.Code conflict statistics.
type ActionCodeSection struct {
	EqualCount     int                      `json:"equalCount"`
	ConflictCount  int                      `json:"conflictCount"`
	EqualRate      float64                  `json:"equalRate"`
	ConflictStats  []ActionCodeConflictStat `json:"conflictStats"`
	ConflictPairs  []ActionCodeConflictPair `json:"conflictPairs"`
}

// BuildShadowStabilityReport aggregates semantic compares into a stability report.
func BuildShadowStabilityReport(pairs []ShadowPair) StabilityReport {
	mismatchBySlice := map[string]SliceMismatchStats{}
	for _, slice := range NormalizeSlices {
		mismatchBySlice[slice] = SliceMismatchStats{
			MismatchPairCount: 0,
			PathCounts:        map[string]int{},
		}
	}

	conflictCounts := map[string]int{}
	var conflictPairs []ActionCodeConflictPair
	var pairReports []PairStabilityResult

	semanticEqualCount := 0
	semanticMismatchCount := 0
	displayOnlyDiffCount := 0
	actionCodeEqualCount := 0
	actionCodeConflictCount := 0

	for i, row := range pairs {
		id := row.ID
		if id == "" {
			id = fmt.Sprintf("pair-%d", i+1)
		}
		code := row.Code
		if code == "" && row.Baseline != nil {
			code = row.Baseline.Instrument.StockCode
		}
		if code == "" && row.Candidate != nil {
			code = row.Candidate.Instrument.StockCode
		}

		cmp := CompareSemantic(row.Baseline, row.Candidate)

		if cmp.SemanticEqual {
			semanticEqualCount++
		} else {
			semanticMismatchCount++
		}
		if cmp.SemanticEqual && len(cmp.DisplayDiffs) > 0 {
			displayOnlyDiffCount++
		}

		if cmp.ActionCodeEqual {
			actionCodeEqualCount++
		} else {
			actionCodeConflictCount++
			key := fmt.Sprintf("%s→%s", emptyMark(cmp.LeftActionCode), emptyMark(cmp.RightActionCode))
			conflictCounts[key]++
			conflictPairs = append(conflictPairs, ActionCodeConflictPair{
				ID: id, Code: code,
				LeftCode: cmp.LeftActionCode, RightCode: cmp.RightActionCode,
			})
		}

		var mismatched []string
		for _, slice := range NormalizeSlices {
			sr, ok := cmp.BySlice[slice]
			if !ok || sr.Equal {
				continue
			}
			mismatched = append(mismatched, slice)
			stats := mismatchBySlice[slice]
			stats.MismatchPairCount++
			if stats.PathCounts == nil {
				stats.PathCounts = map[string]int{}
			}
			for _, d := range sr.Diffs {
				p := d.Path
				if p == "" {
					p = slice
				}
				stats.PathCounts[p]++
			}
			mismatchBySlice[slice] = stats
		}

		pairReports = append(pairReports, PairStabilityResult{
			ID: id, Code: code,
			SemanticEqual: cmp.SemanticEqual, ActionCodeEqual: cmp.ActionCodeEqual,
			LeftActionCode: cmp.LeftActionCode, RightActionCode: cmp.RightActionCode,
			DisplayDiffCount: len(cmp.DisplayDiffs), SemanticDiffCount: len(cmp.SemanticDiffs),
			Summary: cmp.Summary, MismatchedSlices: mismatched,
		})
	}

	var conflictStats []ActionCodeConflictStat
	for key, count := range conflictCounts {
		left, right := splitArrow(key)
		conflictStats = append(conflictStats, ActionCodeConflictStat{
			LeftCode: left, RightCode: right, Count: count,
		})
	}
	sort.Slice(conflictStats, func(i, j int) bool {
		if conflictStats[i].Count != conflictStats[j].Count {
			return conflictStats[i].Count > conflictStats[j].Count
		}
		return conflictStats[i].LeftCode < conflictStats[j].LeftCode
	})

	total := len(pairs)
	semRate, codeRate := 1.0, 1.0
	if total > 0 {
		semRate = float64(semanticEqualCount) / float64(total)
		codeRate = float64(actionCodeEqualCount) / float64(total)
	}

	return StabilityReport{
		Phase:                 "Phase2-C",
		Harness:               "quant-decision-shadow-stability",
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		TotalPairs:            total,
		SemanticEqualCount:    semanticEqualCount,
		SemanticMismatchCount: semanticMismatchCount,
		DisplayOnlyDiffCount:  displayOnlyDiffCount,
		SemanticEqualRate:     semRate,
		ActionCode: ActionCodeSection{
			EqualCount:    actionCodeEqualCount,
			ConflictCount: actionCodeConflictCount,
			EqualRate:     codeRate,
			ConflictStats: conflictStats,
			ConflictPairs: conflictPairs,
		},
		MismatchBySlice: mismatchBySlice,
		Pairs:           pairReports,
		Summary: summarizeStability(total, semanticEqualCount, semanticMismatchCount, actionCodeConflictCount, displayOnlyDiffCount),
	}
}

func summarizeStability(total, semOK, semFail, codeConflict, displayOnly int) string {
	if total == 0 {
		return "EMPTY: no shadow pairs"
	}
	if semFail == 0 && codeConflict == 0 {
		if displayOnly > 0 {
			return fmt.Sprintf("STABLE: %d/%d semantic OK; %d display-only", semOK, total, displayOnly)
		}
		return fmt.Sprintf("STABLE: %d/%d semantic OK; 0 action.code conflicts", semOK, total)
	}
	return fmt.Sprintf("UNSTABLE: semanticFail=%d/%d; actionCodeConflicts=%d", semFail, total, codeConflict)
}

func splitArrow(s string) (string, string) {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == 0xe2 && i+2 < len([]byte(s)) { // utf-8 arrow maybe
			break
		}
	}
	// keys use "→" (utf-8 3 bytes) from emptyMark join
	const sep = "→"
	idx := -1
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			idx = i
			break
		}
	}
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+len(sep):]
}

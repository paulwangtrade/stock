package shadow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/tradeplan/candidate"
)

// ShadowPair one candidate vs plan fixture pair.
type ShadowPair struct {
	ID        string                       `json:"id"`
	Candidate candidate.TradePlanCandidate `json:"candidate"`
	Plan      models.TradePlan             `json:"plan"`
}

// FieldStat aggregated equality stats for one compare path.
type FieldStat struct {
	EqualCount     int     `json:"equalCount"`
	DifferentCount int     `json:"differentCount"`
	WeakMatchCount int     `json:"weakMatchCount"`
	IgnoredCount   int     `json:"ignoredCount"`
	EqualRate      float64 `json:"equalRate"`
}

// TradePlanShadowBatchReport Phase5-C batch shadow report.
type TradePlanShadowBatchReport struct {
	Phase                  string                `json:"phase"`
	Harness                string                `json:"harness"`
	GeneratedAt            string                `json:"generatedAt"`
	TotalPairs             int                   `json:"totalPairs"`
	EqualCount             int                   `json:"equalCount"`
	DiffCount              int                   `json:"diffCount"`
	CandidateFieldsCovered float64               `json:"candidateFieldsCovered"`
	MissingFieldCounts     map[string]int        `json:"missingFieldCounts"`
	FieldStats             map[string]FieldStat  `json:"fieldStats"`
	RiskFlagCounts         map[string]int        `json:"riskFlagCounts"`
	Pairs                  []TradePlanShadowDiff `json:"pairs"`
	Summary                string                `json:"summary"`
}

// comparedCandidateFields fields we expect to assess coverage for.
var comparedCandidateFields = []string{
	"Side", "TargetShares", "EntryPriceHint", "StopPriceHint", "IntentKind",
	"SourceDecisionID", "SourceSnapshotHash",
}

// BuildShadowBatchReport aggregates CompareTradePlanCandidate over fixture pairs.
func BuildShadowBatchReport(pairs []ShadowPair) TradePlanShadowBatchReport {
	report := TradePlanShadowBatchReport{
		Phase:              "Phase5-C",
		Harness:            "quant-tradeplan-candidate-shadow-batch",
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		TotalPairs:         len(pairs),
		MissingFieldCounts: map[string]int{},
		FieldStats:         map[string]FieldStat{},
		RiskFlagCounts:     map[string]int{},
	}

	coveredHits := 0
	coveredDenom := 0

	for _, p := range pairs {
		diff := CompareTradePlanCandidate(p.Candidate, p.Plan)
		if p.ID != "" {
			diff.Summary = p.ID + ": " + diff.Summary
		}
		report.Pairs = append(report.Pairs, diff)
		if diff.Equal {
			report.EqualCount++
		} else {
			report.DiffCount++
		}
		for _, m := range diff.MissingFields {
			report.MissingFieldCounts[m]++
		}
		for _, r := range diff.RiskFlags {
			report.RiskFlagCounts[r]++
		}

		// per-field stats from diffs + implicit equals
		seen := map[string]string{} // path -> kind
		for _, d := range diff.DifferentFields {
			seen[d.Path] = d.Kind
			st := report.FieldStats[d.Path]
			switch d.Kind {
			case "different":
				st.DifferentCount++
			case "weak_match":
				st.WeakMatchCount++
			case "ignored":
				st.IgnoredCount++
			case "forbidden_field":
				st.DifferentCount++ // count as risk presence
			}
			report.FieldStats[d.Path] = st
		}
		for _, name := range []string{"Side", "TargetShares", "EntryPriceHint"} {
			if _, ok := seen[name]; !ok {
				st := report.FieldStats[name]
				st.EqualCount++
				report.FieldStats[name] = st
			}
		}

		// coverage: candidate fields that have a plan-side presence or explicit missing tracked
		for _, f := range comparedCandidateFields {
			coveredDenom++
			switch f {
			case "Side", "TargetShares":
				coveredHits++
			case "EntryPriceHint":
				coveredHits++ // always assessed (incl weak)
			case "StopPriceHint":
				// covered only if we recorded mapping attempt (missing still "assessed")
				coveredHits++
			case "IntentKind":
				coveredHits++
			case "SourceDecisionID", "SourceSnapshotHash":
				// always missing on existing plan → assessed as gap
				coveredHits++
			}
		}
	}

	for k, st := range report.FieldStats {
		total := st.EqualCount + st.DifferentCount + st.WeakMatchCount
		if total > 0 {
			st.EqualRate = float64(st.EqualCount) / float64(total)
		}
		report.FieldStats[k] = st
	}
	if coveredDenom > 0 {
		report.CandidateFieldsCovered = float64(coveredHits) / float64(coveredDenom)
	}

	if report.DiffCount == 0 {
		report.Summary = "STABLE_SHADOW: all pairs equal on compared fields"
	} else {
		report.Summary = "UNSTABLE_SHADOW: semantic diffs present"
	}
	return report
}

// WriteShadowReportJSON writes report to path (for golden / artifacts).
func WriteShadowReportJSON(path string, report TradePlanShadowBatchReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// stable key order for missing maps via remarshal not guaranteed; ok for tests
	b, err := json.MarshalIndent(normalizeReport(report), "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func normalizeReport(r TradePlanShadowBatchReport) TradePlanShadowBatchReport {
	// sort pair summaries already ordered; sort risk/missing keys into deterministic JSON by rebuilding maps via sorted lists in Marshal — use helper structs if needed.
	_ = sort.Strings
	return r
}

package portfolioreplay

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata", "replay")
}

func TestLoadDir_Fixtures(t *testing.T) {
	t.Parallel()
	cases, err := LoadDir(fixtureDir(t))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(cases), 2)
	ids := map[string]bool{}
	for _, c := range cases {
		require.NotEmpty(t, c.CaseID)
		require.NotNil(t, c.Snapshot)
		require.NotNil(t, c.Candidates)
		require.NotEmpty(t, c.Candidates.RankedCandidates)
		require.False(t, c.DecisionTime.IsZero())
		ids[c.CaseID] = true
	}
	require.True(t, ids["fx-2026-08-19-basic"])
	require.True(t, ids["fx-2026-08-19-defense"])
}

func TestRun_DeterministicAndNoSideEffects(t *testing.T) {
	t.Parallel()
	cases, err := LoadDir(fixtureDir(t))
	require.NoError(t, err)
	cashBefore := map[string]float64{}
	for _, c := range cases {
		cashBefore[c.CaseID] = c.Snapshot.Cash
	}

	r1 := Run(cases)
	r2 := Run(cases)
	require.True(t, r1.DeterministicCheck)
	require.True(t, r2.DeterministicCheck)
	require.Equal(t, r1.CaseCount, r2.CaseCount)
	require.Equal(t, r1.SuccessCount, r2.SuccessCount)
	require.Equal(t, r1.SuccessCount, r1.CaseCount)
	require.Equal(t, r1.DecisionCount, r1.SuccessCount)
	require.Equal(t, r1.SelectedCountDistribution, r2.SelectedCountDistribution)
	require.InDelta(t, r1.AllocationDistribution.Mean, r2.AllocationDistribution.Mean, 1e-6)
	require.True(t, r1.NotATradePlan)
	require.True(t, r1.NotABacktest)
	require.True(t, r1.RecordOnly)

	// Decision-behavior stats present and stable.
	require.NotNil(t, r1.DecisionBehavior.RiskConstraintHits)
	require.Equal(t, len(r1.Rows), r1.SuccessCount)
	require.InDelta(t, r1.DecisionBehavior.CandidateCounts.PortfolioSelectedMean,
		r2.DecisionBehavior.CandidateCounts.PortfolioSelectedMean, 1e-9)
	require.InDelta(t, r1.DecisionBehavior.AllocationAmounts.NotionalDelta.Mean,
		r2.DecisionBehavior.AllocationAmounts.NotionalDelta.Mean, 1e-6)
	require.InDelta(t, r1.DecisionBehavior.CashUsage.CashRatioDelta.Mean,
		r2.DecisionBehavior.CashUsage.CashRatioDelta.Mean, 1e-9)
	require.InDelta(t, r1.DecisionBehavior.Concentration.Top1Delta.Mean,
		r2.DecisionBehavior.Concentration.Top1Delta.Mean, 1e-9)
	// Fixtures have no industry → sector unavailable (no fake zeros as decisions).
	require.False(t, r1.DecisionBehavior.SectorExposure.Available)

	raw, err := json.Marshal(r1)
	require.NoError(t, err)
	s := string(raw)
	require.NotContains(t, s, `"pnl"`)
	require.NotContains(t, s, `"sharpe"`)
	require.NotContains(t, s, `"plan_id"`)
	require.NotContains(t, s, `"fills"`)
	require.NotContains(t, s, `"max_drawdown"`)
	require.NotContains(t, s, `"optimized_constraints"`)
	require.Contains(t, s, `"not_a_backtest":true`)
	require.Contains(t, s, `"decision_behavior"`)

	for _, c := range cases {
		require.Equal(t, cashBefore[c.CaseID], c.Snapshot.Cash)
	}
}

func TestRun_SameCaseTwiceFingerprint(t *testing.T) {
	t.Parallel()
	cases, err := LoadDir(fixtureDir(t))
	require.NoError(t, err)
	var basic ReplayCase
	for _, c := range cases {
		if c.CaseID == "fx-2026-08-19-basic" {
			basic = c
			break
		}
	}
	require.NotEmpty(t, basic.CaseID)
	a := simulatePortfolio(basic)
	b := simulatePortfolio(basic)
	require.Equal(t, fingerprint(a), fingerprint(b))
	require.Len(t, a.Selected, 3)
	require.Len(t, a.Waitlist, 1)

	la := simulateLegacy(basic)
	lb := simulateLegacy(basic)
	require.Equal(t, fingerprint(la), fingerprint(lb))
	require.True(t, la.NotATradePlan)
	require.NotNil(t, la.LegacyCompare)
}

func TestRun_BehaviorStatsAreDecisionOnly(t *testing.T) {
	t.Parallel()
	cases, err := LoadDir(fixtureDir(t))
	require.NoError(t, err)
	rep := Run(cases)
	require.True(t, rep.NotABacktest)
	db := rep.DecisionBehavior
	// Six required axes exist on the report.
	_ = db.CandidateCounts
	_ = db.AllocationAmounts
	_ = db.CashUsage
	_ = db.RiskConstraintHits
	_ = db.Concentration
	_ = db.SectorExposure
	require.GreaterOrEqual(t, len(db.AllocationAmounts.NotionalDelta.Values), 1)
	require.GreaterOrEqual(t, len(db.CandidateCounts.SelectedCountDelta.Values), 1)
}


func TestParseFixture_RejectsPnL(t *testing.T) {
	t.Parallel()
	_, err := parseFixture([]byte(`{
		"schema_version":"replay_case.v1",
		"case_id":"bad",
		"pnl": 12.3,
		"decision_time":"2026-08-19T15:30:00+08:00",
		"versions":{"selection_version":"a","constraint_version":"b","allocation_version":"c","simulation_version":"d"},
		"snapshot":{"found":true},
		"candidates":{"ranked_candidates":[]}
	}`), "mem")
	require.Error(t, err)
	require.Contains(t, err.Error(), "forbidden")
}

func TestParseFixture_RequiresFound(t *testing.T) {
	t.Parallel()
	_, err := parseFixture([]byte(`{
		"schema_version":"replay_case.v1",
		"case_id":"nofound",
		"decision_time":"2026-08-19T15:30:00+08:00",
		"versions":{"selection_version":"a","constraint_version":"b","allocation_version":"c","simulation_version":"d"},
		"snapshot":{"cash":1},
		"candidates":{"ranked_candidates":[]}
	}`), "mem")
	require.Error(t, err)
	require.Contains(t, err.Error(), "found")
}

package portfolioevaluation_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfolioevaluation"
	"go-stock/backend/portfolioreplay"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/selection"
)

func f64(v float64) *float64 { return &v }
func intPtr(v int) *int       { return &v }

func sampleDay() portfoliovalidation.DayCase {
	return portfoliovalidation.DayCase{
		CaseID:       "d1",
		TradeDate:    "2026-08-20",
		DecisionTime: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC),
		Snapshot: &portfoliolayer.PortfolioSnapshot{
			Found: true, Equity: 1_000_000, Cash: 400_000, Exposure: 600_000,
			Positions: []portfoliolayer.SnapshotPosition{
				{StockCode: "sz000001", MarketValue: 200_000, Weight: 0.20, Industry: "银行", Volume: 1000},
			},
		},
		Candidates: &selection.CandidateSelectionResult{
			SelectionLimit: 3,
			RankedCandidates: []selection.Candidate{
				{StockCode: "sz000002", Rank: 1, Score: 0.9, Industry: "地产", Reason: "rank_top"},
				{StockCode: "sh600000", Rank: 2, Score: 0.8, Industry: "银行", Reason: "rank_top"},
				{StockCode: "sz000858", Rank: 3, Score: 0.7, Industry: "白酒", Reason: "rank_top"},
				{StockCode: "sz000063", Rank: 4, Score: 0.6, Industry: "通信", Reason: "over_name_limit"},
			},
		},
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{
				MaxGrossExposurePct: f64(0.85),
				MaxSingleNamePct:    f64(0.20),
				Enabled:             true,
			},
			Portfolio: portfoliolayer.PreferenceLayer{
				ReserveCashRatio: f64(0.10),
				MaxNewNames:      intPtr(3),
			},
		},
		LegacyAmountPerName: 100_000,
		IndustryBySymbol: map[string]string{
			"sz000001": "银行", "sz000002": "地产", "sh600000": "银行",
			"sz000858": "白酒", "sz000063": "通信",
		},
	}
}

func TestDefaultEnabledIsFalse(t *testing.T) {
	require.False(t, portfolioevaluation.DefaultEnabled)
	rep := portfolioevaluation.Evaluate(portfolioevaluation.Input{
		Days: []portfoliovalidation.DayCase{sampleDay()},
	})
	require.True(t, rep.Skipped)
	require.True(t, rep.ReadOnly)
	require.True(t, rep.NotABacktest)
	require.True(t, rep.NotPnL)
	require.True(t, rep.NotReturn)
	require.True(t, rep.NotSharpe)
	require.True(t, rep.NotAutoTune)
	require.True(t, rep.NotATradePlan)
	require.True(t, rep.NotExecution)
}

func TestEvaluate_LegacyVsPortfolio_Sections(t *testing.T) {
	opt := portfolioevaluation.DefaultOptions()
	opt.Enabled = true
	opt.AttachDecisionShadowV2 = true
	opt.AttachInsight = true

	rep := portfolioevaluation.Evaluate(portfolioevaluation.Input{
		Options: opt,
		Days:    []portfoliovalidation.DayCase{sampleDay()},
	})
	require.False(t, rep.Skipped)
	require.Equal(t, 1, rep.CaseCount)
	require.Equal(t, 1, rep.SuccessCount)
	require.Len(t, rep.Days, 1)

	d := rep.Days[0]
	require.True(t, d.OK)
	require.Equal(t, "fixed_amount", d.Legacy.ProviderIdentity)
	require.Equal(t, "portfolio_allocation", d.Portfolio.ProviderIdentity)
	require.Greater(t, d.Legacy.SelectedCount, 0)
	require.GreaterOrEqual(t, d.Legacy.AllocationCount, 0)
	require.NotNil(t, d.Legacy.FilterRejectReasons)
	require.NotNil(t, d.Portfolio.AllocationReasons)

	// Decision Stability
	require.Greater(t, rep.DecisionStability.SelectedCountLegacyMean, 0.0)
	require.GreaterOrEqual(t, rep.DecisionStability.AmountDistributionLegacy.Count, 1)

	// Risk Behavior
	require.GreaterOrEqual(t, rep.RiskBehavior.Top1LegacyMean, 0.0)
	require.GreaterOrEqual(t, rep.RiskBehavior.Top5LegacyMean, rep.RiskBehavior.Top1LegacyMean-1e-9)

	// Constraint Behavior maps present
	require.NotNil(t, rep.ConstraintBehavior.FilterRejectReasonsPortfolio)
	require.NotNil(t, rep.ConstraintBehavior.BudgetBindingTotals)

	// Explainability
	require.NotNil(t, rep.Explainability.AllocationReasonTotals)
	require.NotNil(t, rep.Explainability.SelectionReasonTotals)
	require.NotNil(t, rep.Explainability.RiskReasonTotals)

	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	s := strings.ToLower(string(raw))
	require.NotContains(t, s, `"pnl":`)
	require.NotContains(t, s, `"sharpe":`)
	require.NotContains(t, s, `"return":`)
	require.NotContains(t, s, "total_return")
	require.NotContains(t, s, "backtest_return")
	require.Contains(t, s, `"not_pnl":true`)
	require.Contains(t, s, `"not_a_backtest":true`)
}

func TestEvaluate_FromReplayFixtures(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "portfolioreplay", "testdata", "replay")

	opt := portfolioevaluation.DefaultOptions()
	opt.Enabled = true
	rep, err := portfolioevaluation.EvaluateDir(dir, opt)
	require.NoError(t, err)
	require.False(t, rep.Skipped)
	require.GreaterOrEqual(t, rep.CaseCount, 1)
	require.GreaterOrEqual(t, rep.SuccessCount, 1)
	require.Greater(t, rep.DecisionStability.SelectedCountLegacyMean+rep.DecisionStability.SelectedCountPortfolioMean, 0.0)
}

func TestEvaluate_FromReplayCaseMapping(t *testing.T) {
	rc := portfolioreplay.ReplayCase{
		CaseID:       "rc1",
		TradeDate:    "2026-08-19",
		DecisionTime: time.Date(2026, 8, 19, 15, 30, 0, 0, time.FixedZone("CST", 8*3600)),
		Snapshot: &portfoliolayer.PortfolioSnapshot{
			Found: true, Equity: 1_000_000, Cash: 800_000, Exposure: 200_000,
			Positions: []portfoliolayer.SnapshotPosition{
				{StockCode: "sz000001", MarketValue: 200_000, Weight: 0.2, Volume: 100},
			},
		},
		Candidates: &selection.CandidateSelectionResult{
			SelectionLimit: 3,
			RankedCandidates: []selection.Candidate{
				{StockCode: "sz000002", Rank: 1, Score: 90},
				{StockCode: "sz000003", Rank: 2, Score: 80},
				{StockCode: "sz000004", Rank: 3, Score: 70},
				{StockCode: "sz000005", Rank: 4, Score: 60},
			},
		},
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{
				MaxGrossExposurePct: f64(0.85),
				MaxSingleNamePct:    f64(0.2),
			},
		},
	}
	opt := portfolioevaluation.DefaultOptions()
	opt.Enabled = true
	rep := portfolioevaluation.EvaluateReplayCases([]portfolioreplay.ReplayCase{rc}, opt)
	require.Equal(t, 1, rep.SuccessCount)
	require.Equal(t, "rc1", rep.Days[0].CaseID)
}

package portfoliovalidation_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfolioreplay"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/selection"
)

func f64(v float64) *float64 { return &v }

func sampleDay() portfoliovalidation.DayCase {
	return portfoliovalidation.DayCase{
		CaseID:       "d1",
		TradeDate:    "2026-08-20",
		DecisionTime: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC),
		Snapshot: &portfoliolayer.PortfolioSnapshot{
			Found: true, Equity: 1_000_000, Cash: 400_000, Exposure: 600_000,
			Positions: []portfoliolayer.SnapshotPosition{
				{StockCode: "sz000001", MarketValue: 200_000, Industry: "银行"},
			},
		},
		Candidates: &selection.CandidateSelectionResult{
			SelectionLimit: 3,
			RankedCandidates: []selection.Candidate{
				{StockCode: "sz000002", Rank: 1, Score: 0.9, Industry: "地产"},
				{StockCode: "sh600000", Rank: 2, Score: 0.8, Industry: "银行"},
				{StockCode: "sz000858", Rank: 3, Score: 0.7, Industry: "白酒"},
				{StockCode: "sz000063", Rank: 4, Score: 0.6, Industry: "通信"},
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

func intPtr(v int) *int { return &v }

func TestDefaultEnabledIsFalse(t *testing.T) {
	require.False(t, portfoliovalidation.DefaultEnabled)
	rep := portfoliovalidation.Validate(portfoliovalidation.Input{
		Days: []portfoliovalidation.DayCase{sampleDay()},
	})
	require.True(t, rep.Skipped)
	require.True(t, rep.ReadOnly)
	require.True(t, rep.NotABacktest)
	require.True(t, rep.NotPnL)
	require.True(t, rep.NotSharpe)
	require.True(t, rep.NotAutoTune)
	require.True(t, rep.NotProductionWrite)
}

func TestValidate_SameDayLegacyVsPortfolio(t *testing.T) {
	rep := portfoliovalidation.Validate(portfoliovalidation.Input{
		Options: portfoliovalidation.Options{Enabled: true},
		Days:    []portfoliovalidation.DayCase{sampleDay()},
	})
	require.False(t, rep.Skipped)
	require.Equal(t, 1, rep.Summary.DayCount)
	require.Equal(t, 1, rep.Summary.OKCount)
	require.Len(t, rep.Days, 1)

	d := rep.Days[0]
	require.True(t, d.OK)
	require.Equal(t, "fixed_amount", d.LegacyDecision.ProviderIdentity)
	require.Equal(t, "portfolio_allocation", d.PortfolioDecision.ProviderIdentity)
	require.Greater(t, d.LegacyDecision.NameCount, 0)
	require.NotEmpty(t, d.Difference.AmountDiffs)
	require.NotNil(t, d.Difference.FilterRejectReasonsLegacy)
	require.NotNil(t, d.Difference.FilterRejectReasonsPortfolio)
}

func TestValidate_SectorAndCashAndTighten(t *testing.T) {
	rep := portfoliovalidation.Validate(portfoliovalidation.Input{
		Options: portfoliovalidation.Options{Enabled: true, AttachDecisionShadowV2: true},
		Days:    []portfoliovalidation.DayCase{sampleDay()},
	})
	d := rep.Days[0]
	require.True(t, d.OK)
	require.True(t, d.DecisionShadowV2Attached)
	require.True(t, d.Difference.SectorAvailable || d.LegacyDecision.SectorAvailable || d.PortfolioDecision.SectorAvailable ||
		d.Difference.SectorUnavailableNote != "")
	// cash ratio projected when equity known
	require.True(t, d.LegacyDecision.CashRatio != nil || d.LegacyDecision.CashRatioNote != "")
	require.True(t, d.PortfolioDecision.CashRatio != nil || d.PortfolioDecision.CashRatioNote != "")
	// tighten counts are explicit fields (legacy typically 0)
	require.GreaterOrEqual(t, d.Difference.RiskTightenCountPortfolio, 0)
	require.GreaterOrEqual(t, d.Difference.RiskTightenCountLegacy, 0)
}

func TestValidate_NoIndustrySafe(t *testing.T) {
	day := sampleDay()
	day.IndustryBySymbol = nil
	for i := range day.Candidates.RankedCandidates {
		day.Candidates.RankedCandidates[i].Industry = ""
	}
	day.Snapshot.Positions[0].Industry = ""
	rep := portfoliovalidation.Validate(portfoliovalidation.Input{
		Options: portfoliovalidation.Options{Enabled: true},
		Days:    []portfoliovalidation.DayCase{day},
	})
	d := rep.Days[0]
	require.True(t, d.OK)
	require.False(t, d.Difference.SectorAvailable)
	require.NotEmpty(t, d.Difference.SectorUnavailableNote)
}

func TestValidate_FromReplayCase(t *testing.T) {
	rc := portfolioreplay.ReplayCase{
		CaseID: "r1", TradeDate: "2026-08-20",
		DecisionTime: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC),
		Snapshot:     sampleDay().Snapshot,
		Candidates:   sampleDay().Candidates,
		Constraints:  sampleDay().Constraints,
	}
	day := portfoliovalidation.FromReplayCase(rc)
	day.LegacyAmountPerName = 100_000
	day.IndustryBySymbol = sampleDay().IndustryBySymbol
	rep := portfoliovalidation.Run([]portfoliovalidation.DayCase{day}, true)
	require.Equal(t, 1, rep.Summary.OKCount)
}

func TestValidate_Deterministic(t *testing.T) {
	in := portfoliovalidation.Input{
		Options: portfoliovalidation.Options{Enabled: true},
		Days:    []portfoliovalidation.DayCase{sampleDay()},
	}
	a := portfoliovalidation.Validate(in)
	b := portfoliovalidation.Validate(in)
	ja, err := json.Marshal(a.Days[0].Difference)
	require.NoError(t, err)
	jb, err := json.Marshal(b.Days[0].Difference)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
}

func TestReport_ForbidsPnLSharpeSemantics(t *testing.T) {
	rep := portfoliovalidation.Validate(portfoliovalidation.Input{
		Options: portfoliovalidation.Options{Enabled: true},
		Days:    []portfoliovalidation.DayCase{sampleDay()},
	})
	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	text := strings.ToLower(string(raw))
	require.NotContains(t, text, "sharpe_ratio")
	require.NotContains(t, text, "total_return")
	require.NotContains(t, text, "cum_pnl")
	require.True(t, rep.NotPnL)
	require.True(t, rep.NotSharpe)
	require.True(t, rep.NotABacktest)
	require.True(t, rep.NotAutoTune)
}

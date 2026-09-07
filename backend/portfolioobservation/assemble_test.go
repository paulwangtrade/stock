package portfolioobservation_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfolioinsight"
	"go-stock/backend/portfolioobservation"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/sectorcoverage"
	"go-stock/backend/sellsuggestion"
)

func f64(v float64) *float64 { return &v }

func sampleRisk() *portfoliorisk.PortfolioRiskSnapshot {
	return &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		AsOf:  time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC),
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, GrossExposure: f64(0.70), CashRatio: f64(0.25),
			HeadroomVsCap: f64(0.15), CapGross: f64(0.85), Equity: f64(1_000_000),
		},
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, Top1Weight: f64(0.22), Top5Weight: f64(0.55), NameCount: 4, CapSingle: f64(0.20),
		},
		Sector: portfoliorisk.SectorBlock{
			Available: true,
			SectorExposure: []portfoliorisk.SectorWeight{
				{Sector: "银行", Weight: 0.30, NameCount: 2},
			},
			MaxSectorWeight: f64(0.25),
		},
	}
}

func TestAssemble_CurrentPortfolioAndExplain(t *testing.T) {
	risk := sampleRisk()
	insight := portfolioinsight.Build(portfolioinsight.Input{
		AsOf: risk.AsOf, TradeDate: "2026-08-22", Risk: risk,
	})
	cov := &sectorcoverage.SectorCoverageReport{
		HoldingsCoverage: 1, AllowSectorConstraint: true, Note: "holdings_fully_classified",
	}
	sell := &sellsuggestion.Report{
		Enabled: true, Skipped: false,
		Suggestions: []sellsuggestion.SellSuggestion{
			{
				Symbol: "sz000001", Action: sellsuggestion.ActionReduce,
				Reason: "CONCENTRATION", SuggestSellQty: 100, TargetWeight: 0.10,
				RiskReason: "CONCENTRATION",
			},
		},
	}
	val := &portfoliovalidation.PortfolioValidationReport{
		Enabled: true, Skipped: false,
		Summary: portfoliovalidation.AggregateSummary{
			DayCount: 2, OKCount: 2, NameCountDeltaMean: -1, TightenCountPortfolioTotal: 1,
		},
	}

	view := portfolioobservation.Assemble(portfolioobservation.Input{
		TradeDate:  "2026-08-22",
		AccountID:  "1",
		Risk:       risk,
		Insight:    insight,
		SectorCov:  cov,
		Sell:       sell,
		Validation: val,
	})
	require.Equal(t, portfolioobservation.SchemaVersion, view.SchemaVersion)
	require.True(t, view.ReadOnly)
	require.True(t, view.NotATradePlan)
	require.True(t, view.NotOrder)
	require.True(t, view.NotExecution)
	require.True(t, view.NotProviderSwitch)
	require.True(t, view.NotAutoTrade)

	require.True(t, view.CurrentPortfolio.Available)
	require.True(t, view.CurrentPortfolio.Exposure.Available)
	require.NotNil(t, view.CurrentPortfolio.Exposure.GrossExposure)
	require.True(t, view.CurrentPortfolio.Concentration.Available)
	require.True(t, view.CurrentPortfolio.Sector.Available)
	require.True(t, view.CurrentPortfolio.Cash.Available)
	require.NotNil(t, view.CurrentPortfolio.Sector.AllowSectorConstraint)
	require.True(t, *view.CurrentPortfolio.Sector.AllowSectorConstraint)

	require.NotEmpty(t, view.DecisionExplain.WhyBuyLess)
	require.NotEmpty(t, view.DecisionExplain.WhySuggestReduce)
	require.Len(t, view.DecisionExplain.ReduceRows, 1)

	require.True(t, view.Validation.Present)
	require.Equal(t, 2, view.Validation.OKCount)
	require.True(t, view.SectorCoverage.Present)

	raw, err := json.Marshal(view)
	require.NoError(t, err)
	s := string(raw)
	require.NotContains(t, strings.ToLower(s), "createplan")
	require.Contains(t, s, `"not_provider_switch":true`)
}

func TestAssemble_MissingSources_DataGaps(t *testing.T) {
	view := portfolioobservation.Assemble(portfolioobservation.Input{})
	require.False(t, view.CurrentPortfolio.Available)
	require.Contains(t, view.DataGaps, "portfolio_risk_missing")
	require.Contains(t, view.DataGaps, "portfolio_insight_missing")
	require.Contains(t, view.DataGaps, "sell_suggestion_missing_or_disabled")
	require.False(t, view.Sources.RiskPresent)
}

func TestAssemble_SectorCoverageDisallowsConstraint(t *testing.T) {
	risk := sampleRisk()
	cov := &sectorcoverage.SectorCoverageReport{
		HoldingsCoverage: 0.5, AllowSectorConstraint: false, Note: "holdings_coverage_incomplete",
	}
	view := portfolioobservation.Assemble(portfolioobservation.Input{Risk: risk, SectorCov: cov})
	require.NotNil(t, view.CurrentPortfolio.Sector.AllowSectorConstraint)
	require.False(t, *view.CurrentPortfolio.Sector.AllowSectorConstraint)
	require.False(t, view.SectorCoverage.AllowSectorConstraint)
}

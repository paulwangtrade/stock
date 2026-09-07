package portfoliohistory_test

import (
	"encoding/json"
	"testing"
	"time"

	"go-stock/backend/portfoliohistory"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/providershadow"

	"github.com/stretchr/testify/require"
)

func TestDefaultEnabled_IsFalse(t *testing.T) {
	require.False(t, portfoliohistory.DefaultEnabled)
}

func TestRecordDay_Disabled_NoWrite(t *testing.T) {
	store := portfoliohistory.NewMemoryStore()
	rec, err := portfoliohistory.RecordDay(store, portfoliohistory.DayInput{
		Enabled:   false,
		TradeDate: "2026-08-22",
		Risk:      &portfoliorisk.PortfolioRiskSnapshot{Found: true, TradeDate: "2026-08-22"},
	})
	require.NoError(t, err)
	require.Nil(t, rec)
	require.Empty(t, store.List())
}

func TestRecordDay_AndBuildView(t *testing.T) {
	store := portfoliohistory.NewMemoryStore()
	ge := 0.72
	cash := 0.18
	top1 := 0.12
	legAmt := 40000.0
	portAmt := 32000.0

	val := &portfoliovalidation.PortfolioValidationReport{
		Enabled: true,
		Days: []portfoliovalidation.DayValidation{{
			CaseID:    "c1",
			TradeDate: "2026-08-21",
			OK:        true,
			LegacyDecision: portfoliovalidation.SideDecision{
				NameCount: 4, BuyNotionalSum: legAmt,
			},
			PortfolioDecision: portfoliovalidation.SideDecision{
				NameCount: 3, BuyNotionalSum: portAmt,
			},
			Difference: portfoliovalidation.DayDifference{
				NameCountLegacy: 4, NameCountPortfolio: 3, NameCountDelta: -1,
				RiskTightenCountPortfolio: 2,
				FilterRejectReasonsPortfolio: map[string]int{"CASH_INSUFFICIENT": 2, "NAME_LIMIT": 1},
				SectorAvailable: true,
			},
		}},
		Summary: portfoliovalidation.AggregateSummary{DayCount: 1, OKCount: 1},
	}

	risk := &portfoliorisk.PortfolioRiskSnapshot{
		Found:     true,
		TradeDate: "2026-08-21",
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, GrossExposure: &ge, CashRatio: &cash,
		},
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, Top1Weight: &top1, NameCount: 10,
		},
		Sector: portfoliorisk.SectorBlock{Available: false, Note: "unavailable"},
	}

	shadow := &providershadow.ShadowComparisonRecord{
		TradeDate:   "2026-08-21",
		Comparable:  true,
		Fingerprint: "fp-abc",
		ComparisonSummary: providershadow.ComparisonSummary{
			OnlyLegacyCount: 1, OnlyPortfolioCount: 0, CommonCount: 3,
		},
		LegacySummary:    providershadow.EnvelopeRef{OK: true, LineCount: 4},
		PortfolioSummary: providershadow.EnvelopeRef{OK: true, LineCount: 3},
		AllocationBudgetSummary: &providershadow.AllocationBudgetSummary{
			ImpliedLegacyNotional: legAmt,
			Portfolio:             providershadow.BudgetView{AvailableCapital: portAmt, Binding: "gross_headroom"},
			Binding:               "gross_headroom",
		},
		RiskCutSummary: &providershadow.RiskCutSummary{
			GrossHeadroomBinding: true, SingleCapApplied: true,
		},
	}

	rec, err := portfoliohistory.RecordDay(store, portfoliohistory.DayInput{
		Enabled:    true,
		TradeDate:  "2026-08-21",
		Validation: val,
		Risk:       risk,
		Shadow:     shadow,
		RecordedAt: time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.True(t, rec.Validation.Present)
	require.True(t, rec.Validation.OK)
	require.Equal(t, -1, rec.Validation.NameCountDelta)
	require.InDelta(t, -8000, rec.Validation.NotionalDelta, 1e-9)
	require.Equal(t, 2, rec.Validation.FilterRejectTop[0].Count) // CASH first by count
	require.True(t, rec.Risk.Found)
	require.InDelta(t, 0.72, *rec.Risk.GrossExposure, 1e-9)
	require.True(t, rec.AllocationShadow.Present)
	require.True(t, rec.AllocationShadow.Comparable)
	require.Equal(t, "gross_headroom", rec.AllocationShadow.BudgetBinding)

	// Second day + upsert replace
	_, err = portfoliohistory.RecordDay(store, portfoliohistory.DayInput{
		Enabled:   true,
		TradeDate: "2026-08-22",
		Risk: &portfoliorisk.PortfolioRiskSnapshot{
			Found: true, TradeDate: "2026-08-22",
			Exposure: portfoliorisk.ExposureBlock{Available: true, CashRatio: &cash},
		},
	})
	require.NoError(t, err)

	view := portfoliohistory.BuildView(store, portfoliohistory.ViewOptions{})
	require.Equal(t, portfoliohistory.SchemaVersion, view.SchemaVersion)
	require.True(t, view.NotABacktest)
	require.True(t, view.NotPnL)
	require.True(t, view.NotAutoTune)
	require.True(t, view.NotProviderSwitch)
	require.Equal(t, 2, view.Rollup.DayCount)
	require.Equal(t, 1, view.Rollup.ValidationOKDays)
	require.Equal(t, 2, view.Rollup.RiskFoundDays)
	require.Equal(t, 1, view.Rollup.ShadowPresentDays)
	require.Equal(t, "2026-08-21", view.Days[0].TradeDate)
	require.Equal(t, "2026-08-22", view.Days[1].TradeDate)

	raw, err := json.Marshal(view)
	require.NoError(t, err)
	blob := string(raw)
	require.Contains(t, blob, `"not_a_backtest":true`)
	require.Contains(t, blob, `"not_pnl":true`)
	require.NotContains(t, blob, "cumulative_return")
	require.NotContains(t, blob, "CreatePlanWithItems")
	require.NotContains(t, blob, `"sharpe":`)}

func TestSummarize_ValidationSkipped(t *testing.T) {
	rec := portfoliohistory.SummarizeDay(portfoliohistory.DayInput{
		TradeDate: "2026-08-20",
		Validation: &portfoliovalidation.PortfolioValidationReport{
			Enabled: false, Skipped: true, SkipReason: "enabled=false",
		},
	})
	require.True(t, rec.Validation.Present)
	require.True(t, rec.Validation.Skipped)
	require.Contains(t, rec.DataGaps, "validation_skipped")
	require.Contains(t, rec.DataGaps, "risk_missing")
	require.Contains(t, rec.DataGaps, "allocation_shadow_missing")
}

func TestBuildView_MaxDaysAndRange(t *testing.T) {
	store := portfoliohistory.NewMemoryStore()
	for _, d := range []string{"2026-08-18", "2026-08-19", "2026-08-20", "2026-08-21"} {
		_, err := portfoliohistory.RecordDay(store, portfoliohistory.DayInput{
			Enabled: true, TradeDate: d,
			Risk: &portfoliorisk.PortfolioRiskSnapshot{Found: true, TradeDate: d},
		})
		require.NoError(t, err)
	}
	view := portfoliohistory.BuildView(store, portfoliohistory.ViewOptions{
		FromTradeDate: "2026-08-19",
		ToTradeDate:   "2026-08-21",
		MaxDays:       2,
	})
	require.Len(t, view.Days, 2)
	require.Equal(t, "2026-08-20", view.Days[0].TradeDate)
	require.Equal(t, "2026-08-21", view.Days[1].TradeDate)
}

func TestUpsert_RequiresTradeDate(t *testing.T) {
	store := portfoliohistory.NewMemoryStore()
	err := store.Upsert(portfoliohistory.DailyRecord{})
	require.Error(t, err)
}

package portfoliorisk_test

import (
	"encoding/json"
	"testing"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestSuggestTighten_UnavailableBlocksNoPatch(t *testing.T) {
	t.Parallel()
	snap := &portfoliorisk.PortfolioRiskSnapshot{
		Found:         true,
		Exposure:      portfoliorisk.ExposureBlock{Available: false},
		Concentration: portfoliorisk.ConcentrationBlock{Available: false},
		Sector:        portfoliorisk.SectorBlock{Available: false, Note: portfoliorisk.NoteSectorUnavailable},
		Theme:         portfoliorisk.ThemeBlock{Available: false},
		Correlation:   portfoliorisk.CorrelationBlock{Available: false},
		Market:        portfoliorisk.MarketBlock{Available: false, MarketRegime: portfoliorisk.RegimeUnknown},
	}

	base := portfoliolayer.ConstraintSet{}
	got := portfoliorisk.SuggestTighten(snap, base)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotRiskViewWrite)
	require.True(t, got.NotPlanFilter)
	require.True(t, got.NotExecutionWrite)
	require.False(t, got.HasPatches)
	require.Empty(t, got.Notes)
	require.Nil(t, got.ConstraintSet.Portfolio.MaxNewNames)
	require.Nil(t, got.ConstraintSet.Risk.MaxGrossExposurePct)
	require.Equal(t, int64(0), got.ConstraintSet.Execution.MinLot)
}

func TestSuggestTighten_HeadroomExhausted_TightensPreferenceOnly(t *testing.T) {
	t.Parallel()
	head := -0.01
	cap := 0.85
	gross := 0.86
	cash := 0.30
	snap := &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, GrossExposure: &gross, CashRatio: &cash,
			HeadroomVsCap: &head, CapGross: &cap,
		},
		Concentration: portfoliorisk.ConcentrationBlock{Available: true},
		Sector:        portfoliorisk.SectorBlock{Available: false},
		Theme:         portfoliorisk.ThemeBlock{Available: false},
		Correlation:   portfoliorisk.CorrelationBlock{Available: false},
		Market:        portfoliorisk.MarketBlock{Available: true, MarketRegime: "configured_level_3"},
	}
	got := portfoliorisk.SuggestTighten(snap, portfoliolayer.ConstraintSet{})
	require.True(t, got.HasPatches)
	require.NotNil(t, got.ConstraintSet.Portfolio.MaxNewNames)
	require.Equal(t, 0, *got.ConstraintSet.Portfolio.MaxNewNames)
	require.NotNil(t, got.ConstraintSet.Portfolio.ReserveCashRatio)
	require.GreaterOrEqual(t, *got.ConstraintSet.Portfolio.ReserveCashRatio, 0.10)
	require.NotNil(t, got.ConstraintSet.Portfolio.MaxGrossExposurePct)
	require.InDelta(t, 0.85, *got.ConstraintSet.Portfolio.MaxGrossExposurePct, 1e-9)
	// Must not touch Risk / Execution in suggestion payload
	require.Nil(t, got.ConstraintSet.Risk.MaxGrossExposurePct)
	require.False(t, got.ConstraintSet.Risk.BlockNewEntries)
	require.Equal(t, int64(0), got.ConstraintSet.Execution.MinLot)
}

func TestSuggestTighten_Top1OverCap_SkipHeld(t *testing.T) {
	t.Parallel()
	top1 := 0.35
	cap := 0.20
	snap := &portfoliorisk.PortfolioRiskSnapshot{
		Found:    true,
		Exposure: portfoliorisk.ExposureBlock{Available: true},
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, Top1Weight: &top1, CapSingle: &cap,
		},
		Sector:      portfoliorisk.SectorBlock{Available: false},
		Theme:       portfoliorisk.ThemeBlock{Available: false},
		Correlation: portfoliorisk.CorrelationBlock{Available: false},
	}
	got := portfoliorisk.SuggestTighten(snap, portfoliolayer.ConstraintSet{})
	require.True(t, got.HasPatches)
	require.NotNil(t, got.ConstraintSet.Portfolio.SkipAlreadyHolding)
	require.True(t, *got.ConstraintSet.Portfolio.SkipAlreadyHolding)
	require.NotNil(t, got.ConstraintSet.Portfolio.AllowAddToHolding)
	require.False(t, *got.ConstraintSet.Portfolio.AllowAddToHolding)
	require.NotNil(t, got.ConstraintSet.Portfolio.MaxSingleWeight)
	require.InDelta(t, 0.20, *got.ConstraintSet.Portfolio.MaxSingleWeight, 1e-9)
}

func TestSuggestTighten_SectorUnavailable_NoSectorPatch(t *testing.T) {
	t.Parallel()
	snap := &portfoliorisk.PortfolioRiskSnapshot{
		Found:         true,
		Exposure:      portfoliorisk.ExposureBlock{Available: true},
		Concentration: portfoliorisk.ConcentrationBlock{Available: true},
		Sector: portfoliorisk.SectorBlock{
			Available: false,
			// Fake zeros must not trigger — available=false
			SectorExposure: []portfoliorisk.SectorWeight{{Sector: "X", Weight: 0.99}},
		},
		Theme:       portfoliorisk.ThemeBlock{Available: false},
		Correlation: portfoliorisk.CorrelationBlock{Available: false},
	}
	got := portfoliorisk.SuggestTighten(snap, portfoliolayer.ConstraintSet{})
	require.Nil(t, got.ConstraintSet.Portfolio.MaxSectorWeight)
	require.Nil(t, got.ConstraintSet.Portfolio.MaxNamesPerSector)
}

func TestSuggestTighten_SectorOverCap_WhenAvailable(t *testing.T) {
	t.Parallel()
	maxW := 0.25
	snap := &portfoliorisk.PortfolioRiskSnapshot{
		Found:         true,
		Exposure:      portfoliorisk.ExposureBlock{Available: true},
		Concentration: portfoliorisk.ConcentrationBlock{Available: true},
		Sector: portfoliorisk.SectorBlock{
			Available: true, MaxSectorWeight: &maxW,
			SectorExposure: []portfoliorisk.SectorWeight{{Sector: "bank", Weight: 0.40, NameCount: 2}},
		},
		Theme:       portfoliorisk.ThemeBlock{Available: false},
		Correlation: portfoliorisk.CorrelationBlock{Available: false},
	}
	got := portfoliorisk.SuggestTighten(snap, portfoliolayer.ConstraintSet{})
	require.True(t, got.HasPatches)
	require.NotNil(t, got.ConstraintSet.Portfolio.MaxSectorWeight)
	require.InDelta(t, 0.25, *got.ConstraintSet.Portfolio.MaxSectorWeight, 1e-9)
	require.NotNil(t, got.ConstraintSet.Portfolio.MaxNamesPerSector)
	require.Equal(t, 1, *got.ConstraintSet.Portfolio.MaxNamesPerSector)
}

func TestApplyTightenOnly_NeverWidensRiskOrTouchesExecution(t *testing.T) {
	t.Parallel()
	riskGross := 0.70
	riskSingle := 0.15
	base := portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: &riskGross,
			MaxSingleNamePct:    &riskSingle,
			MarketLevel:         2,
			BlockNewEntries:     false,
		},
		Execution: portfoliolayer.ExecutionLayer{MinLot: 100},
		Portfolio: portfoliolayer.PreferenceLayer{},
	}
	wideGross := 0.99
	wideSingle := 0.50
	evil := portfoliolayer.ConstraintSet{
		Portfolio: portfoliolayer.PreferenceLayer{
			MaxGrossExposurePct: &wideGross,
			MaxSingleWeight:     &wideSingle,
		},
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: floatPtr(0.99),
			BlockNewEntries:     true,
			MarketLevel:         9,
		},
		Execution: portfoliolayer.ExecutionLayer{MinLot: 1},
	}
	got := portfoliorisk.ApplyTightenOnly(base, evil)
	require.InDelta(t, 0.70, *got.Risk.MaxGrossExposurePct, 1e-9)
	require.InDelta(t, 0.15, *got.Risk.MaxSingleNamePct, 1e-9)
	require.Equal(t, 2, got.Risk.MarketLevel)
	require.False(t, got.Risk.BlockNewEntries)
	require.Equal(t, int64(100), got.Execution.MinLot)
	// Preference may carry wide numbers but Resolve will still cap by Risk.
	require.NotNil(t, got.Portfolio.MaxGrossExposurePct)
	require.InDelta(t, 0.99, *got.Portfolio.MaxGrossExposurePct, 1e-9)

	resolved := got.Resolve()
	require.InDelta(t, 0.70, resolved.MaxGrossExposurePct, 1e-9, "risk ceiling must win")
	require.InDelta(t, 0.15, resolved.MaxSingleWeight, 1e-9)
}

func TestApplyTightenOnly_PreferenceMinIsTighter(t *testing.T) {
	t.Parallel()
	baseGross := 0.80
	patchGross := 0.60
	baseReserve := 0.05
	patchReserve := 0.15
	base := portfoliolayer.ConstraintSet{
		Portfolio: portfoliolayer.PreferenceLayer{
			MaxGrossExposurePct: &baseGross,
			ReserveCashRatio:    &baseReserve,
		},
		Risk: portfoliolayer.RiskLayer{MaxGrossExposurePct: floatPtr(0.85)},
	}
	patch := portfoliolayer.ConstraintSet{
		Portfolio: portfoliolayer.PreferenceLayer{
			MaxGrossExposurePct: &patchGross,
			ReserveCashRatio:    &patchReserve,
			SkipAlreadyHolding:  boolPtr(true),
		},
	}
	got := portfoliorisk.ApplyTightenOnly(base, patch)
	require.InDelta(t, 0.60, *got.Portfolio.MaxGrossExposurePct, 1e-9)
	require.InDelta(t, 0.15, *got.Portfolio.ReserveCashRatio, 1e-9)
	require.True(t, *got.Portfolio.SkipAlreadyHolding)
	require.False(t, *got.Portfolio.AllowAddToHolding)
}

func TestSuggestTighten_Deterministic(t *testing.T) {
	t.Parallel()
	head := 0.0
	cap := 0.85
	cash := 0.10
	top1 := 0.30
	single := 0.20
	snap := &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, CashRatio: &cash, HeadroomVsCap: &head, CapGross: &cap,
		},
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, Top1Weight: &top1, CapSingle: &single,
		},
		Sector:      portfoliorisk.SectorBlock{Available: false},
		Theme:       portfoliorisk.ThemeBlock{Available: false},
		Correlation: portfoliorisk.CorrelationBlock{Available: false},
		Market:      portfoliorisk.MarketBlock{Available: true, BlockNewEntries: true},
	}
	a := portfoliorisk.SuggestTighten(snap, portfoliolayer.ConstraintSet{})
	b := portfoliorisk.SuggestTighten(snap, portfoliolayer.ConstraintSet{})
	ja, err := json.Marshal(a)
	require.NoError(t, err)
	jb, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
}

func TestSuggestTighten_EndToEndWithBuild_DoesNotAlterRiskView(t *testing.T) {
	t.Parallel()
	risk := &tradingconfig.RiskView{
		MarketLevel: 3, MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20,
	}
	riskBefore := *risk
	snap := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 50_000, TotalExposure: 900_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1, MarketValue: 900_000, Weight: 0.90},
		},
	}
	base := portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: floatPtr(0.85),
			MaxSingleNamePct:    floatPtr(0.20),
		},
		Execution: portfoliolayer.ExecutionLayer{MinLot: 100},
	}
	riskSnap := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot: snap, Risk: risk, Constraints: &base,
	})
	sug := portfoliorisk.SuggestTighten(riskSnap, base)
	effective := portfoliorisk.ApplyTightenOnly(base, sug.ConstraintSet)

	require.Equal(t, riskBefore, *risk, "RiskView must be unchanged")
	require.Equal(t, base.Risk.MarketLevel, effective.Risk.MarketLevel)
	require.InDelta(t, 0.85, *effective.Risk.MaxGrossExposurePct, 1e-9)
	require.Equal(t, int64(100), effective.Execution.MinLot)
	require.True(t, sug.HasPatches)
}

func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }

package portfoliotarget_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfoliotarget"
)

func f64(v float64) *float64 { return &v }
func i32(v int) *int         { return &v }

func baseInput() portfoliotarget.CompileInput {
	return portfoliotarget.CompileInput{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		UserRisk: portfoliotarget.UserRiskProfile{
			RiskTier:            portfoliotarget.RiskTierModerate,
			MaxGrossExposurePct: f64(0.85),
			MaxSingleWeight:     f64(0.20),
			TargetCashRatio:     f64(0.15),
		},
		Strategy: portfoliotarget.StrategyGoal{
			MinNames: i32(2),
			MaxNames: i32(3),
		},
		Objective: portfoliotarget.PortfolioObjective{
			TargetGrossExposurePct: f64(0.80),
			TargetCashRatio:        f64(0.20),
			MaxSingleWeight:        f64(0.15),
		},
		Symbols:   []string{"sz000001", "sz000002", "sh600000", "sz000858"},
		EquityRef: 1_000_000,
		Options:   portfoliotarget.CompileOptions{ScaleToFit: true},
	}
}

func TestCompileTargetPortfolio_CoreFields(t *testing.T) {
	got := portfoliotarget.CompileTargetPortfolio(baseInput())
	require.NotNil(t, got)
	require.Equal(t, portfoliotarget.SchemaVersion, got.SchemaVersion)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotATradePlan)
	require.True(t, got.NotExecution)
	require.True(t, got.NotAutoRebalance)
	require.True(t, got.NotAllocWrite)
	require.InDelta(t, 0.20, got.TargetCashRatio, 1e-9) // objective cash
	require.Equal(t, 3, len(got.TargetPositions))       // max_names=3
	require.LessOrEqual(t, got.MaxSingleWeight, 0.15+1e-12)
	require.LessOrEqual(t, got.TargetGrossExposure+got.TargetCashRatio, 1+1e-5)
	for _, p := range got.TargetPositions {
		require.LessOrEqual(t, p.TargetWeight, got.MaxSingleWeight+1e-9)
		require.InDelta(t, p.TargetWeight*1_000_000, p.TargetAmount, 1)
	}
}

func TestCompile_CannotWidenRiskCeiling(t *testing.T) {
	in := baseInput()
	in.UserRisk.MaxGrossExposurePct = f64(0.70)
	in.UserRisk.MaxSingleWeight = f64(0.10)
	in.Objective.TargetGrossExposurePct = f64(0.95) // attempt widen
	in.Objective.MaxSingleWeight = f64(0.30)
	got := portfoliotarget.CompileTargetPortfolio(in)
	require.LessOrEqual(t, got.TargetGrossExposure, 0.70+1e-9)
	require.LessOrEqual(t, got.MaxSingleWeight, 0.10+1e-9)
}

func TestCompile_SectorUnavailableFailClosed(t *testing.T) {
	in := baseInput()
	in.Sector = portfoliotarget.SectorClassification{Available: false, Note: "incomplete"}
	got := portfoliotarget.CompileTargetPortfolio(in)
	require.False(t, got.Sector.Available)
	require.Nil(t, got.Sector.MaxSectorWeight)
	require.Empty(t, got.TargetSectorWeights)
	for _, p := range got.TargetPositions {
		require.Empty(t, p.SectorName)
	}
}

func TestCompile_SectorAvailable(t *testing.T) {
	in := baseInput()
	in.Sector = portfoliotarget.SectorClassification{
		Available: true,
		Taxonomy:  "fixture",
		BySymbol: map[string]string{
			"sz000001": "银行", "sz000002": "地产", "sh600000": "银行", "sz000858": "白酒",
		},
		MaxSectorWeight: f64(0.30),
	}
	got := portfoliotarget.CompileTargetPortfolio(in)
	require.True(t, got.Sector.Available)
	require.NotNil(t, got.Sector.MaxSectorWeight)
	require.NotEmpty(t, got.TargetSectorWeights)
	require.NotEmpty(t, got.TargetPositions[0].SectorName)
}

func TestCompile_Deterministic(t *testing.T) {
	in := baseInput()
	a := portfoliotarget.CompileTargetPortfolio(in)
	b := portfoliotarget.CompileTargetPortfolio(in)
	require.Equal(t, a.Fingerprint, b.Fingerprint)
	ja, _ := json.Marshal(a.TargetPositions)
	jb, _ := json.Marshal(b.TargetPositions)
	require.JSONEq(t, string(ja), string(jb))
}

func TestProject_RebalanceAndSellAllocation(t *testing.T) {
	got := portfoliotarget.CompileTargetPortfolio(baseInput())
	rb := got.ToRebalanceTarget()
	require.Len(t, rb.Positions, len(got.TargetPositions))
	require.InDelta(t, got.TargetCashRatio, rb.CashBufferWeight, 1e-9)

	sa := got.ToSellAllocationTarget()
	require.Len(t, sa.Names, len(got.TargetPositions))
	require.Contains(t, got.WeightBySymbol(), "sz000001")
}

func TestCompile_SentinelSectorIgnored(t *testing.T) {
	in := baseInput()
	in.Sector = portfoliotarget.SectorClassification{
		Available: true,
		BySymbol:  map[string]string{"sz000001": "unknown", "sz000002": "地产", "sh600000": "银行"},
	}
	got := portfoliotarget.CompileTargetPortfolio(in)
	// unknown not attached
	for _, p := range got.TargetPositions {
		if p.Symbol == "sz000001" {
			require.Empty(t, p.SectorName)
		}
	}
}

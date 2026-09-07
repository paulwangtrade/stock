package holdingdecision_test

import (
	"encoding/json"
	"testing"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfoliorisk"

	"github.com/stretchr/testify/require"
)

func TestSimulateSellDecision_DefaultAllHold(t *testing.T) {
	t.Parallel()
	ret := -0.15
	got := holdingdecision.SimulateSellDecision(holdingdecision.SellDecisionSimInput{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000001", Weight: 0.40, TotalQty: 1000},
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {Symbol: "sz000001", CanSell: true, TotalQty: 1000, AvailableQty: 1000},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {
				Symbol: "sz000001", ReturnRate: &ret, CurrentPrice: ptr(10),
				RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong,
			},
		},
		RiskSnapshot: &holdingdecision.RiskSnapshotFact{Found: true, MaxSingleNamePct: ptr(0.20)},
		Policy:       holdingdecision.DefaultActionPolicy(),
	})
	require.Equal(t, holdingdecision.SimSchemaVersion, got.SchemaVersion)
	require.False(t, got.PersistSellPlans)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotExecution)
	require.True(t, got.NotSellTradePlan)
	require.Equal(t, holdingdecision.ActionHold, got.Decisions[0].Action)
	require.Contains(t, got.DataSourceNote, "Simulation")
}

func TestSimulateSellDecision_HOLD_REDUCE_EXIT(t *testing.T) {
	t.Parallel()
	danger := -0.12
	okRet := 0.01
	cap := 0.20

	got := holdingdecision.SimulateSellDecision(holdingdecision.SellDecisionSimInput{
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000001", Weight: 0.10, TotalQty: 100}, // HOLD
			{Symbol: "sz000002", Weight: 0.35, TotalQty: 200}, // REDUCE by concentration
			{Symbol: "sz000003", Weight: 0.15, TotalQty: 300}, // EXIT when enabled
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, TotalQty: 100, AvailableQty: 100},
			"sz000002": {CanSell: true, TotalQty: 200, AvailableQty: 200},
			"sz000003": {CanSell: false, TotalQty: 300, AvailableQty: 0, LockedQty: 300},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {ReturnRate: &okRet, CurrentPrice: ptr(10), RiskState: papertrading.RiskStateNormal, PeriodState: papertrading.HoldingPeriodShort},
			"sz000002": {ReturnRate: &okRet, CurrentPrice: ptr(10), RiskState: papertrading.RiskStateNormal, PeriodState: papertrading.HoldingPeriodMid},
			"sz000003": {ReturnRate: &danger, CurrentPrice: ptr(10), RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong},
		},
		RiskSnapshot: &holdingdecision.RiskSnapshotFact{Found: true, MaxSingleNamePct: &cap},
		Policy: holdingdecision.ActionPolicy{
			ReduceEnabled: true, ExitEnabled: true, PersistSellPlans: true, // persist forced off
			DefaultReduceFraction: 0.5,
		},
	})
	require.False(t, got.PersistSellPlans)
	by := map[string]string{}
	for _, d := range got.Decisions {
		by[d.Symbol] = d.Action
		require.True(t, d.RecordOnly)
		require.True(t, d.NotAnOrder)
		require.False(t, d.PersistSellPlans)
	}
	require.Equal(t, holdingdecision.ActionHold, by["sz000001"])
	require.Equal(t, holdingdecision.ActionReduce, by["sz000002"])
	require.Equal(t, holdingdecision.ActionExit, by["sz000003"])
	require.False(t, executableOf(got, "sz000003"))
	require.True(t, executableOf(got, "sz000002"))
}

func executableOf(v *holdingdecision.ActionView, sym string) bool {
	for _, d := range v.Decisions {
		if d.Symbol == sym {
			return d.ExecutableHint
		}
	}
	return false
}

func TestSimulateSellDecision_FromPortfolioRiskSnapshot(t *testing.T) {
	t.Parallel()
	ret := 0.02
	cap := 0.20
	top1 := 0.40
	gross := 0.75
	riskSnap := &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, GrossExposure: &gross,
		},
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, Top1Weight: &top1, CapSingle: &cap,
		},
		Market: portfoliorisk.MarketBlock{Available: true, MarketRegime: "configured_level_3"},
	}
	got := holdingdecision.SimulateSellDecision(holdingdecision.SellDecisionSimInput{
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000001", Weight: 0.40, TotalQty: 500},
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, TotalQty: 500, AvailableQty: 500},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {ReturnRate: &ret, CurrentPrice: ptr(12), RiskState: papertrading.RiskStateNormal},
		},
		PortfolioRisk: riskSnap,
		Policy:        holdingdecision.ActionPolicy{ReduceEnabled: true, DefaultReduceFraction: 0.5},
	})
	require.Equal(t, holdingdecision.ActionReduce, got.Decisions[0].Action)
	require.Contains(t, got.Decisions[0].ReasonCodes, holdingdecision.ReasonConcentration)
}

func TestSimulateSellDecision_Deterministic(t *testing.T) {
	t.Parallel()
	in := holdingdecision.SellDecisionSimInput{
		TradeDate: "2026-08-21",
		Holdings:  []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.3, TotalQty: 100}},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, TotalQty: 100, AvailableQty: 100},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {ReturnRate: ptr(-0.12), CurrentPrice: ptr(9), RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong},
		},
		RiskSnapshot: &holdingdecision.RiskSnapshotFact{Found: true},
		Policy:       holdingdecision.ActionPolicy{ExitEnabled: true},
	}
	a := holdingdecision.SimulateSellDecision(in)
	b := holdingdecision.SimulateSellDecision(in)
	require.Equal(t, a.InputsFingerprint, b.InputsFingerprint)
	ja, err := json.Marshal(a)
	require.NoError(t, err)
	jb, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
}

func TestSimulateSellDecision_SourceHasNoSellSideEffects(t *testing.T) {
	t.Parallel()
	// Behavioral: even with PersistSellPlans=true, simulation stays observation-only.
	got := holdingdecision.SimulateSellDecision(holdingdecision.SellDecisionSimInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.5, TotalQty: 1}},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, TotalQty: 1, AvailableQty: 1},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {ReturnRate: ptr(-0.2), CurrentPrice: ptr(1), RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong},
		},
		Policy: holdingdecision.ActionPolicy{ExitEnabled: true, PersistSellPlans: true, ReduceEnabled: true},
	})
	require.False(t, got.PersistSellPlans)
	require.True(t, got.NotSellTradePlan)
	require.True(t, got.NotExecution)
	require.Equal(t, holdingdecision.ActionExit, got.Decisions[0].Action)
}

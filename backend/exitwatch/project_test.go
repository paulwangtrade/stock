package exitwatch_test

import (
	"math"
	"testing"
	"time"

	"go-stock/backend/exitwatch"
	"go-stock/backend/externalmirror"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/positionstate"

	"github.com/stretchr/testify/require"
)

func TestDefaultPolicyMatchesExitEvaluation(t *testing.T) {
	def := papertrading.DefaultExitPolicy
	pol := exitwatch.DefaultPolicy
	require.Equal(t, def.PolicyID, pol.ID)
	require.Equal(t, def.Version, pol.Version)
	require.Equal(t, def.MaxHoldingDays, pol.MaxHoldingDays)
	require.InDelta(t, def.LossWatchThreshold, pol.LossWatchThreshold, 1e-9)
	require.InDelta(t, def.LossReviewThreshold, pol.LossReviewThreshold, 1e-9)
	require.Equal(t, papertrading.ExitObserveClassHold, exitwatch.ClassHold)
	require.Equal(t, papertrading.ExitObserveClassReduce, exitwatch.ClassReduce)
	require.Equal(t, papertrading.ExitObserveClassFlatten, exitwatch.ClassFlatten)
	require.Equal(t, papertrading.ExitObserveClassInsufficient, exitwatch.ClassInsufficient)
}

func TestProject_FailClosed_DoesNotEscalate(t *testing.T) {
	asOf := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	nan := math.NaN()
	severe := -0.2
	cases := []exitwatch.Facts{
		{Source: "", PositionID: "1", StockCode: "sz000001", PriceKnown: true, CostKnown: true, UnrealizedReturn: &severe},
		{Source: "broker", PositionID: "1", StockCode: "sz000001", PriceKnown: true, CostKnown: true, ExitState: "REVIEW_REQUIRED", UseExitEvaluation: true},
		{Source: exitwatch.SourcePaperSim, StockCode: "sz000001", PriceKnown: true, CostKnown: true, UseExitEvaluation: true, ExitState: "REVIEW_REQUIRED"},
		{Source: exitwatch.SourceExternalMirror, PositionID: "4", PriceKnown: true, CostKnown: true, UnrealizedReturn: &severe, HoldingDaysKnown: true, RequireHoldingDays: true, RequireQuantity: true, Quantity: 100, QuantityKnown: true},
		{Source: exitwatch.SourcePaperSim, PositionID: "sz000001", StockCode: "sz000001", CostKnown: true, UseExitEvaluation: true, ExitState: "WATCH", ExitReasonCodes: []string{"TIME_REVIEW"}},
		{Source: exitwatch.SourcePaperSim, PositionID: "sz000001", StockCode: "sz000001", PriceKnown: true, UseExitEvaluation: true, ExitState: "REVIEW_REQUIRED", ExitReasonCodes: []string{"LOSS_REVIEW"}, UnrealizedReturn: &severe},
		{Source: exitwatch.SourceExternalMirror, PositionID: "4", StockCode: "sz000001", PriceKnown: true, CostKnown: true, PriceStale: true, UnrealizedReturn: &severe, HoldingDays: 3, HoldingDaysKnown: true, RequireHoldingDays: true, RequireQuantity: true, Quantity: 100, QuantityKnown: true, ExitReasonCodes: []string{"LOSS_REVIEW", "PLAN_REVIEW"}, SignalReview: true},
		{Source: exitwatch.SourcePaperSim, PositionID: "sz000001", StockCode: "sz000001", PriceKnown: true, CostKnown: true, UseExitEvaluation: true, ExitState: "SELL", UnrealizedReturn: &severe},
		{Source: exitwatch.SourceExternalMirror, PositionID: "4", StockCode: "sz000001", PriceKnown: true, CostKnown: true, UnrealizedReturn: &nan, HoldingDaysKnown: true, RequireHoldingDays: true, RequireQuantity: true, Quantity: 100, QuantityKnown: true},
	}
	for i, f := range cases {
		f.AsOf = asOf
		f.Bar = "2026-09-30"
		item := exitwatch.Project(f)
		require.Equal(t, exitwatch.ClassInsufficient, item.Class, i)
		require.Equal(t, "数据不足", item.Label, i)
		require.False(t, item.SellIntentAllowed, i)
		require.Empty(t, item.ManualSellDraftRef, i)
		require.True(t, item.NotAnOrder, i)
		require.Equal(t, exitwatch.Disclaimer, item.Disclaimer, i)
		require.NotContains(t, item.ReasonCodes, exitwatch.ReasonLoss, i)
		require.NotContains(t, item.ReasonCodes, exitwatch.ReasonTime, i)
		require.NotContains(t, item.ReasonCodes, exitwatch.ReasonSignal, i)
		require.NotContains(t, item.ReasonCodes, exitwatch.ReasonPlan, i)
		require.NotContains(t, item.Summary, "立即卖出")
		require.NotContains(t, item.Summary, "SELL")
	}
}

func TestProject_PaperClasses_DedupPolicyAndDraftLink(t *testing.T) {
	asOf := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	base := exitwatch.Facts{
		Source:            exitwatch.SourcePaperSim,
		PositionID:        "sz000001",
		StockCode:         "sz000001",
		StockName:         "平安银行",
		PriceKnown:        true,
		CostKnown:         true,
		UseExitEvaluation: true,
		AsOf:              asOf,
		Bar:               "2026-09-30",
		ManualDraftRef:    "draft-9",
		T1Locked:          true,
	}

	hold := base
	hold.ExitState = "NORMAL"
	hold.T1Locked = false
	got := exitwatch.Project(hold)
	require.Equal(t, exitwatch.ClassHold, got.Class)
	require.False(t, got.SellIntentAllowed)
	require.Empty(t, got.ManualSellDraftRef)
	require.Equal(t, "default_v1@v1", got.PolicyRef)
	require.Equal(t, asOf, got.AsOf)
	require.Equal(t, "paper_sim|sz000001|HOLD|2026-09-30", got.DedupKey)

	locked := base
	locked.ExitState = "NORMAL"
	got = exitwatch.Project(locked)
	require.Equal(t, exitwatch.ClassHold, got.Class)
	require.Equal(t, []string{exitwatch.ReasonT1Locked}, got.ReasonCodes)
	require.False(t, got.SellIntentAllowed)
	require.Equal(t, "paper_sim|sz000001|T1_LOCKED|2026-09-30", got.DedupKey)

	reduce := base
	reduce.ExitState = "WATCH"
	reduce.ExitReasonCodes = []string{"TIME_REVIEW", "PLAN_REVIEW"}
	reduce.T1Locked = false
	got = exitwatch.Project(reduce)
	require.Equal(t, exitwatch.ClassReduce, got.Class)
	require.Equal(t, []string{exitwatch.ReasonTime, exitwatch.ReasonPlan}, got.ReasonCodes)
	require.Equal(t, exitwatch.ReasonPlan, got.Rule)
	require.True(t, got.SellIntentAllowed)
	require.True(t, got.RequiresManualConfirm)
	require.Equal(t, "draft-9", got.ManualSellDraftRef)
	require.Equal(t, "paper_sim|sz000001|PLAN|2026-09-30", got.DedupKey)

	flat := base
	flat.ExitState = "REVIEW_REQUIRED"
	flat.ExitReasonCodes = []string{"LOSS_REVIEW", "TIME_REVIEW"}
	flat.SignalReview = true
	got = exitwatch.Project(flat)
	require.Equal(t, exitwatch.ClassFlatten, got.Class)
	require.Equal(t, []string{exitwatch.ReasonTime, exitwatch.ReasonLoss, exitwatch.ReasonSignal, exitwatch.ReasonT1Locked}, got.ReasonCodes)
	require.Equal(t, exitwatch.ReasonLoss, got.Rule)
	require.True(t, got.SellIntentAllowed)
	require.Contains(t, got.Summary, exitwatch.Disclaimer)
}

func TestProject_MirrorSellIntentAlwaysFalse(t *testing.T) {
	asOf := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	ret := -0.2
	item := exitwatch.Project(exitwatch.Facts{
		Source:             exitwatch.SourceExternalMirror,
		PositionID:         "7",
		StockCode:          "sh600519",
		Quantity:           100,
		QuantityKnown:      true,
		RequireQuantity:    true,
		CostKnown:          true,
		PriceKnown:         true,
		UnrealizedReturn:   &ret,
		HoldingDays:        30,
		HoldingDaysKnown:   true,
		RequireHoldingDays: true,
		SignalReview:       true,
		ManualDraftRef:     "should-drop",
		AsOf:               asOf,
		Bar:                "2026-09-30",
	})
	require.Equal(t, exitwatch.ClassFlatten, item.Class)
	require.Contains(t, item.ReasonCodes, exitwatch.ReasonLoss)
	require.Contains(t, item.ReasonCodes, exitwatch.ReasonTime)
	require.Contains(t, item.ReasonCodes, exitwatch.ReasonSignal)
	require.NotContains(t, item.ReasonCodes, exitwatch.ReasonPlan)
	require.False(t, item.SellIntentAllowed)
	require.Empty(t, item.ManualSellDraftRef)
	require.Equal(t, "external_mirror|7|LOSS|2026-09-30", item.DedupKey)
	require.Equal(t, "default_v1@v1", item.PolicyRef)
	require.False(t, item.NotAnOrder == false)
}

func TestEvaluatePaper_ReusesExitEvaluationAndPositionState(t *testing.T) {
	asOf := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	px := 9.0
	cost := 10.0
	ret := -0.12
	view := &papertrading.ExitEvaluationView{
		AsOf: asOf,
		Policy: papertrading.ExitPolicyRef{
			PolicyID: "default_v1", Version: 1, MaxHoldingDays: 20,
			LossWatchThreshold: -0.05, LossReviewThreshold: -0.10,
		},
		Holdings: []papertrading.ExitEvaluationStockRow{
			{
				StockCode: "sz000001",
				StockName: "平安银行",
				Evaluation: papertrading.ExitEvaluationLabel{
					State:       papertrading.ExitEvalStateReviewRequired,
					ReasonCodes: []string{papertrading.ExitReasonLossReview},
					Summary:     "需要重新评估",
				},
				Lots: []papertrading.ExitEvaluationLotRow{{
					FillID: 3, UnrealizedReturn: &ret,
				}},
				Observation: papertrading.ExitObservation{Class: papertrading.ExitObserveClassFlatten},
				Explanation: &papertrading.PositionEvaluationExplanation{
					CurrentPrice: &px,
					CostPrice:    &cost,
					Freshness:    papertrading.EvaluationDataFreshness{PriceStatus: papertrading.EvalFreshnessFresh},
					RiskHints:    []string{papertrading.ExplainTagSignalExpired},
				},
			},
			{
				StockCode: "sh600000",
				StockName: "浦发银行",
				Evaluation: papertrading.ExitEvaluationLabel{
					State:       papertrading.ExitEvalStateReviewRequired,
					ReasonCodes: []string{papertrading.ExitReasonLossReview},
				},
				Lots:        []papertrading.ExitEvaluationLotRow{{FillID: 4, UnrealizedReturn: &ret}},
				Observation: papertrading.ExitObservation{Class: papertrading.ExitObserveClassInsufficient},
				Explanation: &papertrading.PositionEvaluationExplanation{
					CurrentPrice: &px,
					CostPrice:    &cost,
					Freshness:    papertrading.EvaluationDataFreshness{PriceStatus: papertrading.EvalFreshnessStale},
				},
			},
		},
	}
	states := []positionstate.PositionStateView{
		{Symbol: "SZ000001", State: positionstate.S1NewLocked, LockedQty: 100, CanSell: false, RiskTag: positionstate.RiskTagNewLocked},
	}
	items := exitwatch.EvaluatePaper(view, states, exitwatch.Options{
		AsOf: asOf, Bar: "2026-09-30",
		ManualDraftRefs: map[string]string{"sz000001": "draft-1"},
	})
	require.Len(t, items, 2)

	var severe, stale exitwatch.Item
	for _, it := range items {
		switch it.StockCode {
		case "sz000001":
			severe = it
		case "sh600000":
			stale = it
		}
	}
	require.Equal(t, exitwatch.ClassFlatten, severe.Class)
	require.Contains(t, severe.ReasonCodes, exitwatch.ReasonLoss)
	require.Contains(t, severe.ReasonCodes, exitwatch.ReasonSignal)
	require.Contains(t, severe.ReasonCodes, exitwatch.ReasonT1Locked)
	require.True(t, severe.SellIntentAllowed)
	require.Equal(t, "draft-1", severe.ManualSellDraftRef)
	require.Equal(t, "paper_sim|sz000001|LOSS|2026-09-30", severe.DedupKey)

	require.Equal(t, exitwatch.ClassInsufficient, stale.Class)
	require.Equal(t, []string{exitwatch.ReasonStale}, stale.ReasonCodes)
	require.False(t, stale.SellIntentAllowed)
	require.Empty(t, stale.ManualSellDraftRef)
}

func TestEvaluateMirror_QuoteOnlyFailClosed(t *testing.T) {
	asOf := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	rows := []externalmirror.View{
		{
			ID: 7, Source: externalmirror.Source, StockCode: "sh600519", StockName: "茅台",
			Quantity: 200, CostPrice: 100, EntryDate: "2026-09-01",
			FeedsTradePlan: true, Tradable: true,
		},
		{
			ID: 8, Source: externalmirror.Source, StockCode: "sz000001",
			Quantity: 100, CostPrice: 10, EntryDate: "2026-09-30",
		},
		{
			ID: 9, Source: "paper_sim", StockCode: "sh600000",
			Quantity: 100, CostPrice: 10, EntryDate: "2026-09-01",
		},
		{
			ID: 10, Source: externalmirror.Source, StockCode: "sz000002",
			Quantity: 100, CostPrice: 0, EntryDate: "2026-09-01",
		},
	}
	fresh := asOf.Add(-time.Minute)
	items := exitwatch.EvaluateMirror(rows, []exitwatch.QuoteSnap{
		{Code: "SH600519", Price: 80, FetchedAt: fresh},
		{Code: "sz000001", Price: 10, FetchedAt: asOf.Add(-48 * time.Hour)},
		{Code: "sz000002", Price: 8, FetchedAt: fresh},
	}, exitwatch.Options{AsOf: asOf, Bar: "2026-09-30"})

	byID := map[string]exitwatch.Item{}
	for _, it := range items {
		byID[it.PositionID] = it
	}
	loss := byID["7"]
	require.Equal(t, exitwatch.SourceExternalMirror, loss.Source)
	require.Equal(t, exitwatch.ClassFlatten, loss.Class)
	require.Equal(t, []string{exitwatch.ReasonTime, exitwatch.ReasonLoss}, loss.ReasonCodes)
	require.False(t, loss.SellIntentAllowed)
	require.NotContains(t, loss.ReasonCodes, exitwatch.ReasonPlan)
	require.NotContains(t, loss.ReasonCodes, exitwatch.ReasonSignal)

	stale := byID["8"]
	require.Equal(t, exitwatch.ClassInsufficient, stale.Class)
	require.Contains(t, stale.ReasonCodes, exitwatch.ReasonStale)
	require.Contains(t, stale.ReasonCodes, exitwatch.ReasonT1Locked)
	require.NotContains(t, stale.ReasonCodes, exitwatch.ReasonLoss)
	require.False(t, stale.SellIntentAllowed)

	foreign := byID["9"]
	require.NotEqual(t, exitwatch.SourcePaperSim, foreign.Source)
	require.Equal(t, exitwatch.ClassInsufficient, foreign.Class)
	require.Equal(t, []string{exitwatch.ReasonNoSource}, foreign.ReasonCodes)
	require.False(t, foreign.SellIntentAllowed)

	noCost := byID["10"]
	require.Equal(t, exitwatch.ClassInsufficient, noCost.Class)
	require.NotContains(t, noCost.ReasonCodes, exitwatch.ReasonLoss)
	require.False(t, noCost.SellIntentAllowed)
}

func TestBuild_NilDB_FailClosedFlags(t *testing.T) {
	view := exitwatch.Build(exitwatch.Options{
		AsOf: time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local),
		Bar:  "2026-09-30",
	})
	require.Empty(t, view.Items)
	require.False(t, view.AutoSell)
	require.False(t, view.PersistTrailing)
	require.False(t, view.PersistHWM)
	require.False(t, view.NewLedger)
	require.False(t, view.WritesTradePlan)
	require.False(t, view.WritesPaperSim)
	require.Equal(t, exitwatch.Disclaimer, view.Disclaimer)
	require.Equal(t, "default_v1@v1", view.PolicyRef)
	require.False(t, view.AsOf.IsZero())
	require.Len(t, view.Sources, 2)
	for _, st := range view.Sources {
		require.False(t, st.OK)
	}
	_, ok := exitwatch.ParseSource("live_broker")
	require.False(t, ok)
}

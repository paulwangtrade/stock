package strategysnapshot_test

import (
	"errors"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/strategysnapshot"

	"github.com/stretchr/testify/require"
)

func samplePlan() *models.TradePlan {
	slip := 0.03
	return &models.TradePlan{
		ID:                42,
		TradeDate:         "2026-08-10",
		PoolID:            7,
		PlanVersion:       3,
		AmountPerStock:    10000,
		MaxNames:          5,
		SourceSession:     models.TradePlanSourceAfterClose,
		PricingStage:      "after_close_intent",
		RiskStatus:        "ok",
		MarketLevel:       1,
		RiskFilteredCount: 1,
		RiskAcceptedCount: 1,
		RiskSummary:       "accepted=1 rejected=1",
		RiskSnapshotJSON:  `{"enabled":true,"accepted":1,"rejected":1}`,
		DefaultEntryRule:  "LIMIT_REF_PLUS_SLIP",
		DefaultMaxSlippage: &slip,
		Items: []models.TradePlanItem{
			{
				ID:              1001,
				StockCode:       "sz000001",
				StockName:       "平安银行",
				StrategyName:    "demo",
				StrategyVersion: "run:9",
				Status:          models.TradePlanItemPending,
				Reason:          "top score",
				RefPrice:        10.5,
				RefSource:       "kline_close",
				RefAsOf:         "2026-08-07",
				EntryRule:       "LIMIT_REF_PLUS_SLIP",
			},
			{
				ID:              1002,
				StockCode:       "sh600000",
				StrategyName:    "demo",
				StrategyVersion: "run:9",
				Status:          models.TradePlanItemSkipped,
				RiskCode:        "CASH",
				RiskMessage:     "insufficient cash",
			},
		},
	}
}

func samplePool() *models.CandidatePool {
	return &models.CandidatePool{
		ID:         7,
		TradeDate:  "2026-08-10",
		Source:     models.CandidatePoolSourceStrategyRun,
		SourceRef:  "strategyId=s1;runId=9",
		ConfigJSON: `{"maxCandidates":20,"strategyWeight":0.6,"signalWeight":0.4,"signalSnapshotId":55}`,
		Items: []models.CandidatePoolItem{
			{
				StockCode:        "sz000001",
				SignalTag:        "强",
				SignalScore:      0.8,
				SignalSnapshotID: 55,
				StrategyName:     "demo",
				StrategyVersion:  "run:9",
			},
			{
				StockCode: "sh600000",
				SignalTag: "",
			},
		},
	}
}

func TestCapture_CreateAndQuery(t *testing.T) {
	svc := strategysnapshot.NewService(strategysnapshot.NewMemoryStore())
	plan := samplePlan()
	pool := samplePool()

	res, err := svc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{
		Plan: plan,
		Pool: pool,
		Now:  time.Date(2026, 8, 9, 16, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NotNil(t, res.PlanSnap)
	require.Equal(t, strategysnapshot.ScopePlan, res.PlanSnap.Scope)
	require.Equal(t, strategysnapshot.SchemaVersion, res.PlanSnap.SchemaVersion)
	require.Equal(t, strategysnapshot.TriggerTradePlanCreate, res.PlanSnap.Trigger)
	require.Len(t, res.ItemSnaps, 2)

	ref, err := svc.GetPlanReference(42)
	require.NoError(t, err)
	require.Equal(t, res.PlanSnap.SnapshotID, ref.PlanSnapshotID)
	require.Equal(t, "sshot:42:1001:3", ref.ItemSnapshotIDs[1001])

	got, err := svc.Get(ref.ItemSnapshotIDs[1001])
	require.NoError(t, err)
	require.Equal(t, strategysnapshot.ScopeItem, got.Scope)
	require.Equal(t, "demo", got.StrategyVersion.StrategyName)
	require.Equal(t, "run:9", got.StrategyVersion.Version)
	require.Equal(t, models.CandidatePoolSourceStrategyRun, got.StrategyVersion.Source)
	require.Equal(t, 10.5, got.MarketDataRef.RefPrice)
	require.Equal(t, "kline_close", got.MarketDataRef.RefSource)
	require.False(t, got.MarketDataRef.Unavailable)
	require.Equal(t, uint(55), got.SignalResult.SignalSnapshotID)
	require.Equal(t, "强", got.SignalResult.Tag)
	require.NotNil(t, got.RiskDecision.ItemAccepted)
	require.True(t, *got.RiskDecision.ItemAccepted)
	require.Equal(t, float64(10000), got.Parameters.Values["amount_per_stock"])
	require.NotEmpty(t, got.Parameters.ParamsHash)

	skipped, err := svc.Get(ref.ItemSnapshotIDs[1002])
	require.NoError(t, err)
	require.NotNil(t, skipped.RiskDecision.ItemAccepted)
	require.False(t, *skipped.RiskDecision.ItemAccepted)
	require.Equal(t, "CASH", skipped.RiskDecision.RiskCode)

	list, err := svc.ListByPlan(42)
	require.NoError(t, err)
	require.Len(t, list, 3) // plan + 2 items
}

func TestCapture_StrategyEvaluationTrigger(t *testing.T) {
	svc := strategysnapshot.NewService(strategysnapshot.NewMemoryStore())
	res, err := svc.CaptureAfterStrategyEvaluation(strategysnapshot.CaptureInput{Plan: samplePlan()})
	require.NoError(t, err)
	require.Equal(t, strategysnapshot.TriggerStrategyEvaluation, res.PlanSnap.Trigger)
}

func TestMissingData_Handling(t *testing.T) {
	svc := strategysnapshot.NewService(strategysnapshot.NewMemoryStore())
	plan := &models.TradePlan{
		ID:          1,
		TradeDate:   "2026-08-10",
		PlanVersion: 1,
		Items: []models.TradePlanItem{
			{ID: 10, StockCode: "sz000002", Status: models.TradePlanItemPending},
		},
	}
	res, err := svc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{Plan: plan})
	require.NoError(t, err)

	item := res.ItemSnaps[0]
	require.True(t, item.MarketDataRef.Unavailable)
	require.Contains(t, item.MarketDataRef.MissingReason, "no price")
	require.True(t, item.SignalResult.Unavailable)
	require.Contains(t, item.SignalResult.MissingReason, "signal")
	require.True(t, item.RiskDecision.Unavailable || item.RiskDecision.RiskStatus == "")
	require.Equal(t, "unknown", item.StrategyVersion.Source)
	require.Empty(t, item.StrategyVersion.StrategyID) // never invent from name
}

func TestQuery_NotFound(t *testing.T) {
	svc := strategysnapshot.NewService(strategysnapshot.NewMemoryStore())
	_, err := svc.Get("missing")
	require.True(t, errors.Is(err, strategysnapshot.ErrNotFound))
	_, err = svc.GetPlanReference(999)
	require.True(t, errors.Is(err, strategysnapshot.ErrNotFound))
}

func TestCapture_RequiresPlanID(t *testing.T) {
	svc := strategysnapshot.NewService(strategysnapshot.NewMemoryStore())
	_, err := svc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{
		Plan: &models.TradePlan{TradeDate: "2026-08-10"},
	})
	require.Error(t, err)
}

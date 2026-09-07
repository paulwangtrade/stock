package strategyexplain_test

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/models"
	"go-stock/backend/strategysnapshot"
	"go-stock/backend/strategyexplain"

	"github.com/stretchr/testify/require"
)

func proUser(t *testing.T) *featuregate.User {
	t.Helper()
	u := &featuregate.User{ID: "explain-pro", Tier: featuregate.TierPro}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(u))
	require.True(t, featuregate.Allow(u, featuregate.FeatureAdvancedObservation))
	return u
}

func seedSnapshot(t *testing.T) (*strategysnapshot.CaptureResult, strategysnapshot.Service) {
	t.Helper()
	store := strategysnapshot.NewMemoryStore()
	snapSvc := strategysnapshot.NewService(store)
	accepted := true
	_ = accepted
	slip := 0.03
	plan := &models.TradePlan{
		ID:                42,
		TradeDate:         "2026-08-10",
		PlanVersion:       1,
		AmountPerStock:    10000,
		MaxNames:          5,
		DefaultEntryRule:  "LIMIT_REF_PLUS_SLIP",
		DefaultMaxSlippage: &slip,
		PricingStage:      "after_close_intent",
		RiskStatus:        "ok",
		RiskAcceptedCount: 1,
		RiskSnapshotJSON:  `{"enabled":true,"accepted":1}`,
		Items: []models.TradePlanItem{
			{
				ID:              1001,
				StockCode:       "sz000001",
				StrategyName:    "demo",
				StrategyVersion: "run:9",
				Status:          models.TradePlanItemPending,
				Reason:          "top score",
				RefPrice:        10.5,
				RefSource:       "kline_close",
				RefAsOf:         "2026-08-07",
				EntryRule:       "LIMIT_REF_PLUS_SLIP",
			},
		},
	}
	pool := &models.CandidatePool{
		ID:        7,
		TradeDate: "2026-08-10",
		Source:    models.CandidatePoolSourceStrategyRun,
		SourceRef: "strategyId=s1;runId=9",
		Items: []models.CandidatePoolItem{
			{
				StockCode:        "sz000001",
				SignalTag:        "强",
				SignalScore:      0.8,
				SignalSnapshotID: 55,
				StrategyName:     "demo",
				StrategyVersion:  "run:9",
			},
		},
	}
	res, err := snapSvc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{
		Plan: plan,
		Pool: pool,
		Now:  time.Date(2026, 8, 9, 16, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, res.ItemSnaps, 1)
	return res, snapSvc
}

func TestExplain_SnapshotExists(t *testing.T) {
	cap, snapSvc := seedSnapshot(t)
	svc := strategyexplain.NewService(snapSvc)
	itemID := cap.ItemSnaps[0].SnapshotID

	out, err := svc.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:            proUser(t),
		SnapshotID:      itemID,
		EntryReasonHint: "top score",
		EntryRuleHint:   "LIMIT_REF_PLUS_SLIP",
		IncludeExit:     true,
		ExitOverlay: &strategyexplain.ExitOverlay{
			HoldingDays: 25,
			ExitState:   "REVIEW_REQUIRED",
			ExitReasonCodes: []string{"TIME_REVIEW", "LOSS_REVIEW"},
			UnrealizedReturn: func() *float64 { v := -0.06; return &v }(),
		},
		Now: time.Date(2026, 8, 9, 17, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.True(t, out.Status == strategyexplain.StatusOK || out.Status == strategyexplain.StatusDegraded)
	require.Equal(t, strategyexplain.SchemaVersion, out.SchemaVersion)
	require.Equal(t, itemID, out.SnapshotID)
	require.True(t, out.Sections.Signal.Available)
	require.Equal(t, "强", out.Sections.Signal.Tag)
	require.Contains(t, out.Sections.Signal.Narrative, "强")
	require.True(t, out.Sections.Risk.Available)
	require.NotNil(t, out.Sections.Risk.Accepted)
	require.True(t, *out.Sections.Risk.Accepted)
	require.Contains(t, out.Sections.Entry.Narrative, "demo")
	require.Contains(t, out.Sections.Entry.Narrative, "10.5")
	require.NotNil(t, out.Sections.Exit)
	require.Equal(t, strategyexplain.ExitModeReview, out.Sections.Exit.Mode)
	require.Contains(t, out.Sections.Exit.Narrative, "重新评估")
	require.NotEmpty(t, out.Headline)
	require.NotEmpty(t, out.Disclaimers)
}

func TestExplain_MissingSnapshot(t *testing.T) {
	svc := strategyexplain.NewService(strategysnapshot.NewService(strategysnapshot.NewMemoryStore()))
	out, err := svc.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:       proUser(t),
		SnapshotID: "sshot:missing",
	})
	require.NoError(t, err)
	require.Equal(t, strategyexplain.StatusMissing, out.Status)
	require.Contains(t, out.Headline, "未保存策略快照")
}

func TestExplain_ByPlanItemReference(t *testing.T) {
	cap, snapSvc := seedSnapshot(t)
	svc := strategyexplain.NewService(snapSvc)
	out, err := svc.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:       proUser(t),
		PlanID:     42,
		PlanItemID: 1001,
	})
	require.NoError(t, err)
	require.NotEqual(t, strategyexplain.StatusMissing, out.Status)
	require.Equal(t, cap.ItemSnaps[0].SnapshotID, out.SnapshotID)
}

func TestExplain_GatedForFree(t *testing.T) {
	_, snapSvc := seedSnapshot(t)
	svc := strategyexplain.NewService(snapSvc)
	free := &featuregate.User{ID: "explain-free", Tier: featuregate.TierFree}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(free))

	out, err := svc.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:       free,
		SnapshotID: "sshot:42:1001:1",
	})
	require.NoError(t, err)
	require.Equal(t, strategyexplain.StatusGated, out.Status)
	require.NotEmpty(t, out.GateReason)
}

func TestExplain_Generation_DegradedWithoutSignal(t *testing.T) {
	store := strategysnapshot.NewMemoryStore()
	snapSvc := strategysnapshot.NewService(store)
	plan := &models.TradePlan{
		ID:          7,
		TradeDate:   "2026-08-10",
		PlanVersion: 1,
		Items: []models.TradePlanItem{
			{ID: 1, StockCode: "sz000002", Status: models.TradePlanItemPending},
		},
	}
	_, err := snapSvc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{Plan: plan})
	require.NoError(t, err)

	svc := strategyexplain.NewService(snapSvc)
	out, err := svc.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:       proUser(t),
		PlanID:     7,
		PlanItemID: 1,
	})
	require.NoError(t, err)
	require.Equal(t, strategyexplain.StatusDegraded, out.Status)
	require.False(t, out.Sections.Signal.Available)
	require.NotEmpty(t, out.Sections.Entry.Narrative)
}

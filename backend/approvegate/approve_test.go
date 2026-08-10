package approvegate

import (
	"fmt"
	"os"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupApproveWriteTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:approve_write_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureTradePlanTables())
}

func seedMaterializedDraft(t *testing.T, tradeDate string) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		PlanVersion:          1,
		EnableExecute:        false,
		AmountPerStock:       100_000,
		MaxNames:             5,
		SourceSession:        models.TradePlanSourceAfterClose,
		Side:                 "buy",
		PricingPolicyVersion: 1,
		PricingStage:         "morning_materialized",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate:    tradeDate,
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		Priority:     1,
		TargetAmount: 100_000,
		LimitPrice:   10.0,
		TargetVolume: 10_000,
		IntentStatus: readiness.IntentPriced,
		Status:       models.TradePlanItemPending,
	}}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func seedUnmaterializedDraft(t *testing.T, tradeDate string) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		PlanVersion:          1,
		AmountPerStock:       100_000,
		MaxNames:             5,
		Side:                 "buy",
		PricingPolicyVersion: 1,
		PricingStage:         "after_close_intent",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate:    tradeDate,
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		Priority:     1,
		TargetAmount: 100_000,
		IntentStatus: readiness.IntentSelected,
		Status:       models.TradePlanItemPending,
	}}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func writeReadinessOpts() *readiness.Options {
	return &readiness.Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval: true,
			IndustryByCode: map[string]string{"sz000001": "银行"},
			NameByCode:     map[string]string{"sz000001": "平安银行"},
			AnchorPriceByCode: map[string]float64{"sz000001": 10},
		},
	}
}

func TestApproveTradePlanByID_ReadyAndRiskPassWrites(t *testing.T) {
	setupApproveWriteTestDB(t)
	plan := seedMaterializedDraft(t, "2026-07-30")
	itemBefore := plan.Items[0]

	res, err := ApproveTradePlanByID(plan.ID, "alice", "manual", &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  passRiskEvaluator,
			ReadinessOpts: writeReadinessOpts(),
		},
	})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, CodeApproveWriteOK, res.Code)
	require.False(t, res.AlreadyApproved)
	require.NotNil(t, res.Plan)
	require.Equal(t, models.TradePlanStatusDraft, res.Plan.Status)
	require.NotNil(t, res.Plan.ApprovedAt)
	require.Equal(t, "alice", res.Plan.ApprovedBy)
	require.Equal(t, "manual", res.Plan.ApprovedSource)
	require.Nil(t, res.Plan.FreezeAt)
	require.False(t, res.Plan.EnableExecute)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.Equal(t, itemBefore.LimitPrice, got.Items[0].LimitPrice)
	require.Equal(t, itemBefore.TargetVolume, got.Items[0].TargetVolume)
	require.Equal(t, itemBefore.IntentStatus, got.Items[0].IntentStatus)
}

func TestApproveTradePlanByID_RiskFailNoWrite(t *testing.T) {
	setupApproveWriteTestDB(t)
	plan := seedMaterializedDraft(t, "2026-07-30")

	res, err := ApproveTradePlanByID(plan.ID, "alice", "manual", &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  failRiskEvaluator,
			ReadinessOpts: writeReadinessOpts(),
		},
	})
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Equal(t, CodeDenied, res.Code)
	require.True(t, hasCode(res.Blockers, CodeRiskNotPassed))

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)
	require.Empty(t, got.ApprovedBy)
	require.Empty(t, got.ApprovedSource)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
}

func TestApproveTradePlanByID_ReadinessFailNoWrite(t *testing.T) {
	setupApproveWriteTestDB(t)
	plan := seedUnmaterializedDraft(t, "2026-07-30")

	res, err := ApproveTradePlanByID(plan.ID, "alice", "manual", &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  passRiskEvaluator,
			ReadinessOpts: writeReadinessOpts(),
		},
	})
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Equal(t, CodeDenied, res.Code)
	require.NotEmpty(t, res.Blockers)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)
	require.Empty(t, got.ApprovedSource)
}

func TestApproveTradePlanByID_DuplicateApproveNoOverwrite(t *testing.T) {
	setupApproveWriteTestDB(t)
	plan := seedMaterializedDraft(t, "2026-07-30")

	first, err := ApproveTradePlanByID(plan.ID, "alice", "manual", &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  passRiskEvaluator,
			ReadinessOpts: writeReadinessOpts(),
		},
	})
	require.NoError(t, err)
	require.True(t, first.OK)
	firstAt := *first.Plan.ApprovedAt

	time.Sleep(2 * time.Millisecond)
	second, err := ApproveTradePlanByID(plan.ID, "bob", "http_api", &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  passRiskEvaluator,
			ReadinessOpts: writeReadinessOpts(),
		},
	})
	require.NoError(t, err)
	require.False(t, second.OK)
	require.True(t, second.AlreadyApproved)
	require.Equal(t, CodeAlreadyApproved, second.Code)
	require.NotNil(t, second.Plan)
	require.Equal(t, "alice", second.Plan.ApprovedBy)
	require.Equal(t, "manual", second.Plan.ApprovedSource)
	require.True(t, second.Plan.ApprovedAt.Equal(firstAt))
	require.Equal(t, models.TradePlanStatusDraft, second.Plan.Status)
}

func TestApproveTradePlanByID_CASConflict(t *testing.T) {
	setupApproveWriteTestDB(t)
	plan := seedMaterializedDraft(t, "2026-07-30")
	// Force CAS miss: eligibility sees draft, DB row is no longer draft.
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Where("id = ?", plan.ID).
		Update("status", models.TradePlanStatusReady).Error)

	view := *plan
	view.Status = models.TradePlanStatusDraft

	res, err := ApproveTradePlanByID(plan.ID, "alice", "manual", &ApproveOptions{
		Eligibility: &EligibilityOptions{
			LoadPlan: func(uint) (*models.TradePlan, error) {
				cp := view
				return &cp, nil
			},
			EvaluateRisk:  passRiskEvaluator,
			ReadinessOpts: writeReadinessOpts(),
		},
	})
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Equal(t, CodeCASMiss, res.Code)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)
	require.Equal(t, models.TradePlanStatusReady, got.Status)
}

func TestApproveDraftGate_RepoCASRequiresUnapprovedDraft(t *testing.T) {
	setupApproveWriteTestDB(t)
	plan := seedMaterializedDraft(t, "2026-07-30")
	at := time.Now()

	ok, err := data.NewTradePlanRepo().ApproveDraftGate(plan.ID, "alice", "system", at)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = data.NewTradePlanRepo().ApproveDraftGate(plan.ID, "bob", "http_api", at.Add(time.Second))
	require.NoError(t, err)
	require.False(t, ok)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, "alice", got.ApprovedBy)
	require.Equal(t, "system", got.ApprovedSource)
}

func TestApproveTradePlanByID_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("approve.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"PromoteDraftToFrozen(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunDailyCandidateAndPlan(",
		"TradePlanStatusReady",
		"FreezeTradePlan(",
	} {
		require.NotContains(t, src, token)
	}
	require.Contains(t, src, "CheckApproveEligibility")
	require.Contains(t, src, "ApproveDraftGate")
}

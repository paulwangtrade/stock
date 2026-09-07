package strategy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/models"
	"go-stock/backend/providershadow"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func setAfterCloseDraftShadowForTest(t *testing.T, rt *providershadow.Runtime) {
	t.Helper()
	prev := afterCloseDraftShadow
	afterCloseDraftShadow = rt
	t.Cleanup(func() { afterCloseDraftShadow = prev })
}

func TestG12_ShadowDefaultOff_ProductionUnchangedAndNoRecords(t *testing.T) {
	require.False(t, providershadow.DefaultEnabled)
	require.False(t, afterCloseDraftShadow.Enabled())
	require.False(t, tradingconfig.ProviderShadowEnabled())

	setupDraftPlanTestDB(t)
	stubPlanFilterContext(t, r1WideRisk(1_000_000, defaultMaxPlanNames, defaultMaxCandidates))
	store := providershadow.NewMemoryStore()
	setAfterCloseDraftShadowForTest(t, providershadow.NewRuntime(providershadow.Config{
		Enabled: false,
		Store:   store,
	}))

	pool := seedReadyPool(t, "2026-08-21", "sz000002", "sz000001", "sz000003")
	want, err := FilterPoolForTradePlan(pool, 100_000, defaultMaxPlanNames)
	require.NoError(t, err)

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
	require.Equal(t, len(want.Items), len(plan.Items))
	for i := range plan.Items {
		require.Equal(t, want.Items[i].Candidate.StockCode, plan.Items[i].StockCode)
		require.Equal(t, want.Items[i].Candidate.TargetAmount, plan.Items[i].TargetAmount)
		require.Equal(t, 100_000.0, plan.Items[i].TargetAmount)
	}
	require.Empty(t, store.List())
}

func TestG12_ShadowOn_LegacyStillEntersFilter_PortfolioOnlyRecord(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubPlanFilterContext(t, r1WideRisk(1_000_000, defaultMaxPlanNames, defaultMaxCandidates))
	store := providershadow.NewMemoryStore()
	setAfterCloseDraftShadowForTest(t, providershadow.NewRuntime(providershadow.Config{
		Enabled: true,
		Store:   store,
	}))

	pool := seedReadyPool(t, "2026-08-21", "sz000002", "sz000001", "sz000003")
	filtered, err := FilterPoolForTradePlan(pool, 100_000, defaultMaxPlanNames)
	require.NoError(t, err)
	require.Empty(t, store.List(), "FilterPool must not run shadow")

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
	require.Equal(t, len(filtered.Items), len(plan.Items))
	for i := range plan.Items {
		require.Equal(t, filtered.Items[i].Candidate.StockCode, plan.Items[i].StockCode)
		require.Equal(t, 100_000.0, plan.Items[i].TargetAmount)
		require.Equal(t, filtered.Items[i].Candidate.TargetAmount, plan.Items[i].TargetAmount)
	}
	require.Len(t, store.List(), 1)
	rec := store.List()[0]
	require.Equal(t, providershadow.ProviderVersionStamp, rec.ProviderVersion)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.LegacyProviderVersion)
	require.Equal(t, decisionprovider.ProviderPortfolioAllocation, rec.PortfolioProviderVersion)
	require.NotEmpty(t, rec.ContextFingerprint)
	require.NotNil(t, rec.Report)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.Report.ChainProvider)
	require.Equal(t, rec.Report.Legacy, rec.LegacySummary)
	require.Equal(t, rec.Report.Portfolio, rec.PortfolioSummary)
}

func TestG12_ShadowStoreFailure_DraftStillGenerated(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubPlanFilterContext(t, r1WideRisk(1_000_000, defaultMaxPlanNames, defaultMaxCandidates))
	setAfterCloseDraftShadowForTest(t, providershadow.NewRuntime(providershadow.Config{
		Enabled: true,
		Store:   failShadowStore{},
	}))

	pool := seedReadyPool(t, "2026-08-21", "sz000002", "sz000001")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.NotZero(t, plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
}

func TestG12_ShadowPortfolioFailure_DraftStillGenerated(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubPlanFilterContext(t, r1WideRisk(1_000_000, defaultMaxPlanNames, defaultMaxCandidates))
	store := providershadow.NewMemoryStore()
	setAfterCloseDraftShadowForTest(t, providershadow.NewRuntime(providershadow.Config{
		Enabled:   true,
		Store:     store,
		Portfolio: failPortfolioProvider{},
	}))

	pool := seedReadyPool(t, "2026-08-21", "sz000002", "sz000001")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.NotZero(t, plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
	require.NotEmpty(t, plan.Items)
	require.Len(t, store.List(), 1)
	rec := store.List()[0]
	require.True(t, rec.Failure.PortfolioFailed)
	require.NotEmpty(t, rec.Failure.ErrorReason)
}

func TestG12_ShadowForbiddenPathsDoNotImportRuntime(t *testing.T) {
	files := []string{
		"freeze_trade_plan.go",
		"morning_position_materialize.go",
		"morning_plan_preparation.go",
		"plan_risk_bridge.go",
		"rescale_trade_plan_cash.go",
	}
	for _, name := range files {
		src, err := os.ReadFile(name)
		require.NoError(t, err, name)
		require.NotContains(t, string(src), "go-stock/backend/providershadow", name)
		require.NotContains(t, string(src), "NewPortfolioDecisionProvider", name)
	}
	draft, err := os.ReadFile("draft_shadow.go")
	require.NoError(t, err)
	require.Contains(t, string(draft), "go-stock/backend/providershadow")
	require.NotContains(t, string(draft), "NewTradePlanRepo")
	require.NotContains(t, string(draft), "CreatePlanWithItems")
	require.NotContains(t, string(draft), "FreezeTradePlan")
}

func TestG12_ShadowPackageDoesNotCallTradeWriteAPIs(t *testing.T) {
	root := filepath.Join("..", "providershadow")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, e.Name()))
		require.NoError(t, err, e.Name())
		text := string(src)
		require.NotContains(t, text, "NewTradePlanRepo", e.Name())
		require.NotContains(t, text, "CreatePlanWithItems", e.Name())
		require.NotContains(t, text, "FreezeTradePlan", e.Name())
		require.NotContains(t, text, "MaterializePositions", e.Name())
		require.NotContains(t, text, "RescaleTradePlanForCash", e.Name())
		require.NotContains(t, text, "go-stock/backend/execution", e.Name())
	}
}

type failPortfolioProvider struct{}

func (failPortfolioProvider) Name() string { return decisionprovider.ProviderPortfolioAllocation }

func (failPortfolioProvider) Decide(ctx decisionprovider.DecisionContext) (*decisionprovider.DecisionEnvelope, error) {
	err := &decisionprovider.DecisionError{Code: decisionprovider.ErrCodeInvalidAllocation, Message: "injected shadow failure"}
	return &decisionprovider.DecisionEnvelope{
		OK:           false,
		Provider:     decisionprovider.ProviderPortfolioAllocation,
		DecisionTime: ctx.DecisionTime,
		Error:        err,
		Lines:        []decisionprovider.DecisionLine{},
	}, err
}

type failShadowStore struct{}

func (failShadowStore) Append(providershadow.ShadowComparisonRecord) error {
	return errShadowStoreInjected
}

func (failShadowStore) List() []providershadow.ShadowComparisonRecord { return nil }

var errShadowStoreInjected = errString("injected shadow store failure")

type errString string

func (e errString) Error() string { return string(e) }

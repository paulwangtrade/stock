package strategy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestRunDailyCandidateAndPlan_SourceUsesDraftBuilder(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("build_trade_plan.go"))
	require.NoError(t, err)
	src := string(b)

	fn := src
	idx := strings.Index(fn, "func RunDailyCandidateAndPlan")
	require.GreaterOrEqual(t, idx, 0)
	rest := fn[idx:]
	next := strings.Index(rest[len("func RunDailyCandidateAndPlan"):], "\nfunc ")
	if next >= 0 {
		rest = rest[:len("func RunDailyCandidateAndPlan")+next]
	}
	require.Contains(t, rest, "BuildDraftTradePlanFromCandidatePool(")
	require.NotContains(t, rest, "BuildTradePlan(")
	require.NotContains(t, rest, "FilterPoolForTradePlan(")
}

func TestRunDailyCandidateAndPlan_FallbackPersistsDraft(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-08-19", "sz000001", "sh600519")

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.False(t, plan.EnableExecute)
	require.Nil(t, plan.FreezeAt)
	require.Nil(t, plan.ApprovedAt)
	require.True(t, plan.IsDraft())
	require.False(t, plan.IsFrozen())
	require.Equal(t, "buy", plan.Side)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.False(t, got.EnableExecute)
	require.Nil(t, got.FreezeAt)
}

func TestBuildTradePlanForDate_CaseE_StillLegacyReady(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-08-19", "sz000001")

	plan, err := BuildTradePlanForDate("2026-08-19")
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, plan.Status)
	require.Nil(t, plan.FreezeAt)
	require.Equal(t, pool.ID, plan.PoolID)
	require.False(t, plan.IsFrozen(), "TEMP BuildTradePlanForDate remains naked ready")
}

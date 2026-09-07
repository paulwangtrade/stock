package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestDeriveLifecycleDisplayStatus(t *testing.T) {
	now := time.Now()
	require.Equal(t, papertrading.LifecycleDisplayDraft, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusDraft}, ""))
	require.Equal(t, papertrading.LifecycleDisplayApproved, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusDraft, ApprovedAt: &now}, ""))
	require.Equal(t, papertrading.LifecycleDisplayFrozen, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusReady, FreezeAt: &now, ApprovedAt: &now}, ""))
	require.Equal(t, papertrading.LifecycleDisplayExecuting, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusExecuting}, ""))
	require.Equal(t, papertrading.LifecycleDisplayExecuting, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusReady, FreezeAt: &now}, papertrading.RunStatusRunning))
	require.Equal(t, papertrading.LifecycleDisplayCompleted, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusDone}, ""))
	require.Equal(t, papertrading.LifecycleDisplayCompleted, papertrading.DeriveLifecycleDisplayStatus(
		models.TradePlan{Status: models.TradePlanStatusSuperseded}, ""))
}

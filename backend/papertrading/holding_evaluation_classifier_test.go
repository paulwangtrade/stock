package papertrading_test

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestClassifyProfitState_ProfitLossBreakeven(t *testing.T) {
	p := 0.05
	require.Equal(t, papertrading.ProfitStateProfit, papertrading.ClassifyProfitState(&p))
	l := -0.032
	require.Equal(t, papertrading.ProfitStateLoss, papertrading.ClassifyProfitState(&l))
	z := 0.0
	require.Equal(t, papertrading.ProfitStateBreakeven, papertrading.ClassifyProfitState(&z))
	tiny := 0.0005
	require.Equal(t, papertrading.ProfitStateBreakeven, papertrading.ClassifyProfitState(&tiny))
}

func TestClassifyProfitState_NoPrice(t *testing.T) {
	require.Equal(t, papertrading.ProfitStateUnknown, papertrading.ClassifyProfitState(nil))
}

func TestClassifyHoldingPeriod_LongTerm(t *testing.T) {
	require.Equal(t, papertrading.HoldingPeriodShort, papertrading.ClassifyHoldingPeriodState(3, papertrading.DefaultHoldingPeriodThresholds))
	require.Equal(t, papertrading.HoldingPeriodMid, papertrading.ClassifyHoldingPeriodState(13, papertrading.DefaultHoldingPeriodThresholds))
	require.Equal(t, papertrading.HoldingPeriodLong, papertrading.ClassifyHoldingPeriodState(25, papertrading.DefaultHoldingPeriodThresholds))
}

func TestClassify_NoForbiddenActionWords(t *testing.T) {
	labels := []string{
		papertrading.ClassifyProfitState(ptrFloat(0.1)),
		papertrading.ClassifyProfitState(ptrFloat(-0.1)),
		papertrading.ClassifyHoldingPeriodState(100, papertrading.DefaultHoldingPeriodThresholds),
	}
	for _, s := range labels {
		require.NotContains(t, s, "BUY")
		require.NotContains(t, s, "SELL")
		require.NotContains(t, s, "EXIT")
	}
}

package safetygate

import (
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func validSpec() Spec {
	return Spec{
		Symbol:       "sz000001",
		Side:         "buy",
		LimitPrice:   10.0,
		TargetVolume: 1000,
	}
}

func frozenPlan() *models.TradePlan {
	now := time.Now()
	return &models.TradePlan{
		ID:         1,
		Status:     models.TradePlanStatusReady,
		ApprovedAt: &now,
		ApprovedBy: "gate-test",
		FreezeAt:   &now,
		FreezeBy:   "gate-test",
	}
}

func TestValidateBrokerSubmit_PASS_FrozenValidSpec(t *testing.T) {
	spec := validSpec()
	res := ValidateBrokerSubmit(spec, Context{
		Plan:         frozenPlan(),
		SubmitPrice:  spec.LimitPrice,
		SubmitVolume: spec.TargetVolume,
	})
	require.True(t, res.Allowed)
	require.Empty(t, res.Blockers)
}

func TestValidateBrokerSubmit_FAIL_NotFrozen(t *testing.T) {
	spec := validSpec()
	res := ValidateBrokerSubmit(spec, Context{
		Plan: &models.TradePlan{
			ID:     2,
			Status: models.TradePlanStatusDraft,
		},
		SubmitPrice:  spec.LimitPrice,
		SubmitVolume: spec.TargetVolume,
	})
	require.False(t, res.Allowed)
	require.Contains(t, stringsJoin(res.Blockers), BlockPlanNotFrozen)
}

func TestValidateBrokerSubmit_FAIL_NakedReady(t *testing.T) {
	spec := validSpec()
	res := ValidateBrokerSubmit(spec, Context{
		Plan: &models.TradePlan{
			ID:     3,
			Status: models.TradePlanStatusReady,
			// FreezeAt nil
		},
		SubmitPrice:  spec.LimitPrice,
		SubmitVolume: spec.TargetVolume,
	})
	require.False(t, res.Allowed)
	require.Contains(t, stringsJoin(res.Blockers), BlockPlanNotFrozen)
}

func TestValidateBrokerSubmit_FAIL_NoPrice(t *testing.T) {
	spec := validSpec()
	spec.LimitPrice = 0
	res := ValidateBrokerSubmit(spec, Context{
		Plan:         frozenPlan(),
		SubmitPrice:  10,
		SubmitVolume: spec.TargetVolume,
	})
	require.False(t, res.Allowed)
	require.Contains(t, stringsJoin(res.Blockers), BlockSpecPriceInvalid)
}

func TestValidateBrokerSubmit_FAIL_NoVolume(t *testing.T) {
	spec := validSpec()
	spec.TargetVolume = 0
	res := ValidateBrokerSubmit(spec, Context{
		Plan:         frozenPlan(),
		SubmitPrice:  spec.LimitPrice,
		SubmitVolume: 1000,
	})
	require.False(t, res.Allowed)
	require.Contains(t, stringsJoin(res.Blockers), BlockSpecVolumeInvalid)
}

func TestValidateBrokerSubmit_FAIL_IllegalSpec(t *testing.T) {
	spec := Spec{Symbol: "", Side: "hold", LimitPrice: -1, TargetVolume: 50}
	res := ValidateBrokerSubmit(spec, Context{
		Plan:         frozenPlan(),
		SubmitPrice:  -1,
		SubmitVolume: 50,
	})
	require.False(t, res.Allowed)
	joined := stringsJoin(res.Blockers)
	require.Contains(t, joined, BlockSpecSymbolMissing)
	require.Contains(t, joined, BlockSpecSideInvalid)
	require.Contains(t, joined, BlockSpecPriceInvalid)
	require.Contains(t, joined, BlockSpecVolumeInvalid)
}

func TestValidateBrokerSubmit_FAIL_RuntimeOverride(t *testing.T) {
	spec := validSpec()
	res := ValidateBrokerSubmit(spec, Context{
		Plan:         frozenPlan(),
		SubmitPrice:  12.5, // live reprice
		SubmitVolume: 800,  // recalc volume
	})
	require.False(t, res.Allowed)
	joined := stringsJoin(res.Blockers)
	require.Contains(t, joined, BlockRuntimePriceOverride)
	require.Contains(t, joined, BlockRuntimeVolumeOverride)
}

func TestValidateBrokerSubmit_SkipFrozen_Warns(t *testing.T) {
	spec := validSpec()
	res := ValidateBrokerSubmit(spec, Context{
		SkipFrozenCheck: true,
		SubmitPrice:     spec.LimitPrice,
		SubmitVolume:    spec.TargetVolume,
	})
	require.True(t, res.Allowed)
	require.Contains(t, res.Warnings, WarnNoPlanContext)
}

func TestSpecFromItem(t *testing.T) {
	s := SpecFromItem(models.TradePlanItem{
		StockCode: "SH600000", Side: "", LimitPrice: 9.5, TargetVolume: 200,
	})
	require.Equal(t, "SH600000", s.Symbol)
	require.Equal(t, "buy", s.Side)
	require.Equal(t, 9.5, s.LimitPrice)
	require.Equal(t, int64(200), s.TargetVolume)
}

func stringsJoin(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ";"
		}
		out += s
	}
	return out
}

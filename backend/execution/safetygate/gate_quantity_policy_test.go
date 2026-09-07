package safetygate_test

import (
	"testing"
	"time"

	"go-stock/backend/execution/safetygate"
	"go-stock/backend/models"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func frozenPlan() *models.TradePlan {
	now := time.Now()
	return &models.TradePlan{
		ID: 1, Status: models.TradePlanStatusReady,
		ApprovedAt: &now, ApprovedBy: "m25b", FreezeAt: &now, FreezeBy: "m25b",
	}
}

func submitOK(spec safetygate.Spec) safetygate.Context {
	return safetygate.Context{
		Plan: frozenPlan(), SubmitPrice: spec.LimitPrice, SubmitVolume: spec.TargetVolume,
	}
}

func TestValidateBrokerSubmit_FlagOff_LegacyPercent100(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.False(t, tradingrule.EnableQuantityPolicy())
	require.Equal(t, int64(100), safetygate.DefaultMinLot)

	// STAR 201 fails legacy %100 — old behavior retained when Flag OFF
	spec := safetygate.Spec{Symbol: "sh688981", Side: "buy", LimitPrice: 50, TargetVolume: 201}
	res := safetygate.ValidateBrokerSubmit(spec, submitOK(spec))
	require.False(t, res.Allowed)
	require.Contains(t, res.Blockers, safetygate.BlockSpecVolumeInvalid)

	spec100 := safetygate.Spec{Symbol: "sz000001", Side: "buy", LimitPrice: 10, TargetVolume: 100}
	resOK := safetygate.ValidateBrokerSubmit(spec100, submitOK(spec100))
	require.True(t, resOK.Allowed)
}

func TestValidateBrokerSubmit_FlagOn_MAIN_STAR_ETF_CB(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	cases := []struct {
		name    string
		code    string
		vol     int64
		allowed bool
	}{
		{"MAIN_100", "sz000001", 100, true},
		{"STAR_199", "sh688981", 199, false},
		{"STAR_200", "sh688981", 200, true},
		{"STAR_201", "sh688981", 201, true}, // no longer blocked by %100
		{"ETF_100", "sh510300", 100, true},
		{"CB_10", "sh113052", 10, true},
		{"CB_11", "sh113052", 11, false}, // Validate-only, no normalize
		{"CB_9", "sh113052", 9, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := safetygate.Spec{
				Symbol: tc.code, Side: "buy", LimitPrice: 10, TargetVolume: tc.vol,
			}
			res := safetygate.ValidateBrokerSubmit(spec, submitOK(spec))
			if tc.allowed {
				require.True(t, res.Allowed, "blockers=%v", res.Blockers)
			} else {
				require.False(t, res.Allowed)
				require.Contains(t, res.Blockers, safetygate.BlockSpecVolumeInvalid)
			}
		})
	}
}

func TestValidateBrokerSubmit_FlagOn_DoesNotNormalizeSpec(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	spec := safetygate.Spec{Symbol: "sh113052", Side: "buy", LimitPrice: 120, TargetVolume: 11}
	before := spec.TargetVolume
	res := safetygate.ValidateBrokerSubmit(spec, submitOK(spec))
	require.False(t, res.Allowed)
	require.Equal(t, before, spec.TargetVolume, "SafetyGate must not mutate Spec volume")
}

func TestValidateBrokerSubmit_FlagOn_SellKeepsLegacyLot(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	// Sell path stays on DefaultMinLot multiples (out of buy Policy scope for M2.5-B).
	spec := safetygate.Spec{Symbol: "sh688981", Side: "sell", LimitPrice: 50, TargetVolume: 201}
	res := safetygate.ValidateBrokerSubmit(spec, submitOK(spec))
	require.False(t, res.Allowed)
	require.Contains(t, res.Blockers, safetygate.BlockSpecVolumeInvalid)

	specOK := safetygate.Spec{Symbol: "sz000001", Side: "sell", LimitPrice: 10, TargetVolume: 200}
	resOK := safetygate.ValidateBrokerSubmit(specOK, submitOK(specOK))
	require.True(t, resOK.Allowed)
}

func TestValidateBrokerSubmit_FlagOn_PreservesOtherRiskChecks(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	spec := safetygate.Spec{Symbol: "sz000001", Side: "buy", LimitPrice: 10, TargetVolume: 100}
	res := safetygate.ValidateBrokerSubmit(spec, safetygate.Context{
		Plan: &models.TradePlan{ID: 9, Status: models.TradePlanStatusDraft},
		SubmitPrice: 10, SubmitVolume: 100,
	})
	require.False(t, res.Allowed)
	require.Contains(t, stringsJoin(res.Blockers), safetygate.BlockPlanNotFrozen)
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

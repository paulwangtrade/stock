package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApp_FeatureGateAllow_EnabledDisabledUnknown(t *testing.T) {
	a := &App{}

	require.True(t, a.FeatureGateAllow("pro", "AdvancedRisk"))
	require.False(t, a.FeatureGateAllow("free", "AdvancedRisk"))

	dUnknown := a.FeatureGateCanAccess("enterprise", "NotARealFeature")
	require.False(t, dUnknown.Allowed)
	require.Equal(t, "UNKNOWN_FEATURE", dUnknown.Reason)

	dDisabled := a.FeatureGateCanAccess("free", "Backtest")
	require.False(t, dDisabled.Allowed)
	require.Equal(t, "FEATURE_DISABLED", dDisabled.Reason)

	dOK := a.FeatureGateCanAccess("enterprise", "RealtimeSignal")
	require.True(t, dOK.Allowed)
	require.Equal(t, "OK", dOK.Reason)

	all := a.FeatureGateEvaluateAll("pro")
	require.NotEmpty(t, all)
	require.Equal(t, a.FeatureGateKnownFeatures()[0], all[0].Feature)
}

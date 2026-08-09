package featuregate_test

import (
	"testing"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"

	"github.com/stretchr/testify/require"
)

func setupEntitlements(t *testing.T) entitlement.Service {
	t.Helper()
	svc := entitlement.NewService(entitlement.NewMemoryStore())
	entitlement.SetDefaultForTest(svc)
	t.Cleanup(func() { entitlement.SetDefaultForTest(nil) })
	return svc
}

func TestAllow_EnabledForPro(t *testing.T) {
	svc := setupEntitlements(t)
	user := &featuregate.User{ID: "u1", Tier: featuregate.TierPro}
	require.NoError(t, svc.EnsureTierDefaults(user))

	require.True(t, featuregate.Allow(user, featuregate.FeatureAdvancedRisk))
	require.True(t, featuregate.Allow(user, featuregate.FeatureAIAnalysis))
	require.True(t, featuregate.Allow(user, featuregate.FeatureBacktest))
	require.True(t, featuregate.Allow(user, featuregate.FeatureAdvancedObservation))

	d := featuregate.CanAccess(user, featuregate.FeatureAdvancedRisk)
	require.True(t, d.Allowed)
	require.Equal(t, featuregate.ReasonOK, d.Reason)
	require.Equal(t, featuregate.TierPro, d.Tier)
}

func TestAllow_DisabledForFree(t *testing.T) {
	svc := setupEntitlements(t)
	user := &featuregate.User{ID: "u1", Tier: featuregate.TierFree}
	require.NoError(t, svc.EnsureTierDefaults(user))

	require.False(t, featuregate.Allow(user, featuregate.FeatureAdvancedRisk))
	require.False(t, featuregate.Allow(user, featuregate.FeatureAIAnalysis))
	require.False(t, featuregate.Allow(user, featuregate.FeatureBacktest))
	require.False(t, featuregate.Allow(user, featuregate.FeatureRealtimeSignal))
	require.False(t, featuregate.Allow(user, featuregate.FeatureAdvancedObservation))

	d := featuregate.CanAccess(user, featuregate.FeatureBacktest)
	require.False(t, d.Allowed)
	require.Equal(t, featuregate.ReasonFeatureDisabled, d.Reason)
}

func TestAllow_UnknownFeature(t *testing.T) {
	setupEntitlements(t)
	user := &featuregate.User{ID: "u1", Tier: featuregate.TierEnterprise}
	unknown := featuregate.Feature("NotARealFeature")
	require.False(t, featuregate.Allow(user, unknown))

	d := featuregate.CanAccess(user, unknown)
	require.False(t, d.Allowed)
	require.Equal(t, featuregate.ReasonUnknownFeature, d.Reason)
}

func TestAllow_EnterpriseRealtimeSignal(t *testing.T) {
	svc := setupEntitlements(t)
	ent := &featuregate.User{ID: "e1", Tier: featuregate.TierEnterprise}
	pro := &featuregate.User{ID: "p1", Tier: featuregate.TierPro}
	require.NoError(t, svc.EnsureTierDefaults(ent))
	require.NoError(t, svc.EnsureTierDefaults(pro))
	require.True(t, featuregate.Allow(ent, featuregate.FeatureRealtimeSignal))
	require.False(t, featuregate.Allow(pro, featuregate.FeatureRealtimeSignal))
}

func TestAllow_EnterpriseMultiAccount(t *testing.T) {
	svc := setupEntitlements(t)
	ent := &featuregate.User{ID: "e2", Tier: featuregate.TierEnterprise}
	pro := &featuregate.User{ID: "p2", Tier: featuregate.TierPro}
	require.NoError(t, svc.EnsureTierDefaults(ent))
	require.NoError(t, svc.EnsureTierDefaults(pro))
	require.True(t, featuregate.Allow(ent, featuregate.FeatureMultiAccount))
	require.False(t, featuregate.Allow(pro, featuregate.FeatureMultiAccount))
}

func TestCanAccess_NilUser(t *testing.T) {
	setupEntitlements(t)
	d := featuregate.CanAccess(nil, featuregate.FeatureAdvancedRisk)
	require.False(t, d.Allowed)
	require.Equal(t, featuregate.ReasonNilUser, d.Reason)
}

func TestEvaluateAll_CatalogSize(t *testing.T) {
	svc := setupEntitlements(t)
	user := featuregate.LocalFreeUser()
	require.NoError(t, svc.EnsureTierDefaults(&user))
	all := featuregate.EvaluateAll(&user)
	require.Len(t, all, len(featuregate.KnownFeatures()))
	for _, d := range all {
		require.True(t, featuregate.IsKnown(d.Feature))
		require.False(t, d.Allowed)
	}
}

func TestNormalizeTier_UnknownFallsBackToFree(t *testing.T) {
	require.Equal(t, featuregate.TierFree, featuregate.NormalizeTier(""))
	require.Equal(t, featuregate.TierFree, featuregate.NormalizeTier("gold"))
	require.Equal(t, featuregate.TierPro, featuregate.NormalizeTier(featuregate.TierPro))
}

func TestSourceBoundary_NoTradingImports(t *testing.T) {
	require.NotEmpty(t, featuregate.KnownFeatures())
	require.False(t, featuregate.IsKnown(""))
}

func TestAllow_DoesNotUsePlanDirectlyWithoutEntitlement(t *testing.T) {
	setupEntitlements(t)
	// Pro tier alone must not unlock features.
	user := &featuregate.User{ID: "bare-pro", Tier: featuregate.TierPro}
	require.False(t, featuregate.Allow(user, featuregate.FeatureAdvancedRisk))
}

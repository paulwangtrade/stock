package entitlement_test

import (
	"testing"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"

	"github.com/stretchr/testify/require"
)

func TestActiveEntitlement(t *testing.T) {
	svc := entitlement.NewService(entitlement.NewMemoryStore())
	entitlement.SetDefaultForTest(svc)
	t.Cleanup(func() { entitlement.SetDefaultForTest(nil) })

	user := &featuregate.User{ID: "u-active", Tier: featuregate.TierFree}
	_, err := svc.Grant(user.ID, featuregate.FeatureAdvancedRisk, time.Time{}, entitlement.SourceManual)
	require.NoError(t, err)

	require.True(t, svc.HasFeature(user, featuregate.FeatureAdvancedRisk))
	require.True(t, featuregate.Allow(user, featuregate.FeatureAdvancedRisk),
		"FeatureGate must delegate to Entitlement, not user.Plan")
}

func TestExpiredEntitlement(t *testing.T) {
	svc := entitlement.NewService(entitlement.NewMemoryStore())
	entitlement.SetDefaultForTest(svc)
	t.Cleanup(func() { entitlement.SetDefaultForTest(nil) })

	user := &featuregate.User{ID: "u-expired"}
	past := time.Now().UTC().Add(-time.Hour)
	_, err := svc.Grant(user.ID, featuregate.FeatureBacktest, past, entitlement.SourceTrial)
	require.NoError(t, err)

	e, err := svc.Get(user.ID, featuregate.FeatureBacktest)
	require.NoError(t, err)
	require.False(t, e.IsActive(time.Now().UTC()))
	require.False(t, svc.HasFeature(user, featuregate.FeatureBacktest))
	require.False(t, featuregate.Allow(user, featuregate.FeatureBacktest))
}

func TestMissingEntitlement(t *testing.T) {
	svc := entitlement.NewService(entitlement.NewMemoryStore())
	entitlement.SetDefaultForTest(svc)
	t.Cleanup(func() { entitlement.SetDefaultForTest(nil) })

	user := &featuregate.User{ID: "u-missing", Tier: featuregate.TierPro}
	// Tier on user must NOT grant access by itself — only Entitlement rows do.
	require.False(t, svc.HasFeature(user, featuregate.FeatureAIAnalysis))
	require.False(t, featuregate.Allow(user, featuregate.FeatureAIAnalysis))

	_, err := svc.Get(user.ID, featuregate.FeatureAIAnalysis)
	require.ErrorIs(t, err, entitlement.ErrNotFound)
}

func TestEnsureTierDefaults_ThenGate(t *testing.T) {
	svc := entitlement.NewService(entitlement.NewMemoryStore())
	entitlement.SetDefaultForTest(svc)
	t.Cleanup(func() { entitlement.SetDefaultForTest(nil) })

	user := &featuregate.User{ID: "u-pro", Tier: featuregate.TierPro}
	require.NoError(t, svc.EnsureTierDefaults(user))
	require.True(t, featuregate.Allow(user, featuregate.FeatureAdvancedObservation))
	require.False(t, featuregate.Allow(user, featuregate.FeatureRealtimeSignal))
}

func TestSources_SubscriptionLicenseTrial(t *testing.T) {
	svc := entitlement.NewService(entitlement.NewMemoryStore())
	future := time.Now().UTC().Add(24 * time.Hour)
	_, err := svc.Grant("u1", featuregate.FeatureAdvancedRisk, future, entitlement.SourceSubscription)
	require.NoError(t, err)
	_, err = svc.Grant("u1", featuregate.FeatureBacktest, time.Time{}, entitlement.SourceLicense)
	require.NoError(t, err)
	_, err = svc.Grant("u1", featuregate.FeatureAIAnalysis, future, entitlement.SourceTrial)
	require.NoError(t, err)

	list, err := svc.List("u1")
	require.NoError(t, err)
	require.Len(t, list, 3)
}

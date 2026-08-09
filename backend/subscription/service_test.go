package subscription_test

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/subscription"
	"go-stock/backend/user"

	"github.com/stretchr/testify/require"
)

func localUser(id string) *user.User {
	return &user.User{
		UserID:      id,
		DisplayName: id,
		Kind:        user.KindLocal,
		Status:      user.StatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

func TestActiveSubscription(t *testing.T) {
	svc := subscription.NewService(subscription.NewMemoryStore())
	ctx := context.Background()
	u := localUser("active-1")

	sub, err := svc.AssignPlan(ctx, u.UserID, subscription.PlanPRO, time.Now().UTC().Add(30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, subscription.StatusActive, sub.Status)
	require.Equal(t, subscription.PlanPRO, sub.PlanCode)
	require.False(t, sub.StartsAt.IsZero())
	require.False(t, sub.IsExpired(time.Now().UTC()))

	plan, err := svc.GetCurrentPlan(ctx, u)
	require.NoError(t, err)
	require.Equal(t, subscription.PlanPRO, plan.Code)

	// Permission is not judged here — sync Subscription → Entitlement → FeatureGate.
	require.NoError(t, svc.SyncToFeatureGate(ctx, u))
	fg := &featuregate.User{ID: u.UserID, Tier: featuregate.TierPro}
	require.True(t, entitlement.Default().HasFeature(fg, featuregate.FeatureAdvancedRisk))
	require.True(t, featuregate.Allow(fg, featuregate.FeatureAdvancedRisk))
}

func TestExpiredSubscription(t *testing.T) {
	svc := subscription.NewService(subscription.NewMemoryStore())
	ctx := context.Background()
	u := localUser("expired-1")

	past := time.Now().UTC().Add(-24 * time.Hour)
	sub, err := svc.AssignPlan(ctx, u.UserID, subscription.PlanPRO, past)
	require.NoError(t, err)
	require.Equal(t, subscription.StatusExpired, sub.Status)
	require.True(t, sub.IsExpired(time.Now().UTC()))

	plan, err := svc.GetCurrentPlan(ctx, u)
	require.NoError(t, err)
	require.Equal(t, subscription.PlanFREE, plan.Code)

	ent, err := svc.ResolveEntitlement(ctx, u)
	require.NoError(t, err)
	require.Equal(t, "expired_fallback", ent.Source)
	require.Equal(t, subscription.PlanFREE, ent.PlanCode)

	require.NoError(t, svc.SyncToFeatureGate(ctx, u))
	require.False(t, featuregate.Allow(ent.ToFeatureGateUser(), featuregate.FeatureAdvancedRisk))
}

func TestFreeSubscription(t *testing.T) {
	svc := subscription.NewService(subscription.NewMemoryStore())
	ctx := context.Background()
	u := localUser("free-1")

	// No row → free
	plan, err := svc.GetCurrentPlan(ctx, u)
	require.NoError(t, err)
	require.Equal(t, subscription.PlanFREE, plan.Code)

	sub, err := svc.AssignPlan(ctx, u.UserID, subscription.PlanFREE, time.Time{})
	require.NoError(t, err)
	require.Equal(t, subscription.StatusNone, sub.Status)
	require.Equal(t, subscription.PlanFREE, sub.PlanCode)

	plan, err = svc.GetCurrentPlan(ctx, u)
	require.NoError(t, err)
	require.Equal(t, subscription.PlanFREE, plan.Code)

	ent, err := svc.ResolveEntitlement(ctx, u)
	require.NoError(t, err)
	require.False(t, ent.HasEntitlement(featuregate.FeatureBacktest))
}

func TestProfessionalPlan_MapsToProCatalog(t *testing.T) {
	svc := subscription.NewService(subscription.NewMemoryStore())
	ctx := context.Background()
	u := localUser("pro-fessional")
	_, err := svc.AssignPlan(ctx, u.UserID, subscription.PlanPROFESSIONAL, time.Now().Add(time.Hour))
	require.NoError(t, err)

	plan, err := svc.GetCurrentPlan(ctx, u)
	require.NoError(t, err)
	require.Equal(t, subscription.PlanPROFESSIONAL, plan.Code)

	ent, err := svc.ResolveEntitlement(ctx, u)
	require.NoError(t, err)
	require.True(t, ent.HasEntitlement(featuregate.FeatureAdvancedRisk))
	require.False(t, ent.HasEntitlement(featuregate.FeatureRealtimeSignal))
	require.Equal(t, featuregate.TierPro, ent.ToFeatureGateUser().Tier)
}

func TestCatalog_ContainsAllPlans(t *testing.T) {
	codes := map[subscription.PlanCode]bool{}
	for _, p := range subscription.Catalog() {
		codes[p.Code] = true
	}
	require.True(t, codes[subscription.PlanFREE])
	require.True(t, codes[subscription.PlanPRO])
	require.True(t, codes[subscription.PlanPROFESSIONAL])
	require.True(t, codes[subscription.PlanENTERPRISE])
}

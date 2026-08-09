package user_test

import (
	"context"
	"testing"

	"go-stock/backend/featuregate"
	"go-stock/backend/user"

	"github.com/stretchr/testify/require"
)

func TestEnsureLocalUser_CreatesOnce(t *testing.T) {
	svc := user.NewService(user.NewMemoryStore())
	ctx := context.Background()

	u1, err := svc.EnsureLocalUser(ctx)
	require.NoError(t, err)
	require.Equal(t, user.DefaultLocalUserID, u1.ID())
	require.Equal(t, user.DefaultLocalDisplayName, u1.DisplayName)
	require.Equal(t, user.KindLocal, u1.Kind)
	require.Equal(t, user.StatusActive, u1.Status)
	require.False(t, u1.CreatedAt.IsZero())
	require.True(t, u1.IsLocal())
	require.False(t, u1.IsCloud())

	u2, err := svc.EnsureLocalUser(ctx)
	require.NoError(t, err)
	require.Equal(t, u1.UserID, u2.UserID)
	require.Equal(t, u1.CreatedAt, u2.CreatedAt)
}

func TestGetProfile_PreferencesAndSettings(t *testing.T) {
	svc := user.NewService(user.NewMemoryStore())
	ctx := context.Background()
	u, err := svc.EnsureLocalUser(ctx)
	require.NoError(t, err)

	p, err := svc.GetProfile(ctx, u.UserID)
	require.NoError(t, err)
	require.NotNil(t, p.Preferences)
	require.NotNil(t, p.Settings)

	name := "Desk Trader"
	got, err := svc.UpdateProfile(ctx, u.UserID, user.ProfilePatch{
		DisplayName: &name,
		Preferences: map[string]string{"theme": "dark"},
		Settings:    map[string]string{"locale": "zh-CN"},
	})
	require.NoError(t, err)
	require.Equal(t, name, got.DisplayName)
	require.Equal(t, "dark", got.Pref("theme"))
	require.Equal(t, "zh-CN", got.Setting("locale"))
}

func TestCloudUserStub_DomainOnly(t *testing.T) {
	u := user.NewCloudUserStub("cloud-1", "Cloud User", "oidc|sub-1")
	require.True(t, u.IsCloud())
	require.Equal(t, user.KindCloud, u.Kind)
	require.False(t, u.CreatedAt.IsZero())

	store := user.NewMemoryStore()
	require.NoError(t, store.SaveUser(context.Background(), u))
	got, err := store.GetUser(context.Background(), "cloud-1")
	require.NoError(t, err)
	require.Equal(t, u.DisplayName, got.DisplayName)
}

func TestFeatureGateSubject_CommercialChain(t *testing.T) {
	u := &user.User{UserID: "local-device", Status: user.StatusActive}
	sub := u.FeatureGateSubject()
	require.NotNil(t, sub)
	require.Equal(t, "local-device", sub.ID)
	require.Equal(t, featuregate.TierFree, sub.Tier)
}

func TestGetUser_NotFound(t *testing.T) {
	svc := user.NewService(user.NewMemoryStore())
	_, err := svc.GetUser(context.Background(), "missing")
	require.ErrorIs(t, err, user.ErrNotFound)
}

func TestUpdateProfile_EmptyDisplayNameRejected(t *testing.T) {
	svc := user.NewService(user.NewMemoryStore())
	ctx := context.Background()
	_, err := svc.EnsureLocalUser(ctx)
	require.NoError(t, err)
	empty := "   "
	_, err = svc.UpdateProfile(ctx, user.DefaultLocalUserID, user.ProfilePatch{DisplayName: &empty})
	require.Error(t, err)
}

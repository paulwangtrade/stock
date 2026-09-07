package tradingrule_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestQuantityPolicyConfig_DefaultDisabled(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	tradingrule.ResetQuantityPolicyConfigCache()
	t.Cleanup(func() {
		tradingrule.ResetEnableQuantityPolicyForTest()
		tradingrule.ResetQuantityPolicyConfigCache()
	})

	// Ensure no override; use in-memory config default.
	tradingrule.SetQuantityPolicyConfigForTest(tradingrule.QuantityPolicyConfig{EnableQuantityPolicy: false})
	require.False(t, tradingrule.EnableQuantityPolicy())
}

func TestQuantityPolicyConfig_FileEnable(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	tradingrule.ResetQuantityPolicyConfigCache()
	t.Cleanup(func() {
		tradingrule.ResetEnableQuantityPolicyForTest()
		tradingrule.ResetQuantityPolicyConfigCache()
		_ = os.Remove(tradingrule.QuantityPolicyConfigPath())
	})

	dir := "data"
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "quantity_policy.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"enableQuantityPolicy":true}`), 0o644))
	tradingrule.ResetQuantityPolicyConfigCache()

	require.True(t, tradingrule.EnableQuantityPolicy())

	// Override still wins for tests
	tradingrule.SetEnableQuantityPolicyForTest(false)
	require.False(t, tradingrule.EnableQuantityPolicy())
}

func TestQuantityPolicyConfig_SaveRoundTrip(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(func() {
		tradingrule.ResetEnableQuantityPolicyForTest()
		tradingrule.ResetQuantityPolicyConfigCache()
		_ = os.Remove(tradingrule.QuantityPolicyConfigPath())
	})

	require.NoError(t, tradingrule.SaveQuantityPolicyConfig(tradingrule.QuantityPolicyConfig{
		EnableQuantityPolicy: true,
	}))
	tradingrule.ResetQuantityPolicyConfigCache()
	cfg := tradingrule.GetQuantityPolicyConfig()
	require.True(t, cfg.EnableQuantityPolicy)

	// Restore safe default for other tests sharing cwd
	require.NoError(t, tradingrule.SaveQuantityPolicyConfig(tradingrule.QuantityPolicyConfig{
		EnableQuantityPolicy: false,
	}))
}

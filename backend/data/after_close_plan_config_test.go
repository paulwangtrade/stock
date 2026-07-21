package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAfterClosePlanConfig_DefaultDisabled(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		ResetAfterClosePlanConfigCache()
	})
	ResetAfterClosePlanConfigCache()

	cfg := GetAfterClosePlanConfig()
	require.False(t, cfg.AfterClosePlanEnabled)
	require.False(t, IsAfterClosePlanEnabled())
}

func TestAfterClosePlanConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		ResetAfterClosePlanConfigCache()
	})
	ResetAfterClosePlanConfigCache()

	require.NoError(t, SaveAfterClosePlanConfig(AfterClosePlanConfig{AfterClosePlanEnabled: true}))
	ResetAfterClosePlanConfigCache()
	require.True(t, IsAfterClosePlanEnabled())

	raw, err := os.ReadFile(filepath.Join("data", afterClosePlanConfigFile))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"after_close_plan_enabled": true`)
}

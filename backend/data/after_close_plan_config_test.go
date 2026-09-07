package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const shippedAfterClosePlanJSON = `{
  "after_close_plan_enabled": true
}`

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

func TestAfterClosePlanConfig_LoadShippedDefaultJSON(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		ResetAfterClosePlanConfigCache()
	})
	ResetAfterClosePlanConfigCache()

	require.NoError(t, os.MkdirAll("data", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("data", afterClosePlanConfigFile), []byte(shippedAfterClosePlanJSON), 0o644))

	cfg := GetAfterClosePlanConfig()
	require.True(t, cfg.AfterClosePlanEnabled)
	require.True(t, IsAfterClosePlanEnabled())
}

func TestAfterClosePlanConfig_PathRelativeToWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		ResetAfterClosePlanConfigCache()
	})
	ResetAfterClosePlanConfigCache()

	require.Equal(t, filepath.Join("data", afterClosePlanConfigFile), AfterClosePlanConfigPath())

	require.NoError(t, os.MkdirAll("data", 0o755))
	raw, _ := json.MarshalIndent(AfterClosePlanConfig{AfterClosePlanEnabled: true}, "", "  ")
	require.NoError(t, os.WriteFile(AfterClosePlanConfigPath(), raw, 0o644))

	require.True(t, IsAfterClosePlanEnabled())
}

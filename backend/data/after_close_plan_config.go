package data

import (
	"encoding/json"
	"os"
	"sync"

	runtimeutil "go-stock/backend/runtime"
)

const afterClosePlanConfigFile = "after_close_plan.json"

// AfterClosePlanConfig controls the after-close Candidate→Draft→Risk cron.
// It does not enable Approve, Freeze, or Execution.
type AfterClosePlanConfig struct {
	AfterClosePlanEnabled bool `json:"after_close_plan_enabled"`
}

var (
	afterClosePlanMu     sync.RWMutex
	afterClosePlanCached *AfterClosePlanConfig
)

func defaultAfterClosePlanConfig() AfterClosePlanConfig {
	return AfterClosePlanConfig{AfterClosePlanEnabled: false}
}

func afterClosePlanConfigPath() string {
	return runtimeutil.GetConfigPath(afterClosePlanConfigFile)
}

// AfterClosePlanConfigPath is the cwd-relative path used at runtime (e.g. build/bin/data/ when exe cwd is build/bin).
func AfterClosePlanConfigPath() string {
	return afterClosePlanConfigPath()
}

// GetAfterClosePlanConfig reads data/after_close_plan.json relative to process cwd.
// Missing file → enabled=false. Shipped default: repo data/after_close_plan.json (copy to build/bin/data for packaged exe).
func GetAfterClosePlanConfig() AfterClosePlanConfig {
	afterClosePlanMu.RLock()
	if afterClosePlanCached != nil {
		cfg := *afterClosePlanCached
		afterClosePlanMu.RUnlock()
		return cfg
	}
	afterClosePlanMu.RUnlock()

	cfg := defaultAfterClosePlanConfig()
	raw, err := os.ReadFile(afterClosePlanConfigPath())
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}

	afterClosePlanMu.Lock()
	afterClosePlanCached = &cfg
	afterClosePlanMu.Unlock()
	return cfg
}

// IsAfterClosePlanEnabled reports whether the after-close cron should run the workflow.
func IsAfterClosePlanEnabled() bool {
	return GetAfterClosePlanConfig().AfterClosePlanEnabled
}

// SaveAfterClosePlanConfig writes data/after_close_plan.json and refreshes cache.
func SaveAfterClosePlanConfig(cfg AfterClosePlanConfig) error {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(afterClosePlanConfigPath(), raw, 0o644); err != nil {
		return err
	}
	afterClosePlanMu.Lock()
	afterClosePlanCached = &cfg
	afterClosePlanMu.Unlock()
	return nil
}

// ResetAfterClosePlanConfigCache clears the in-memory config cache (tests).
func ResetAfterClosePlanConfigCache() {
	afterClosePlanMu.Lock()
	afterClosePlanCached = nil
	afterClosePlanMu.Unlock()
}

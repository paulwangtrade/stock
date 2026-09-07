package tradingrule

import (
	"encoding/json"
	"os"
	"sync"

	runtimeutil "go-stock/backend/runtime"
)

const quantityPolicyConfigFile = "quantity_policy.json"

// QuantityPolicyConfig is the controlled-enable config (Phase12-M2.4).
// Default production strategy: EnableQuantityPolicy=false (legacy /100 + blind Spec).
type QuantityPolicyConfig struct {
	// EnableQuantityPolicy turns on Materialize Policy sizing + Broker Validate-only.
	EnableQuantityPolicy bool `json:"enableQuantityPolicy"`
}

var (
	qtyCfgMu     sync.RWMutex
	qtyCfgCached *QuantityPolicyConfig

	qtyOverrideMu sync.RWMutex
	qtyOverride   *bool // non-nil → test/forced override of EnableQuantityPolicy()
)

func defaultQuantityPolicyConfig() QuantityPolicyConfig {
	return QuantityPolicyConfig{EnableQuantityPolicy: false}
}

func quantityPolicyConfigPath() string {
	return runtimeutil.GetConfigPath(quantityPolicyConfigFile)
}

// QuantityPolicyConfigPath is the cwd-relative runtime path.
func QuantityPolicyConfigPath() string { return quantityPolicyConfigPath() }

// GetQuantityPolicyConfig loads data/quantity_policy.json (missing → disabled defaults).
func GetQuantityPolicyConfig() QuantityPolicyConfig {
	qtyCfgMu.RLock()
	if qtyCfgCached != nil {
		c := *qtyCfgCached
		qtyCfgMu.RUnlock()
		return c
	}
	qtyCfgMu.RUnlock()

	c := defaultQuantityPolicyConfig()
	raw, err := os.ReadFile(quantityPolicyConfigPath())
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}

	qtyCfgMu.Lock()
	qtyCfgCached = &c
	qtyCfgMu.Unlock()
	return c
}

// SaveQuantityPolicyConfig writes config and refreshes cache (ops / controlled enable).
func SaveQuantityPolicyConfig(c QuantityPolicyConfig) error {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(quantityPolicyConfigPath(), raw, 0o644); err != nil {
		return err
	}
	qtyCfgMu.Lock()
	copy := c
	qtyCfgCached = &copy
	qtyCfgMu.Unlock()
	return nil
}

// ResetQuantityPolicyConfigCache clears the file-backed cache (tests).
func ResetQuantityPolicyConfigCache() {
	qtyCfgMu.Lock()
	qtyCfgCached = nil
	qtyCfgMu.Unlock()
}

// SetQuantityPolicyConfigForTest injects config without touching disk.
func SetQuantityPolicyConfigForTest(c QuantityPolicyConfig) {
	qtyCfgMu.Lock()
	copy := c
	qtyCfgCached = &copy
	qtyCfgMu.Unlock()
}

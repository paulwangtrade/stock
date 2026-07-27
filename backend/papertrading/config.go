package papertrading

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const configFile = "paper_trading_mvp.json"

// DefaultInitialCash is the seed cash for the default paper-sim account.
const DefaultInitialCash = 1_000_000.0

// Config is the Phase6.6 Paper Trading MVP feature switch.
// It is intentionally isolated from paper_open_buy.json / after_close_plan.json
// so that toggling it cannot change any existing (real Execution) behavior.
type Config struct {
	EnablePaperTrading bool    `json:"enablePaperTrading"`
	InitialCash        float64 `json:"initialCash,omitempty"`
}

var (
	cfgMu     sync.RWMutex
	cfgCached *Config
)

func defaultConfig() Config {
	return Config{EnablePaperTrading: false, InitialCash: DefaultInitialCash}
}

func configPath() string {
	return filepath.Join("data", configFile)
}

// GetConfig reads data/paper_trading_mvp.json (default: disabled).
func GetConfig() Config {
	cfgMu.RLock()
	if cfgCached != nil {
		c := *cfgCached
		cfgMu.RUnlock()
		return c
	}
	cfgMu.RUnlock()

	cfg := defaultConfig()
	raw, err := os.ReadFile(configPath())
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}
	if cfg.InitialCash <= 0 {
		cfg.InitialCash = DefaultInitialCash
	}

	cfgMu.Lock()
	cfgCached = &cfg
	cfgMu.Unlock()
	return cfg
}

// IsEnabled reports whether Paper Trading MVP is switched on. Default false.
func IsEnabled() bool {
	return GetConfig().EnablePaperTrading
}

// SaveConfig writes the config file and refreshes cache (used by tests / future UI).
func SaveConfig(cfg Config) error {
	if cfg.InitialCash <= 0 {
		cfg.InitialCash = DefaultInitialCash
	}
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath(), raw, 0o644); err != nil {
		return err
	}
	cfgMu.Lock()
	cfgCached = &cfg
	cfgMu.Unlock()
	return nil
}

// ResetConfigCache clears the in-memory config cache (tests).
func ResetConfigCache() {
	cfgMu.Lock()
	cfgCached = nil
	cfgMu.Unlock()
}

// SetConfigForTest injects a config into the cache without touching disk (tests).
func SetConfigForTest(cfg Config) {
	if cfg.InitialCash <= 0 {
		cfg.InitialCash = DefaultInitialCash
	}
	cfgMu.Lock()
	cfgCached = &cfg
	cfgMu.Unlock()
}

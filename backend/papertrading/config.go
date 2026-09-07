package papertrading

import (
	"encoding/json"
	"os"
	"sync"

	runtimeutil "go-stock/backend/runtime"
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
	// FillMode selects exclusive Fill cron (Phase10-C.4-A).
	// "A" (default): 09:31 Session A open fill.
	// "B": 15:10 Session B close fill (observation sampling).
	// Empty / unknown → A. Does not bypass Session Policy; only which cron is registered.
	FillMode string `json:"fillMode,omitempty"`
	// PositionSizerMode selects Draft buy amount: "fixed_amount" (default) | "portfolio_aware".
	// Omitted from old JSON files → empty → normalized to fixed_amount (behavior unchanged).
	PositionSizerMode string `json:"positionSizerMode,omitempty"`
	// PortfolioAware is ignored unless PositionSizerMode=portfolio_aware.
	PortfolioAware *PortfolioAwareConfig `json:"portfolioAware,omitempty"`
}

// PortfolioAwareConfig is the P2 buy-budget policy (only used when mode=portfolio_aware).
type PortfolioAwareConfig struct {
	MaxExposure             float64 `json:"maxExposure,omitempty"`
	MaxSinglePositionWeight float64 `json:"maxSinglePositionWeight,omitempty"`
	ReserveCashRatio        float64 `json:"reserveCashRatio,omitempty"`
	MinOrderAmount          float64 `json:"minOrderAmount,omitempty"`
}

var (
	cfgMu     sync.RWMutex
	cfgCached *Config
)

func defaultConfig() Config {
	return Config{
		EnablePaperTrading: false,
		InitialCash:        DefaultInitialCash,
		FillMode:           FillModeA,
		PositionSizerMode:  PositionSizerModeFixedAmount,
	}
}

func configPath() string {
	return runtimeutil.GetConfigPath(configFile)
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
	cfg.FillMode = NormalizeFillMode(cfg.FillMode)
	cfg.PositionSizerMode = NormalizePositionSizerMode(cfg.PositionSizerMode)

	cfgMu.Lock()
	cfgCached = &cfg
	cfgMu.Unlock()
	return cfg
}

// SaveConfig writes the config file and refreshes cache (used by tests / future UI).
func SaveConfig(cfg Config) error {
	if cfg.InitialCash <= 0 {
		cfg.InitialCash = DefaultInitialCash
	}
	cfg.FillMode = NormalizeFillMode(cfg.FillMode)
	cfg.PositionSizerMode = NormalizePositionSizerMode(cfg.PositionSizerMode)
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
	cfg.FillMode = NormalizeFillMode(cfg.FillMode)
	cfg.PositionSizerMode = NormalizePositionSizerMode(cfg.PositionSizerMode)
	cfgMu.Lock()
	cfgCached = &cfg
	cfgMu.Unlock()
}

// GetConfig still reads/caches paper_trading_mvp.json for tests and SaveConfig.
// Business enable/fill/cash reads should prefer tradingconfig.Default() (wired via RegisterPaperMVPLoader).

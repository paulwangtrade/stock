package tradingautomation

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	runtimeutil "go-stock/backend/runtime"
)

const configFile = "trading_automation.json"

// Mode values for Trading Automation Policy.
const (
	ModeManual   = "MANUAL"
	ModeAssisted = "ASSISTED"
	ModeAuto     = "AUTO"
)

// Config controls morning T1–T2 automation (Phase10-C.8.2).
// Default MANUAL preserves existing UI-only flow.
type Config struct {
	AutomationMode string `json:"automation_mode"`
	MaterializeTime string `json:"materialize_time"`
	ApprovalTime    string `json:"approval_time"`
	FreezeTime      string `json:"freeze_time"`
	FreezeDeadline  string `json:"freeze_deadline"`
}

var (
	cfgMu     sync.RWMutex
	cfgCached *Config
)

func defaultConfig() Config {
	return Config{
		AutomationMode:  ModeManual,
		MaterializeTime: "09:20",
		ApprovalTime:    "09:25",
		FreezeTime:      "09:29:00",
		FreezeDeadline:  "09:29:30",
	}
}

func configPath() string {
	return runtimeutil.GetConfigPath(configFile)
}

// ConfigPath is the cwd-relative runtime path.
func ConfigPath() string { return configPath() }

// GetConfig loads data/trading_automation.json (missing → MANUAL defaults).
func GetConfig() Config {
	cfgMu.RLock()
	if cfgCached != nil {
		c := *cfgCached
		cfgMu.RUnlock()
		return normalizeConfig(c)
	}
	cfgMu.RUnlock()

	c := defaultConfig()
	raw, err := os.ReadFile(configPath())
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}
	c = normalizeConfig(c)

	cfgMu.Lock()
	cfgCached = &c
	cfgMu.Unlock()
	return c
}

// SaveConfig writes config and refreshes cache.
func SaveConfig(c Config) error {
	c = normalizeConfig(c)
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath(), raw, 0o644); err != nil {
		return err
	}
	cfgMu.Lock()
	copy := c
	cfgCached = &copy
	cfgMu.Unlock()
	return nil
}

// ResetConfigCache clears cached config (tests).
func ResetConfigCache() {
	cfgMu.Lock()
	cfgCached = nil
	cfgMu.Unlock()
}

// SetConfigForTest sets in-memory config without writing trading_automation.json.
func SetConfigForTest(c Config) {
	c = normalizeConfig(c)
	cfgMu.Lock()
	copy := c
	cfgCached = &copy
	cfgMu.Unlock()
}

func normalizeConfig(c Config) Config {
	mode := strings.ToUpper(strings.TrimSpace(c.AutomationMode))
	switch mode {
	case ModeAssisted, ModeAuto:
		c.AutomationMode = mode
	default:
		c.AutomationMode = ModeManual
	}
	if strings.TrimSpace(c.MaterializeTime) == "" {
		c.MaterializeTime = "09:20"
	}
	if strings.TrimSpace(c.ApprovalTime) == "" {
		c.ApprovalTime = "09:25"
	}
	if strings.TrimSpace(c.FreezeTime) == "" {
		c.FreezeTime = "09:29:00"
	}
	if strings.TrimSpace(c.FreezeDeadline) == "" {
		c.FreezeDeadline = "09:29:30"
	}
	return c
}

func Mode() string { return GetConfig().AutomationMode }

func ShouldAutoMaterialize() bool {
	m := Mode()
	return m == ModeAssisted || m == ModeAuto
}

func ShouldAutoApprove() bool { return Mode() == ModeAuto }

func ShouldAutoFreeze() bool { return Mode() == ModeAuto }

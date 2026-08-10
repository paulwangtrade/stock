package tradingconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

// LegacyAdapter loads TradingConfig fields from existing JSON stores
// without changing their on-disk formats.
//
// Priority for Phase6.5 / H.4 business values:
//   paper_open_buy / after_close_plan / paper_trading_mvp  (authoritative files)
// Automation is loaded for observability only and must not override Phase6 values.
type LegacyAdapter struct {
	risk *LegacyRiskConfigAdapter
}

// PaperMVPLoader optionally supplies Track-B MVP config (registered by papertrading to avoid import cycles).
// When nil, LegacyAdapter reads data/paper_trading_mvp.json directly (same defaults).
type PaperMVPLoader func() PaperMVPView

var (
	mvpLoaderMu sync.RWMutex
	mvpLoader   PaperMVPLoader
)

// RegisterPaperMVPLoader wires Track-B config into the Provider (papertrading init).
func RegisterPaperMVPLoader(fn PaperMVPLoader) {
	mvpLoaderMu.Lock()
	defer mvpLoaderMu.Unlock()
	mvpLoader = fn
}

// Load returns a resolved snapshot from legacy sources.
func (a *LegacyAdapter) Load() Resolved {
	if a == nil {
		a = &LegacyAdapter{}
	}
	if a.risk == nil {
		a.risk = &LegacyRiskConfigAdapter{}
	}
	return Resolved{
		Position:   a.loadPosition(),
		Workflow:   a.loadWorkflow(),
		Execution:  a.loadExecution(),
		PaperMVP:   a.loadPaperMVP(),
		Risk:       a.risk.Load(),
		Automation: a.loadAutomation(),
	}
}

func (a *LegacyAdapter) loadPosition() PositionView {
	cfg := data.GetPaperOpenBuyConfig()
	amount := cfg.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = DefaultFixedAmount
	}
	return PositionView{
		SizingMethod:      SizingMethodFixedAmount,
		MaxPositionAmount: amount,
		Source:            SourceLegacyPaperOpenBuy,
	}
}

func (a *LegacyAdapter) loadExecution() ExecutionSwitchView {
	cfg := data.GetPaperOpenBuyConfig()
	return ExecutionSwitchView{
		EnablePaperOpenBuy: cfg.EnablePaperOpenBuy,
		Source:             SourceLegacyPaperOpenBuy,
	}
}

func (a *LegacyAdapter) loadWorkflow() WorkflowView {
	return WorkflowView{
		AfterCloseEnabled: data.IsAfterClosePlanEnabled(),
		Source:            SourceLegacyAfterClose,
	}
}

func (a *LegacyAdapter) loadPaperMVP() PaperMVPView {
	mvpLoaderMu.RLock()
	fn := mvpLoader
	mvpLoaderMu.RUnlock()
	if fn != nil {
		v := fn()
		if v.Source == "" {
			v.Source = SourceLegacyPaperMVP
		}
		if v.InitialCash <= 0 {
			v.InitialCash = DefaultPaperInitialCash
		}
		if strings.TrimSpace(v.FillMode) == "" {
			v.FillMode = DefaultFillMode
		}
		return v
	}
	return loadPaperMVPFromJSONFile()
}

func loadPaperMVPFromJSONFile() PaperMVPView {
	out := PaperMVPView{
		EnablePaperTrading: false,
		InitialCash:        DefaultPaperInitialCash,
		FillMode:           DefaultFillMode,
		Source:             SourceLegacyPaperMVP,
	}
	raw, err := os.ReadFile(filepath.Join("data", "paper_trading_mvp.json"))
	if err != nil || len(raw) == 0 {
		return out
	}
	var parsed struct {
		EnablePaperTrading bool    `json:"enablePaperTrading"`
		InitialCash        float64 `json:"initialCash"`
		FillMode           string  `json:"fillMode"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return out
	}
	out.EnablePaperTrading = parsed.EnablePaperTrading
	if parsed.InitialCash > 0 {
		out.InitialCash = parsed.InitialCash
	}
	fm := strings.ToUpper(strings.TrimSpace(parsed.FillMode))
	if fm == "B" {
		out.FillMode = "B"
	} else {
		out.FillMode = DefaultFillMode
	}
	return out
}

func (a *LegacyAdapter) loadAutomation() AutomationView {
	out := AutomationView{Source: SourceLegacyAutomation}
	// Observability-only in Phase6.5-A. Skip when DB is not ready (unit tests / early boot).
	if db.Dao == nil {
		return out
	}
	cfg := data.GetSettingConfig()
	if cfg == nil || strings.TrimSpace(cfg.SignalParams) == "" {
		return out
	}
	var parsed struct {
		Automation map[string]any `json:"automation"`
	}
	if err := json.Unmarshal([]byte(cfg.SignalParams), &parsed); err != nil {
		return out
	}
	if parsed.Automation == nil {
		return out
	}
	out.Present = true
	out.Enabled = asBool(parsed.Automation["enabled"])
	out.AccountEquity = asFloat(parsed.Automation["accountEquity"])
	out.RiskPerTradePct = asFloat(parsed.Automation["riskPerTradePct"])
	out.MaxPositionPct = asFloat(parsed.Automation["maxPositionPct"])
	out.MaxTotalExposurePct = asFloat(parsed.Automation["maxTotalExposurePct"])
	out.ScanIntervalMinutes = asInt(parsed.Automation["scanIntervalMinutes"])
	out.AlertCooldownMinutes = asInt(parsed.Automation["alertCooldownMinutes"])
	return out
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	default:
		return false
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		return 0
	}
}

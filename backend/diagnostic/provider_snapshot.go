package diagnostic

import (
	"strings"

	"go-stock/backend/controlledadoption"
	"go-stock/backend/models"
)

// ProviderModeSnapshot exposes adoption enums and whitelist counts only.
type ProviderModeSnapshot struct {
	Adoption                 string `json:"adoption"`
	KillSwitch               bool   `json:"kill_switch"`
	AccountWhitelistCount    int    `json:"account_whitelist_count"`
	StrategyWhitelistCount   int    `json:"strategy_whitelist_count"`
	DateWhitelistCount       int    `json:"date_whitelist_count"`
	LastPlanProviderMode     string `json:"last_plan_provider_mode,omitempty"`
	LastPlanDecisionProvider string `json:"last_plan_decision_provider,omitempty"`
}

var allowedAdoptions = map[string]struct{}{
	controlledadoption.AdoptionOff:        {},
	controlledadoption.AdoptionShadow:     {},
	controlledadoption.AdoptionControlled: {},
}

var allowedPlanProviderModes = map[string]struct{}{
	models.TradePlanProviderModeOff:        {},
	models.TradePlanProviderModeShadow:     {},
	models.TradePlanProviderModeSimulation: {},
	models.TradePlanProviderModeControlled: {},
	models.TradePlanProviderModeOn:         {},
}

var allowedDecisionProviders = map[string]struct{}{
	models.TradePlanDecisionProviderFixed:     {},
	models.TradePlanDecisionProviderPortfolio: {},
}

// SnapshotProviderMode builds a diagnostic-safe provider projection.
func SnapshotProviderMode(planProviderMode, planDecisionProvider string) ProviderModeSnapshot {
	p := controlledadoption.Active()
	out := ProviderModeSnapshot{
		Adoption:               normalizeAdoption(p.Adoption),
		KillSwitch:             p.KillSwitch,
		AccountWhitelistCount:  len(p.AccountIDs),
		StrategyWhitelistCount: len(p.StrategyNames),
		DateWhitelistCount:     len(p.TradeDates),
		LastPlanProviderMode:     normalizePlanProviderMode(planProviderMode),
		LastPlanDecisionProvider: normalizeDecisionProvider(planDecisionProvider),
	}
	return out
}

func normalizeAdoption(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := allowedAdoptions[v]; ok {
		return v
	}
	return controlledadoption.AdoptionOff
}

func normalizePlanProviderMode(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := allowedPlanProviderModes[v]; ok {
		return v
	}
	return ""
}

func normalizeDecisionProvider(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := allowedDecisionProviders[v]; ok {
		return v
	}
	return ""
}

package controlledadoption

import (
	"strings"
	"sync"
)

const (
	AdoptionOff        = "off"
	AdoptionShadow     = "shadow"
	AdoptionControlled = "controlled"

	ReasonAdoptionOff          = "adoption_off"
	ReasonKillSwitch           = "kill_switch"
	ReasonEmptyWhitelist       = "empty_whitelist_fail_closed"
	ReasonAccountMiss          = "account_not_in_whitelist"
	ReasonStrategyMiss         = "strategy_not_in_whitelist"
	ReasonStrategyAmbiguous    = "strategy_ambiguous"
	ReasonDateMiss             = "trade_date_not_in_whitelist"
	ReasonMatched              = "whitelist_matched"
)

// ControlledProviderPolicy is the G.8 adoption gate. Production default is OFF.
// Match requires account ∩ strategy ∩ date (all three lists non-empty + membership).
type ControlledProviderPolicy struct {
	// Adoption: off | shadow | controlled. Only "controlled" may select Portfolio.
	Adoption string `json:"adoption"`
	// KillSwitch, when true, forces Legacy for all new Drafts (next vis).
	KillSwitch bool `json:"kill_switch"`
	// AccountIDs must be non-empty for any match when Adoption=controlled.
	AccountIDs []uint `json:"account_ids"`
	// StrategyNames must be non-empty for any match (small-scope: explicit list).
	StrategyNames []string `json:"strategy_names"`
	// TradeDates are YYYY-MM-DD; must be non-empty for any match.
	TradeDates []string `json:"trade_dates"`
}

// MatchInput is one Draft-vis evaluation keyset.
type MatchInput struct {
	AccountID    uint
	StrategyName string
	TradeDate    string
}

// MatchResult is the Decide-time routing decision (not a TradePlan).
type MatchResult struct {
	Matched bool   `json:"matched"`
	Reason  string `json:"reason"`
}

// DefaultPolicy returns production defaults: adoption=off, kill_switch=false, empty lists.
func DefaultPolicy() ControlledProviderPolicy {
	return ControlledProviderPolicy{
		Adoption:      AdoptionOff,
		KillSwitch:    false,
		AccountIDs:    nil,
		StrategyNames: nil,
		TradeDates:    nil,
	}
}

var (
	policyMu sync.RWMutex
	active   = DefaultPolicy()
)

// Active returns a copy of the process policy.
func Active() ControlledProviderPolicy {
	policyMu.RLock()
	defer policyMu.RUnlock()
	return clonePolicy(active)
}

// SetActive replaces the process policy (tests / small-scope ops). Not a cron auto-switch.
func SetActive(p ControlledProviderPolicy) {
	policyMu.Lock()
	defer policyMu.Unlock()
	active = clonePolicy(p)
}

// ResetActive restores DefaultPolicy (tests).
func ResetActive() {
	SetActive(DefaultPolicy())
}

// ResolveMatch evaluates account ∩ strategy ∩ date under the given policy.
func ResolveMatch(p ControlledProviderPolicy, in MatchInput) MatchResult {
	adoption := strings.ToLower(strings.TrimSpace(p.Adoption))
	if adoption == "" {
		adoption = AdoptionOff
	}
	if p.KillSwitch {
		return MatchResult{Matched: false, Reason: ReasonKillSwitch}
	}
	if adoption != AdoptionControlled {
		return MatchResult{Matched: false, Reason: ReasonAdoptionOff}
	}
	if len(p.AccountIDs) == 0 || len(p.StrategyNames) == 0 || len(p.TradeDates) == 0 {
		return MatchResult{Matched: false, Reason: ReasonEmptyWhitelist}
	}
	if !accountAllowed(p.AccountIDs, in.AccountID) {
		return MatchResult{Matched: false, Reason: ReasonAccountMiss}
	}
	strat := strings.TrimSpace(in.StrategyName)
	if strat == "" {
		return MatchResult{Matched: false, Reason: ReasonStrategyMiss}
	}
	if !strategyAllowed(p.StrategyNames, strat) {
		return MatchResult{Matched: false, Reason: ReasonStrategyMiss}
	}
	date := strings.TrimSpace(in.TradeDate)
	if date == "" || !dateAllowed(p.TradeDates, date) {
		return MatchResult{Matched: false, Reason: ReasonDateMiss}
	}
	return MatchResult{Matched: true, Reason: ReasonMatched}
}

// ResolveActive is ResolveMatch against the process policy.
func ResolveActive(in MatchInput) MatchResult {
	return ResolveMatch(Active(), in)
}

func accountAllowed(ids []uint, id uint) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func strategyAllowed(names []string, name string) bool {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, n := range names {
		if strings.ToLower(strings.TrimSpace(n)) == want {
			return true
		}
	}
	return false
}

func dateAllowed(dates []string, tradeDate string) bool {
	want := strings.TrimSpace(tradeDate)
	for _, d := range dates {
		if strings.TrimSpace(d) == want {
			return true
		}
	}
	return false
}

func clonePolicy(p ControlledProviderPolicy) ControlledProviderPolicy {
	out := p
	if p.AccountIDs != nil {
		out.AccountIDs = append([]uint(nil), p.AccountIDs...)
	}
	if p.StrategyNames != nil {
		out.StrategyNames = append([]string(nil), p.StrategyNames...)
	}
	if p.TradeDates != nil {
		out.TradeDates = append([]string(nil), p.TradeDates...)
	}
	return out
}

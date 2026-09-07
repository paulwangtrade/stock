package dailypilot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func hash8(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('|')
		}
		b.WriteString(p)
	}
	if b.Len() == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(b.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:8])
}

func policyPresent(p PolicySnapshotInput) bool {
	return strings.TrimSpace(p.Adoption) != "" ||
		p.KillSwitch ||
		len(p.AccountIDs) > 0 ||
		len(p.StrategyNames) > 0 ||
		len(p.TradeDates) > 0
}

func projectScope(p PolicySnapshotInput) PilotScope {
	adoption := normalizeAdoption(p.Adoption)
	dates := sortedCopyStrings(p.TradeDates)
	scope := PilotScope{
		AccountWhitelistCount:  len(p.AccountIDs),
		StrategyWhitelistCount: len(p.StrategyNames),
		DateWhitelistCount:     len(dates),
		DatesListed:            dates,
		Adoption:               adoption,
		KillSwitch:             p.KillSwitch,
	}
	if len(p.AccountIDs) > 0 {
		scope.AccountIDHash8 = hash8(fmt.Sprintf("%d", p.AccountIDs[0]))
	}
	if len(p.StrategyNames) > 0 {
		scope.StrategyNameHash8 = hash8(p.StrategyNames[0])
	}
	scope.ScopeExpanded = scope.AccountWhitelistCount != 1 ||
		scope.StrategyWhitelistCount != 1 ||
		(scope.DateWhitelistCount != 1 && scope.DateWhitelistCount != 2)
	return scope
}

func projectPolicyPoint(p PolicySnapshotInput) PolicyPoint {
	return PolicyPoint{
		Adoption:               normalizeAdoption(p.Adoption),
		KillSwitch:             p.KillSwitch,
		AccountWhitelistCount:  len(p.AccountIDs),
		StrategyWhitelistCount: len(p.StrategyNames),
		DateWhitelistCount:     len(p.TradeDates),
	}
}

func buildRollback(tradeDate string, open, close PolicySnapshotInput, openOK, closeOK bool) RollbackBlock {
	rb := RollbackBlock{
		FrozenPlansUntouched:   true,
		RecommendedHumanAction: actionNone,
	}
	if openOK {
		rb.Open = projectPolicyPoint(open)
	}
	if closeOK {
		rb.Close = projectPolicyPoint(close)
		rb.KillSwitchNow = close.KillSwitch
		rb.AdoptionNow = normalizeAdoption(close.Adoption)
	}
	rb.StillInPilotWindow = stillInPilotWindow(tradeDate, close.TradeDatesOr(open))
	if openOK && closeOK {
		rb.ScopeDelta = scopeDelta(open, close)
		rb.Transition, rb.RecommendedHumanAction = deriveTransition(open, close, rb.ScopeDelta)
	} else {
		rb.Transition = transitionUnknown
	}
	return rb
}

func (p PolicySnapshotInput) TradeDatesOr(fallback PolicySnapshotInput) []string {
	if len(p.TradeDates) > 0 {
		return p.TradeDates
	}
	return fallback.TradeDates
}

func stillInPilotWindow(tradeDate string, dates []string) bool {
	td := strings.TrimSpace(tradeDate)
	if td == "" {
		return false
	}
	for _, d := range dates {
		if strings.TrimSpace(d) == td {
			return true
		}
	}
	return false
}

func scopeDelta(open, close PolicySnapshotInput) ScopeDeltaBlock {
	d := ScopeDeltaBlock{
		AccountCountDelta:  len(close.AccountIDs) - len(open.AccountIDs),
		StrategyCountDelta: len(close.StrategyNames) - len(open.StrategyNames),
		DateCountDelta:     len(close.TradeDates) - len(open.TradeDates),
	}
	d.Expanded = d.AccountCountDelta > 0 || d.StrategyCountDelta > 0 || d.DateCountDelta > 0
	return d
}

func deriveTransition(open, close PolicySnapshotInput, delta ScopeDeltaBlock) (transition, action string) {
	openAd := normalizeAdoption(open.Adoption)
	closeAd := normalizeAdoption(close.Adoption)

	if delta.Expanded {
		return transitionScopeChanged, actionInvestigateScope
	}
	if close.KillSwitch {
		return transitionKilled, actionConsiderReset
	}
	if openAd == "off" && closeAd == "controlled" && !close.KillSwitch {
		return transitionEnabledControlled, actionNone
	}
	if openAd == "controlled" && closeAd == "off" && !close.KillSwitch {
		return transitionResetToOff, actionConsiderKeepOff
	}
	if openAd == closeAd && !delta.Expanded {
		return transitionUnchanged, actionNone
	}
	return transitionUnknown, actionNone
}

func normalizeAdoption(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "shadow":
		return "shadow"
	case "controlled":
		return "controlled"
	default:
		return "off"
	}
}

func sortedCopyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func sampleSymbols(all []string, limit int) []string {
	if len(all) == 0 {
		return nil
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	out := make([]string, len(all))
	copy(out, all)
	return out
}

func topReasonCounts(counts map[string]int, limit int) []ReasonCount {
	if len(counts) == 0 {
		return nil
	}
	out := make([]ReasonCount, 0, len(counts))
	for code, n := range counts {
		out = append(out, ReasonCount{Code: code, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Code < out[j].Code
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

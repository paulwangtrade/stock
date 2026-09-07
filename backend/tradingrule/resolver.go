package tradingrule

import (
	"fmt"
	"strings"

	"go-stock/backend/instrument"
)

// Resolver maps InstrumentIdentity (+ as-of) to TradingRuleProfile.
type Resolver interface {
	Resolve(id instrument.InstrumentIdentity, asOf string) (TradingRuleProfile, error)
}

// StaticResolver serves the builtin RuleVersion pack (c6h-static-v1).
type StaticResolver struct {
	versions  RuleVersionStore
	templates map[string]ruleTemplate
}

// NewStaticResolver returns the production default static rule pack.
func NewStaticResolver() *StaticResolver {
	return &StaticResolver{
		versions:  newStaticVersionStore(),
		templates: staticTemplates(),
	}
}

// NewStaticResolverWithStore allows tests / future repos to inject RuleVersionStore.
func NewStaticResolverWithStore(store RuleVersionStore) *StaticResolver {
	if store == nil {
		store = newStaticVersionStore()
	}
	return &StaticResolver{
		versions:  store,
		templates: staticTemplates(),
	}
}

// Resolve implements Resolver.
func (r *StaticResolver) Resolve(id instrument.InstrumentIdentity, asOf string) (TradingRuleProfile, error) {
	if r == nil {
		return TradingRuleProfile{}, fmt.Errorf("tradingrule: nil resolver")
	}
	market := strings.TrimSpace(id.Market)
	if market == "" {
		return TradingRuleProfile{}, fmt.Errorf("tradingrule: empty market on identity")
	}

	ruleKey, effectiveType := resolveScopeKey(id)
	tplKey := templateKeyForEffective(market, effectiveType)
	tpl, ok := r.templates[tplKey]
	if !ok {
		tpl = r.templates["CN:EQUITY"]
		ruleKey = "CN:UNKNOWN→EQUITY"
		market = instrument.MarketCN
	}

	ver, err := r.versions.Lookup(ruleKey, strings.TrimSpace(asOf))
	if err != nil {
		return TradingRuleProfile{}, err
	}

	return TradingRuleProfile{
		Market:       market,
		SecurityType: id.SecurityType, // preserve classified type (may be UNKNOWN)
		RuleKey:      ruleKey,
		Availability: AvailabilityRules{SellablePolicy: tpl.sellable},
		PriceLimit: PriceLimitRules{
			Mode:     tpl.limitMode,
			LimitPct: tpl.limitPct,
		},
		Session: SessionRules{
			Template:   SessionNormal,
			CalendarID: tpl.calendarID,
		},
		Settlement: SettlementRules{
			CashSettlePolicy: tpl.cashSettle,
		},
		VersionID:     ver.VersionID,
		EffectiveFrom: ver.EffectiveFrom,
		EffectiveTo:   ver.EffectiveTo,
		Source:        ver.Source,
		UpdatedAt:     ver.UpdatedAt,
	}, nil
}

// ResolveCode is a convenience: Classify then Resolve (still no Broker).
func ResolveCode(code, asOf string) (instrument.InstrumentIdentity, TradingRuleProfile, error) {
	id, err := instrument.Classify(code)
	if err != nil {
		return instrument.InstrumentIdentity{}, TradingRuleProfile{}, err
	}
	profile, err := NewStaticResolver().Resolve(id, asOf)
	return id, profile, err
}

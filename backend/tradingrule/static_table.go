package tradingrule

import "go-stock/backend/instrument"

// ruleTemplate is the static payload for one (market, effective security type) key.
type ruleTemplate struct {
	sellable   SellablePolicy
	limitMode  PriceLimitMode
	limitPct   PriceLimitPct
	calendarID string
	cashSettle CashSettlePolicy
}

func staticTemplates() map[string]ruleTemplate {
	return map[string]ruleTemplate{
		"CN:EQUITY": {
			sellable: SellableT1, limitMode: PriceLimitPctBand, limitPct: PriceLimitPct10,
			calendarID: "CN_A", cashSettle: CashSettleT1,
		},
		"CN:ETF": {
			sellable: SellableT0, limitMode: PriceLimitNone, limitPct: PriceLimitPctNone,
			calendarID: "CN_A", cashSettle: CashSettleT0,
		},
		"CN:CONVERTIBLE_BOND": {
			sellable: SellableT0, limitMode: PriceLimitNone, limitPct: PriceLimitPctNone,
			calendarID: "CN_A", cashSettle: CashSettleT0,
		},
		"HK:EQUITY": {
			sellable: SellableT0, limitMode: PriceLimitNone, limitPct: PriceLimitPctNone,
			calendarID: "HK", cashSettle: CashSettleT2,
		},
		"US:EQUITY": {
			sellable: SellableT0, limitMode: PriceLimitNone, limitPct: PriceLimitPctNone,
			calendarID: "US", cashSettle: CashSettleT0,
		},
	}
}

// resolveScopeKey maps identity to a template key.
// CN UNKNOWN → CN:EQUITY (fail-closed T1). HK/US UNKNOWN → market EQUITY.
func resolveScopeKey(id instrument.InstrumentIdentity) (ruleKey string, effectiveType instrument.SecurityType) {
	market := id.Market
	sec := id.SecurityType
	switch market {
	case instrument.MarketCN:
		switch sec {
		case instrument.SecurityETF:
			return "CN:ETF", sec
		case instrument.SecurityConvertibleBond:
			return "CN:CONVERTIBLE_BOND", sec
		case instrument.SecurityEquity:
			return "CN:EQUITY", sec
		case instrument.SecurityUnknown:
			return "CN:UNKNOWN→EQUITY", instrument.SecurityEquity
		default:
			return "CN:UNKNOWN→EQUITY", instrument.SecurityEquity
		}
	case instrument.MarketHK:
		if sec == instrument.SecurityUnknown {
			return "HK:UNKNOWN→EQUITY", instrument.SecurityEquity
		}
		return "HK:EQUITY", instrument.SecurityEquity
	case instrument.MarketUS:
		if sec == instrument.SecurityUnknown {
			return "US:UNKNOWN→EQUITY", instrument.SecurityEquity
		}
		return "US:EQUITY", instrument.SecurityEquity
	default:
		// Safest bootstrap: treat as CN equity T1 template.
		return "CN:UNKNOWN→EQUITY", instrument.SecurityEquity
	}
}

func templateKeyForEffective(market string, effective instrument.SecurityType) string {
	switch market {
	case instrument.MarketHK:
		return "HK:EQUITY"
	case instrument.MarketUS:
		return "US:EQUITY"
	default:
		switch effective {
		case instrument.SecurityETF:
			return "CN:ETF"
		case instrument.SecurityConvertibleBond:
			return "CN:CONVERTIBLE_BOND"
		default:
			return "CN:EQUITY"
		}
	}
}

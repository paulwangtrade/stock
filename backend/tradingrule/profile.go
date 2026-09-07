// Package tradingrule resolves InstrumentIdentity into TradingRuleProfile.
//
// Phase10-C.6-M (R2): static RuleVersion pack only.
// Does not wire Broker/Fill/Position or write DB.
//
// Package path is tradingrule (singular), per C.6-K freeze — not tradingrules.
package tradingrule

import "go-stock/backend/instrument"

// SellablePolicy is share availability (when bought shares may be sold).
type SellablePolicy string

const (
	SellableT0 SellablePolicy = "T0"
	SellableT1 SellablePolicy = "T1"
)

// PriceLimitMode is the trading-band policy (not today's absolute limit price).
type PriceLimitMode string

const (
	PriceLimitPctBand PriceLimitMode = "pct_band"
	PriceLimitNone    PriceLimitMode = "none"
)

// PriceLimitPct enumerates supported percentage bands.
type PriceLimitPct int

const (
	PriceLimitPctNone PriceLimitPct = 0
	PriceLimitPct10   PriceLimitPct = 10
	PriceLimitPct20   PriceLimitPct = 20
	PriceLimitPct30   PriceLimitPct = 30
)

// SessionTemplate is instrument-level session state (not Gateway A/B/C clock).
type SessionTemplate string

const (
	SessionNormal      SessionTemplate = "NORMAL"
	SessionDelayedOpen SessionTemplate = "DELAYED_OPEN" // reserved
	SessionSuspended   SessionTemplate = "SUSPENDED"    // reserved
)

// CashSettlePolicy is cash/settlement lag (orthogonal to share sellable).
type CashSettlePolicy string

const (
	CashSettleT0 CashSettlePolicy = "T0"
	CashSettleT1 CashSettlePolicy = "T1"
	CashSettleT2 CashSettlePolicy = "T2"
)

// AvailabilityRules describes when shares become sellable.
type AvailabilityRules struct {
	SellablePolicy SellablePolicy `json:"sellablePolicy"`
}

// PriceLimitRules describes pct trading band policy.
type PriceLimitRules struct {
	Mode     PriceLimitMode `json:"mode"`
	LimitPct PriceLimitPct  `json:"limitPct"` // 10|20|30; ignored when Mode=none
}

// SessionRules is the instrument session template + calendar id.
type SessionRules struct {
	Template   SessionTemplate `json:"template"`
	CalendarID string          `json:"calendarId"`
}

// SettlementRules is cash settlement lag (placeholder for paper MVP).
type SettlementRules struct {
	CashSettlePolicy CashSettlePolicy `json:"cashSettlePolicy"`
}

// TradingRuleProfile is the Resolver output consumed by future Availability / fillBuy.
type TradingRuleProfile struct {
	Market       string                  `json:"market"`
	SecurityType instrument.SecurityType `json:"securityType"`
	RuleKey      string                  `json:"ruleKey"`

	Availability AvailabilityRules `json:"availability"`
	PriceLimit   PriceLimitRules   `json:"priceLimit"`
	Session      SessionRules      `json:"session"`
	Settlement   SettlementRules   `json:"settlement"`

	// Version metadata (C.6-H); required even for the single static pack.
	VersionID     string `json:"versionId"`
	EffectiveFrom string `json:"effectiveFrom"`
	EffectiveTo   string `json:"effectiveTo,omitempty"` // empty = still open
	Source        string `json:"source"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
}

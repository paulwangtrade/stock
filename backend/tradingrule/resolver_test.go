package tradingrule_test

import (
	"testing"

	"go-stock/backend/instrument"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestStaticResolver_RequiredFixtures(t *testing.T) {
	r := tradingrule.NewStaticResolver()
	cases := []struct {
		code     string
		sellable tradingrule.SellablePolicy
		limitMode tradingrule.PriceLimitMode
		limitPct  tradingrule.PriceLimitPct
		secType   instrument.SecurityType
		ruleKey   string
	}{
		{"600519", tradingrule.SellableT1, tradingrule.PriceLimitPctBand, tradingrule.PriceLimitPct10, instrument.SecurityEquity, "CN:EQUITY"},
		{"510300", tradingrule.SellableT0, tradingrule.PriceLimitNone, tradingrule.PriceLimitPctNone, instrument.SecurityETF, "CN:ETF"},
		{"113052", tradingrule.SellableT0, tradingrule.PriceLimitNone, tradingrule.PriceLimitPctNone, instrument.SecurityConvertibleBond, "CN:CONVERTIBLE_BOND"},
		{"0700.HK", tradingrule.SellableT0, tradingrule.PriceLimitNone, tradingrule.PriceLimitPctNone, instrument.SecurityEquity, "HK:EQUITY"},
		{"AAPL", tradingrule.SellableT0, tradingrule.PriceLimitNone, tradingrule.PriceLimitPctNone, instrument.SecurityEquity, "US:EQUITY"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			id, err := instrument.Classify(tc.code)
			require.NoError(t, err)
			require.Equal(t, tc.secType, id.SecurityType)

			p, err := r.Resolve(id, "2026-08-14")
			require.NoError(t, err)
			require.Equal(t, tc.sellable, p.Availability.SellablePolicy)
			require.Equal(t, tc.limitMode, p.PriceLimit.Mode)
			require.Equal(t, tc.limitPct, p.PriceLimit.LimitPct)
			require.Equal(t, tradingrule.SessionNormal, p.Session.Template)
			require.Equal(t, tc.ruleKey, p.RuleKey)
			require.Equal(t, tradingrule.StaticVersionID, p.VersionID)
			require.Equal(t, tradingrule.StaticSource, p.Source)
			require.Equal(t, tradingrule.StaticEffectiveFrom, p.EffectiveFrom)
			require.Empty(t, p.EffectiveTo)
			require.NotEmpty(t, p.Session.CalendarID)
		})
	}
}

func TestStaticResolver_UnknownMapsToCNEquityT1(t *testing.T) {
	r := tradingrule.NewStaticResolver()
	id := instrument.InstrumentIdentity{
		Symbol:       "sz204001",
		Market:       instrument.MarketCN,
		Exchange:     "SZSE",
		SecurityType: instrument.SecurityUnknown,
	}
	p, err := r.Resolve(id, "2026-01-01")
	require.NoError(t, err)
	require.Equal(t, tradingrule.SellableT1, p.Availability.SellablePolicy)
	require.Equal(t, tradingrule.PriceLimitPctBand, p.PriceLimit.Mode)
	require.Equal(t, tradingrule.PriceLimitPct10, p.PriceLimit.LimitPct)
	require.Equal(t, "CN:UNKNOWN→EQUITY", p.RuleKey)
	require.Equal(t, instrument.SecurityUnknown, p.SecurityType) // identity preserved
}

func TestStaticResolver_PriceLimitConstantsSupported(t *testing.T) {
	// Type surface must include 10/20/30/NONE even if static table mostly uses 10 + NONE.
	require.Equal(t, tradingrule.PriceLimitPct(10), tradingrule.PriceLimitPct10)
	require.Equal(t, tradingrule.PriceLimitPct(20), tradingrule.PriceLimitPct20)
	require.Equal(t, tradingrule.PriceLimitPct(30), tradingrule.PriceLimitPct30)
	require.Equal(t, tradingrule.PriceLimitPct(0), tradingrule.PriceLimitPctNone)
}

func TestStaticResolver_RuleVersionStoreInjectable(t *testing.T) {
	store := &stubVersionStore{v: tradingrule.RuleVersion{
		VersionID:     "test-v0",
		Status:        tradingrule.VersionStatusActive,
		EffectiveFrom: "2020-01-01",
		Source:        "test",
		UpdatedAt:     "2020-01-02",
	}}
	r := tradingrule.NewStaticResolverWithStore(store)
	id, err := instrument.Classify("600519")
	require.NoError(t, err)
	p, err := r.Resolve(id, "2021-06-01")
	require.NoError(t, err)
	require.Equal(t, "test-v0", p.VersionID)
	require.Equal(t, "test", p.Source)
	require.Equal(t, "CN:EQUITY", store.lastScope)
}

func TestResolveCode(t *testing.T) {
	id, p, err := tradingrule.ResolveCode("510300", "2026-08-14")
	require.NoError(t, err)
	require.Equal(t, instrument.SecurityETF, id.SecurityType)
	require.Equal(t, tradingrule.SellableT0, p.Availability.SellablePolicy)
}

func TestResolver_EmptyMarketError(t *testing.T) {
	_, err := tradingrule.NewStaticResolver().Resolve(instrument.InstrumentIdentity{}, "2026-08-14")
	require.Error(t, err)
}

type stubVersionStore struct {
	v         tradingrule.RuleVersion
	lastScope string
}

func (s *stubVersionStore) Lookup(scopeKey, asOf string) (tradingrule.RuleVersion, error) {
	s.lastScope = scopeKey
	_ = asOf
	out := s.v
	out.ScopeKey = scopeKey
	return out, nil
}

// Compile-time checks: interfaces reserved for future RuleVersion repositories.
var (
	_ tradingrule.Resolver        = (*tradingrule.StaticResolver)(nil)
	_ tradingrule.RuleVersionStore = (*stubVersionStore)(nil)
)

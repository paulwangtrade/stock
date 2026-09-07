package tradingrule_test

import (
	"testing"

	"go-stock/backend/instrument"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func metaMAIN() instrument.QuantityMeta {
	return instrument.QuantityMeta{
		SecurityType:  instrument.SecurityEquity,
		MarketSegment: instrument.BoardMAIN,
		QtyUnit:       instrument.QtyUnitShare,
		LotSize:       100,
	}
}

func metaSTAR() instrument.QuantityMeta {
	return instrument.QuantityMeta{
		SecurityType:  instrument.SecurityEquity,
		MarketSegment: instrument.BoardSTAR,
		QtyUnit:       instrument.QtyUnitShare,
		LotSize:       200,
	}
}

func metaETF() instrument.QuantityMeta {
	return instrument.QuantityMeta{
		SecurityType:  instrument.SecurityETF,
		MarketSegment: instrument.BoardUNKNOWN,
		QtyUnit:       instrument.QtyUnitETFShare,
		LotSize:       100,
	}
}

func metaCB() instrument.QuantityMeta {
	return instrument.QuantityMeta{
		SecurityType:  instrument.SecurityConvertibleBond,
		MarketSegment: instrument.BoardUNKNOWN,
		QtyUnit:       instrument.QtyUnitBondUnit,
		LotSize:       10,
	}
}

func TestPolicyFromInstrumentMeta_RuleKeys(t *testing.T) {
	require.Equal(t, "CN:EQUITY:MAIN", tradingrule.PolicyFromInstrumentMeta(metaMAIN()).RuleKey())
	require.Equal(t, "CN:EQUITY:STAR", tradingrule.PolicyFromInstrumentMeta(metaSTAR()).RuleKey())
	require.Equal(t, "CN:ETF:DEFAULT", tradingrule.PolicyFromInstrumentMeta(metaETF()).RuleKey())
	require.Equal(t, "CN:CONVERTIBLE_BOND", tradingrule.PolicyFromInstrumentMeta(metaCB()).RuleKey())
}

func TestNormalizeBuyQuantity_MAIN(t *testing.T) {
	cases := []struct {
		raw  int64
		want int64
		why  string
	}{
		{99, 0, tradingrule.BuyQtyReasonRejectBelowMin},
		{100, 100, tradingrule.BuyQtyReasonOK},
		{101, 100, tradingrule.BuyQtyReasonAdjusted},
	}
	for _, tc := range cases {
		got := tradingrule.NormalizeBuyQuantity(metaMAIN(), tc.raw)
		require.Equal(t, tc.want, got.NormalizedQty, "raw=%d", tc.raw)
		require.Equal(t, tc.why, got.Reason, "raw=%d", tc.raw)
		require.Equal(t, tc.raw, got.RawQty)
		require.Equal(t, "CN:EQUITY:MAIN", got.RuleKey)
	}
}

func TestNormalizeBuyQuantity_STAR(t *testing.T) {
	cases := []struct {
		raw  int64
		want int64
		why  string
	}{
		{199, 0, tradingrule.BuyQtyReasonRejectBelowMin},
		{200, 200, tradingrule.BuyQtyReasonOK},
		{201, 201, tradingrule.BuyQtyReasonOK},
	}
	for _, tc := range cases {
		got := tradingrule.NormalizeBuyQuantity(metaSTAR(), tc.raw)
		require.Equal(t, tc.want, got.NormalizedQty, "raw=%d", tc.raw)
		require.Equal(t, tc.why, got.Reason, "raw=%d", tc.raw)
		require.Equal(t, "CN:EQUITY:STAR", got.RuleKey)
	}
}

func TestNormalizeBuyQuantity_ETF(t *testing.T) {
	cases := []struct {
		raw  int64
		want int64
		why  string
	}{
		{99, 0, tradingrule.BuyQtyReasonRejectBelowMin},
		{100, 100, tradingrule.BuyQtyReasonOK},
	}
	for _, tc := range cases {
		got := tradingrule.NormalizeBuyQuantity(metaETF(), tc.raw)
		require.Equal(t, tc.want, got.NormalizedQty, "raw=%d", tc.raw)
		require.Equal(t, tc.why, got.Reason, "raw=%d", tc.raw)
		require.Equal(t, "CN:ETF:DEFAULT", got.RuleKey)
	}
}

func TestNormalizeBuyQuantity_CB(t *testing.T) {
	cases := []struct {
		raw  int64
		want int64
		why  string
	}{
		{9, 0, tradingrule.BuyQtyReasonRejectBelowMin},
		{10, 10, tradingrule.BuyQtyReasonOK},
		{11, 10, tradingrule.BuyQtyReasonAdjusted},
	}
	for _, tc := range cases {
		got := tradingrule.NormalizeBuyQuantity(metaCB(), tc.raw)
		require.Equal(t, tc.want, got.NormalizedQty, "raw=%d", tc.raw)
		require.Equal(t, tc.why, got.Reason, "raw=%d", tc.raw)
		require.Equal(t, "CN:CONVERTIBLE_BOND", got.RuleKey)
	}
}

func TestQuantityPolicy_InterfaceDirect(t *testing.T) {
	var p tradingrule.QuantityPolicy = tradingrule.TemplateSTAR()
	got := p.NormalizeBuy(201)
	require.Equal(t, int64(201), got.NormalizedQty)
	require.Equal(t, tradingrule.BuyQtyReasonOK, got.Reason)
}

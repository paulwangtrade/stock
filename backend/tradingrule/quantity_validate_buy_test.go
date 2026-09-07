package tradingrule_test

import (
	"testing"

	"go-stock/backend/instrument"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestValidateBuyQuantity_BoardMatrix(t *testing.T) {
	cases := []struct {
		name   string
		meta   instrument.QuantityMeta
		qty    int64
		accept bool
	}{
		{"MAIN_99", instrument.TemplateQuantityMeta(instrument.SecurityEquity, instrument.BoardMAIN), 99, false},
		{"MAIN_100", instrument.TemplateQuantityMeta(instrument.SecurityEquity, instrument.BoardMAIN), 100, true},
		{"MAIN_101", instrument.TemplateQuantityMeta(instrument.SecurityEquity, instrument.BoardMAIN), 101, false},
		{"STAR_199", instrument.TemplateQuantityMeta(instrument.SecurityEquity, instrument.BoardSTAR), 199, false},
		{"STAR_200", instrument.TemplateQuantityMeta(instrument.SecurityEquity, instrument.BoardSTAR), 200, true},
		{"STAR_201", instrument.TemplateQuantityMeta(instrument.SecurityEquity, instrument.BoardSTAR), 201, true},
		{"ETF_99", instrument.TemplateQuantityMeta(instrument.SecurityETF, instrument.BoardUNKNOWN), 99, false},
		{"ETF_100", instrument.TemplateQuantityMeta(instrument.SecurityETF, instrument.BoardUNKNOWN), 100, true},
		{"CB_9", instrument.TemplateQuantityMeta(instrument.SecurityConvertibleBond, instrument.BoardUNKNOWN), 9, false},
		{"CB_10", instrument.TemplateQuantityMeta(instrument.SecurityConvertibleBond, instrument.BoardUNKNOWN), 10, true},
		{"CB_11_strict", instrument.TemplateQuantityMeta(instrument.SecurityConvertibleBond, instrument.BoardUNKNOWN), 11, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := tradingrule.ValidateBuyQuantity(tc.meta, tc.qty)
			require.Equal(t, tc.accept, v.Accepted)
		})
	}
}

func TestApplyBrokerBuyQuantityGate_FlagOff_Passthrough(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	v := tradingrule.ApplyBrokerBuyQuantityGate("sh688981", 199)
	require.True(t, v.Accepted)
	require.Equal(t, int64(199), v.Qty)
	require.Equal(t, "FLAG_OFF", v.Reason)
}

func TestApplyBrokerBuyQuantityGate_FlagOn_STAR_CB(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	star199 := tradingrule.ApplyBrokerBuyQuantityGate("sh688981", 199)
	require.False(t, star199.Accepted)
	require.Equal(t, tradingrule.BuyQtyReasonRejectBelowMin, star199.Reason)

	star200 := tradingrule.ApplyBrokerBuyQuantityGate("sh688981", 200)
	require.True(t, star200.Accepted)
	require.Equal(t, int64(200), star200.Qty)

	star201 := tradingrule.ApplyBrokerBuyQuantityGate("sh688981", 201)
	require.True(t, star201.Accepted)
	require.Equal(t, int64(201), star201.Qty)

	cb9 := tradingrule.ApplyBrokerBuyQuantityGate("sh113052", 9)
	require.False(t, cb9.Accepted)

	cb10 := tradingrule.ApplyBrokerBuyQuantityGate("sh113052", 10)
	require.True(t, cb10.Accepted)
	require.Equal(t, int64(10), cb10.Qty)

	// M2.3.2 Validate-only: CB 11 reject (no auto-normalize to 10)
	cb11 := tradingrule.ApplyBrokerBuyQuantityGate("sh113052", 11)
	require.False(t, cb11.Accepted)
	require.True(t, cb11.WouldNormalize)
	require.Equal(t, int64(10), cb11.SuggestedQty)
}

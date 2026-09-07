package instrument_test

import (
	"testing"

	"go-stock/backend/instrument"

	"github.com/stretchr/testify/require"
)

func TestClassify_RequiredFixtures(t *testing.T) {
	// Bare CN digits inherit NormalizeStockCode exchange inference (60→SSE, 30→SZSE;
	// 51/11 without prefix may not map to listing exchange — type is authoritative).
	cases := []struct {
		code     string
		market   string
		secType  instrument.SecurityType
		exchange string // empty = only require non-empty
	}{
		{"600519", instrument.MarketCN, instrument.SecurityEquity, "SSE"},
		{"300750", instrument.MarketCN, instrument.SecurityEquity, "SZSE"},
		{"510300", instrument.MarketCN, instrument.SecurityETF, ""},
		{"113052", instrument.MarketCN, instrument.SecurityConvertibleBond, ""},
		{"0700.HK", instrument.MarketHK, instrument.SecurityEquity, "HKEX"},
		{"AAPL", instrument.MarketUS, instrument.SecurityEquity, "US"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			id, err := instrument.Classify(tc.code)
			require.NoError(t, err)
			require.Equal(t, tc.market, id.Market)
			require.Equal(t, tc.secType, id.SecurityType)
			require.NotEmpty(t, id.Exchange)
			if tc.exchange != "" {
				require.Equal(t, tc.exchange, id.Exchange)
			}
			require.NotEmpty(t, id.Symbol)
			require.NotEmpty(t, id.TSCode)
			require.NotEmpty(t, id.Source)
		})
	}
}

func TestClassify_CNPrefixTable(t *testing.T) {
	cases := []struct {
		code    string
		secType instrument.SecurityType
	}{
		{"sh600519", instrument.SecurityEquity},
		{"sz000001", instrument.SecurityEquity},
		{"sz300750", instrument.SecurityEquity},
		{"sh688981", instrument.SecurityEquity},
		{"sh510300", instrument.SecurityETF},
		{"sz159919", instrument.SecurityETF},
		{"sh113052", instrument.SecurityConvertibleBond},
		{"sz128136", instrument.SecurityConvertibleBond},
		{"hk00700", instrument.SecurityEquity},
		{"gb_aapl", instrument.SecurityEquity},
		{"AAPL.US", instrument.SecurityEquity},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			id, err := instrument.Classify(tc.code)
			require.NoError(t, err)
			require.Equal(t, tc.secType, id.SecurityType)
		})
	}
}

func TestClassify_UnknownCNSegment(t *testing.T) {
	// 204xxx is not in equity/ETF/CB prefix tables → UNKNOWN (fail-closed for later T1).
	id, err := instrument.Classify("204001")
	require.NoError(t, err)
	require.Equal(t, instrument.MarketCN, id.Market)
	require.Equal(t, instrument.SecurityUnknown, id.SecurityType)
}

func TestClassify_Errors(t *testing.T) {
	_, err := instrument.Classify("")
	require.Error(t, err)
	_, err = instrument.Classify("贵州茅台")
	require.Error(t, err)
}

func TestClassify_HKSymbolPadding(t *testing.T) {
	id, err := instrument.Classify("0700.HK")
	require.NoError(t, err)
	require.Equal(t, "hk00700", id.Symbol)
	require.Equal(t, "00700.HK", id.TSCode)
}

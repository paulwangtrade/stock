// Package instrument provides financial-instrument identity classification.
//
// Phase10-C.6-L (R1): code → InstrumentIdentity only.
// Does not resolve TradingRule, touch Broker/Fill/Position, or write DB.
package instrument

// Market mirrors data.NormalizeStockCode markets (CN | HK | US).
const (
	MarketCN = "CN"
	MarketHK = "HK"
	MarketUS = "US"
)

// SecurityType is the C.6-E frozen instrument class.
type SecurityType string

const (
	SecurityEquity          SecurityType = "EQUITY"
	SecurityETF             SecurityType = "ETF"
	SecurityConvertibleBond SecurityType = "CONVERTIBLE_BOND"
	SecurityUnknown         SecurityType = "UNKNOWN"
)

// Classification source / confidence (audit fields; not trading rules).
const (
	SourceCodePrefix    = "code_prefix"
	SourceDefaultMarket = "default_market"

	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// InstrumentIdentity is the Classifier output: what the instrument is, not how it trades.
type InstrumentIdentity struct {
	Input         string       `json:"input"`
	Symbol        string       `json:"symbol"` // SinaCode when available (aligned with paper_sim)
	Market        string       `json:"market"` // CN | HK | US
	Exchange      string       `json:"exchange"`
	SecurityType  SecurityType `json:"securityType"`
	MarketSegment string       `json:"marketSegment,omitempty"` // board: MAIN|CHINEXT|STAR|BSE|UNKNOWN (Phase12-M1)
	TSCode        string       `json:"tsCode,omitempty"`
	Source        string       `json:"source,omitempty"`
	Confidence    string       `json:"confidence,omitempty"`
}

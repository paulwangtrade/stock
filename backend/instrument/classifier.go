package instrument

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
)

// Classifier turns a raw code into InstrumentIdentity (no TradingRule, no DB).
type Classifier struct{}

// NewClassifier returns the default static code-prefix classifier.
func NewClassifier() *Classifier {
	return &Classifier{}
}

// Classify normalizes code then assigns security_type.
// Normalize failure → error. Recognizable market but unclear type → UNKNOWN (not error).
func Classify(code string) (InstrumentIdentity, error) {
	return NewClassifier().Classify(code)
}

// Classify implements InstrumentClassifier.
func (c *Classifier) Classify(code string) (InstrumentIdentity, error) {
	raw := strings.TrimSpace(code)
	if raw == "" {
		return InstrumentIdentity{}, fmt.Errorf("instrument: empty stock code")
	}

	n, err := data.NormalizeStockCode(raw)
	if err != nil {
		return InstrumentIdentity{}, fmt.Errorf("instrument: normalize: %w", err)
	}

	id := InstrumentIdentity{
		Input:    raw,
		Symbol:   strings.TrimSpace(n.SinaCode),
		Market:   n.Market,
		Exchange: n.Exchange,
		TSCode:   n.TSCode,
	}
	if id.Symbol == "" {
		id.Symbol = n.Symbol
	}

	switch n.Market {
	case data.MarketHK, data.MarketUS:
		id.SecurityType = SecurityEquity
		id.MarketSegment = BoardUNKNOWN
		id.Source = SourceDefaultMarket
		id.Confidence = ConfidenceMedium
		return id, nil
	case data.MarketCN:
		id.SecurityType, id.Source, id.Confidence = classifyCNSymbol(n.Symbol)
		id.MarketSegment = classifyCNBoard(n.Symbol, id.SecurityType)
		return id, nil
	default:
		id.SecurityType = SecurityUnknown
		id.MarketSegment = BoardUNKNOWN
		id.Source = SourceCodePrefix
		id.Confidence = ConfidenceLow
		return id, nil
	}
}

// classifyCNSymbol applies C.6-E code-prefix heuristics on the 6-digit CN symbol.
// Order: convertible → ETF → equity → UNKNOWN (fail-closed toward equity/T1 later).
func classifyCNSymbol(symbol string) (SecurityType, string, string) {
	s := strings.TrimSpace(symbol)
	if len(s) != 6 {
		return SecurityUnknown, SourceCodePrefix, ConfidenceLow
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return SecurityUnknown, SourceCodePrefix, ConfidenceLow
		}
	}

	prefix2 := s[:2]

	// Convertible bonds: SH 11xxxx, SZ 12xxxx
	switch prefix2 {
	case "11", "12":
		return SecurityConvertibleBond, SourceCodePrefix, ConfidenceHigh
	}

	// ETF / listed funds: SH 51/56/58, SZ 15/16/18
	switch prefix2 {
	case "51", "56", "58", "15", "16", "18":
		return SecurityETF, SourceCodePrefix, ConfidenceHigh
	}

	// Common equity boards
	switch prefix2 {
	case "60", "00", "30", "68":
		return SecurityEquity, SourceCodePrefix, ConfidenceHigh
	}

	// BSE-style leading digit 4/8 (and 9 reserved in normalize)
	switch s[0] {
	case '4', '8':
		return SecurityEquity, SourceCodePrefix, ConfidenceMedium
	}

	return SecurityUnknown, SourceCodePrefix, ConfidenceLow
}

// classifyCNBoard maps 6-digit CN symbol → market_segment (board). Additive; does not change Sellable.
func classifyCNBoard(symbol string, secType SecurityType) string {
	s := strings.TrimSpace(symbol)
	if len(s) != 6 {
		return BoardUNKNOWN
	}
	// Non-equity: board still inferred from listing prefix when possible; else UNKNOWN.
	prefix2 := s[:2]
	switch prefix2 {
	case "68":
		return BoardSTAR
	case "30":
		return BoardCHINEXT
	case "60", "00":
		return BoardMAIN
	}
	if s[0] == '4' || s[0] == '8' {
		return BoardBSE
	}
	switch secType {
	case SecurityETF, SecurityConvertibleBond:
		// Listed funds / CB: segment not equity-board; keep UNKNOWN for M1 templates.
		return BoardUNKNOWN
	default:
		return BoardUNKNOWN
	}
}

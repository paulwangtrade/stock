package instrument

import (
	"fmt"
	"strings"
)

// QuantityMeta is the resolved instrument quantity foundation (read-only for M1).
type QuantityMeta struct {
	Symbol         string       `json:"symbol"`
	SecurityType   SecurityType `json:"security_type"`
	MarketSegment  string       `json:"market_segment"` // board
	QtyUnit        string       `json:"qty_unit"`
	LotSize        int64        `json:"lot_size"`
	Source         string       `json:"source"`
	TemplateKey    string       `json:"template_key,omitempty"`
	IdentitySource string       `json:"identity_source,omitempty"`
}

// InstrumentMetadataDTO is the API/DTO projection for M1 (snake_case JSON).
type InstrumentMetadataDTO struct {
	Symbol        string `json:"symbol"`
	SecurityType  string `json:"security_type"`
	MarketSegment string `json:"market_segment"`
	QtyUnit       string `json:"qty_unit"`
	LotSize       int64  `json:"lot_size"`
	Source        string `json:"source"`
	TemplateKey   string `json:"template_key,omitempty"`
}

// ToDTO maps QuantityMeta → DTO.
func (m QuantityMeta) ToDTO() InstrumentMetadataDTO {
	return InstrumentMetadataDTO{
		Symbol:        m.Symbol,
		SecurityType:  string(m.SecurityType),
		MarketSegment: m.MarketSegment,
		QtyUnit:       m.QtyUnit,
		LotSize:       m.LotSize,
		Source:        m.Source,
		TemplateKey:   m.TemplateKey,
	}
}

// TemplateKeyFor builds a static template id.
func TemplateKeyFor(secType SecurityType, board string) string {
	b := strings.TrimSpace(board)
	if b == "" {
		b = BoardUNKNOWN
	}
	switch secType {
	case SecurityETF:
		return "CN:ETF:DEFAULT"
	case SecurityConvertibleBond:
		return "CN:CONVERTIBLE_BOND"
	case SecurityEquity:
		return fmt.Sprintf("CN:EQUITY:%s", b)
	default:
		return "CN:UNKNOWN"
	}
}

// TemplateQuantityMeta returns static defaults (no DB). Does not touch trading.
// Product defaults (M1):
//
//	MAIN equity → SHARE / 100
//	STAR equity → SHARE / 200
//	CB → BOND_UNIT / 10
//	ETF → ETF_SHARE / 100
func TemplateQuantityMeta(secType SecurityType, board string) QuantityMeta {
	board = strings.TrimSpace(board)
	if board == "" {
		board = BoardUNKNOWN
	}
	key := TemplateKeyFor(secType, board)
	meta := QuantityMeta{
		SecurityType:  secType,
		MarketSegment: board,
		Source:        MetaSourceTemplate,
		TemplateKey:   key,
	}
	switch secType {
	case SecurityConvertibleBond:
		meta.QtyUnit = QtyUnitBondUnit
		meta.LotSize = 10
		meta.MarketSegment = board
		return meta
	case SecurityETF:
		meta.QtyUnit = QtyUnitETFShare
		meta.LotSize = 100
		return meta
	case SecurityEquity:
		meta.QtyUnit = QtyUnitShare
		switch board {
		case BoardSTAR:
			meta.LotSize = 200
		default:
			// MAIN / CHINEXT / BSE / UNKNOWN equity → 100 (fail-closed toward现网一手)
			meta.LotSize = 100
		}
		return meta
	default:
		meta.SecurityType = SecurityUnknown
		meta.QtyUnit = QtyUnitShare
		meta.LotSize = 100
		meta.Source = MetaSourceUnknown
		meta.TemplateKey = "CN:UNKNOWN"
		meta.MarketSegment = BoardUNKNOWN
		return meta
	}
}

// ResolveQuantityMetaFromIdentity applies template (+ optional override row).
func ResolveQuantityMetaFromIdentity(id InstrumentIdentity, override *InstrumentQuantityMeta) QuantityMeta {
	board := strings.TrimSpace(id.MarketSegment)
	if board == "" {
		board = BoardUNKNOWN
	}
	meta := TemplateQuantityMeta(id.SecurityType, board)
	meta.Symbol = strings.TrimSpace(id.Symbol)
	if meta.Symbol == "" {
		meta.Symbol = strings.TrimSpace(id.Input)
	}
	meta.IdentitySource = id.Source

	if override == nil {
		return meta
	}
	// DB override wins for qty fields; keep classified type/board unless override supplies them.
	if u := strings.TrimSpace(override.QtyUnit); u != "" {
		meta.QtyUnit = u
	}
	if override.LotSize > 0 {
		meta.LotSize = override.LotSize
	}
	if s := strings.TrimSpace(override.MarketSegment); s != "" {
		meta.MarketSegment = s
	}
	if s := strings.TrimSpace(override.SecurityType); s != "" {
		meta.SecurityType = SecurityType(s)
	}
	meta.Source = MetaSourceDBOverride
	meta.TemplateKey = TemplateKeyFor(meta.SecurityType, meta.MarketSegment)
	return meta
}

package instrument

import (
	"strings"

	"gorm.io/gorm"
)

// ResolveInstrumentMetadata classifies code then resolves quantity meta (template ± DB override).
// Phase12-M1: read-only foundation — must not be wired into Broker / Materialize / Unlock.
func ResolveInstrumentMetadata(code string, database *gorm.DB) (QuantityMeta, error) {
	id, err := Classify(code)
	if err != nil {
		return QuantityMeta{}, err
	}
	var override *InstrumentQuantityMeta
	sym := strings.TrimSpace(id.Symbol)
	if sym == "" {
		sym = strings.ToLower(strings.TrimSpace(code))
	}
	if row, ok := LookupQuantityMetaOverride(database, sym); ok {
		override = row
	}
	return ResolveQuantityMetaFromIdentity(id, override), nil
}

// ResolveInstrumentMetadataDTO is the DTO convenience wrapper.
func ResolveInstrumentMetadataDTO(code string, database *gorm.DB) (InstrumentMetadataDTO, error) {
	m, err := ResolveInstrumentMetadata(code, database)
	if err != nil {
		return InstrumentMetadataDTO{}, err
	}
	return m.ToDTO(), nil
}

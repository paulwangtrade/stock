package instrument

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// InstrumentQuantityMeta is an additive per-code override table (Phase12-M1).
// Trading paths must NOT read this until M2 Feature Flag — foundation / observation only.
type InstrumentQuantityMeta struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	StockCode     string    `gorm:"size:32;uniqueIndex:uidx_instrument_quantity_meta_stock_code;not null" json:"stock_code"` // prefer sina code
	SecurityType  string    `gorm:"size:32" json:"security_type"`
	MarketSegment string    `gorm:"size:16;column:market_segment" json:"market_segment"` // board
	QtyUnit       string    `gorm:"size:16" json:"qty_unit"`
	LotSize       int64     `json:"lot_size"`
	Notes         string    `gorm:"size:256" json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (InstrumentQuantityMeta) TableName() string { return "instrument_quantity_meta" }

// EnsureQuantityMetaTable creates the override table (idempotent AutoMigrate).
func EnsureQuantityMetaTable(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("instrument: nil db")
	}
	return database.AutoMigrate(&InstrumentQuantityMeta{})
}

// LookupQuantityMetaOverride returns a DB row for normalized stock code, if any.
func LookupQuantityMetaOverride(database *gorm.DB, stockCode string) (*InstrumentQuantityMeta, bool) {
	code := strings.ToLower(strings.TrimSpace(stockCode))
	if database == nil || code == "" || !database.Migrator().HasTable(&InstrumentQuantityMeta{}) {
		return nil, false
	}
	var row InstrumentQuantityMeta
	err := database.Where("stock_code = ?", code).Limit(1).Find(&row).Error
	if err != nil || row.ID == 0 {
		return nil, false
	}
	return &row, true
}

// UpsertQuantityMetaOverride writes/updates a per-code override (admin/test helper; not used by trading).
func UpsertQuantityMetaOverride(database *gorm.DB, row *InstrumentQuantityMeta) error {
	if database == nil || row == nil {
		return fmt.Errorf("instrument: nil db or row")
	}
	row.StockCode = strings.ToLower(strings.TrimSpace(row.StockCode))
	if row.StockCode == "" {
		return fmt.Errorf("instrument: empty stock_code")
	}
	var existing InstrumentQuantityMeta
	err := database.Where("stock_code = ?", row.StockCode).Limit(1).Find(&existing).Error
	if err != nil {
		return err
	}
	if existing.ID == 0 {
		return database.Create(row).Error
	}
	row.ID = existing.ID
	row.CreatedAt = existing.CreatedAt
	return database.Save(row).Error
}

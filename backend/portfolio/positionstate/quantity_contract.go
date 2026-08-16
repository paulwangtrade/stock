package positionstate

// Phase12-M0 quantity contract (load path only; does not change Calculate).
//
// Canonical fill qty is stored in paper_sim_fills.volume. There is no fills.quantity column.
// LotRecord.Quantity is the in-memory canonical qty alias after this SELECT.

const (
	// FillQtyPhysicalColumn is the sole DB column for fill trade quantity.
	FillQtyPhysicalColumn = "volume"

	// fillLotQtySelectSQL maps f.volume → row.Quantity (canonical qty).
	// Do NOT substitute f.quantity — SQLite/GORM will yield empty lots.
	fillLotQtySelectSQL = "o.stock_code AS stock_code, o.trade_date AS trade_date, o.side AS side, f.volume AS quantity"

	// wrongFillLotQtySelectSQL is the pre-M0 bug; kept for regression tests only.
	wrongFillLotQtySelectSQL = "o.stock_code AS stock_code, o.trade_date AS trade_date, o.side AS side, f.quantity AS quantity"
)

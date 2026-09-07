package instrument

// Market segment (board) — Phase12-M1. Orthogonal to SellablePolicy / OrderQuantityPolicy.
const (
	BoardMAIN    = "MAIN"    // 沪深主板等
	BoardCHINEXT = "CHINEXT" // 创业板
	BoardSTAR    = "STAR"    // 科创板
	BoardBSE     = "BSE"     // 北交所风格
	BoardUNKNOWN = "UNKNOWN"
)

// QtyUnit is the canonical unit for instrument quantity metadata (Phase12-M0/M1).
const (
	QtyUnitShare    = "SHARE"
	QtyUnitETFShare = "ETF_SHARE"
	QtyUnitBondUnit = "BOND_UNIT"
)

// MetaSource audit: where lot_size / qty_unit came from.
const (
	MetaSourceTemplate   = "template"
	MetaSourceDBOverride = "db_override"
	MetaSourceUnknown    = "unknown_fail_closed"
)

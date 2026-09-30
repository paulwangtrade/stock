package signalattribution

// Research-only labels. These strings are the product contract for the UI.
const (
	StatusOK           = "ok"
	StatusInsufficient = "insufficient"
	TextInsufficient   = "数据不足"
	EmptyStatText      = "—"

	BasisLocalClose    = "local_day_close"
	BasisSnapshot      = "snapshot_recorded"
	BasisLabelLocal    = "日线收盘"
	BasisLabelSnapshot = "快照记录价"

	ReasonNoEntry     = "no_entry"
	ReasonNoFuture    = "no_future_bar"
	ReasonCalendarGap = "calendar_gap"
	ReasonBadAsOf     = "bad_asof"

	Disclaimer        = "仅研究对照，不构成交易建议，不进入模拟交易计划"
	ResearchStatLabel = "研究统计，不是收益证明"
	ScopeNote         = "只对照信号扫描快照里的命中，不连接入场、持仓或退出，也不写入交易计划。"
	CalendarNote      = "交易日顺序来自该股票本地日线，并用周末休市规则检查缺口。节假日表未接入时，跨过工作日的缺口视为日历不完整，对应列显示数据不足，不估算涨跌。"
	BarNote           = "对照价优先用对照日的本地不复权收盘；没有日线时，若快照里带有记录价，则标明「快照记录价」。后续涨跌只读本地日线，不请求行情，缺数据不补。"
)

// Horizons are trading-day offsets after the snapshot as-of day.
var Horizons = []int{1, 3, 10}

// Query selects one research snapshot. SnapshotID wins when set.
type Query struct {
	SnapshotID uint   `json:"snapshotId"`
	TradeDate  string `json:"tradeDate"`
	Session    string `json:"session"`
	StrategyID string `json:"strategyId"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// DayBar is one local daily close. Date is YYYY-MM-DD.
type DayBar struct {
	Date  string
	Close float64
}

// HitInput is one snapshot hit plus an optional recorded price.
// BarKey is the normalized ts_code used to look up local bars.
type HitInput struct {
	Code          string
	Name          string
	SnapshotPrice float64
	BarKey        string
}

// SnapshotMeta is the chosen signal-scan snapshot header.
type SnapshotMeta struct {
	ID           uint
	TradeDate    string
	Session      string
	StrategyID   string
	StrategyName string
}

// HorizonCell is one forward window. ReturnRate is nil unless Status is ok.
type HorizonCell struct {
	Horizon     int      `json:"horizon"`
	Status      string   `json:"status"`
	Text        string   `json:"text"`
	Reason      string   `json:"reason,omitempty"`
	ReturnRate  *float64 `json:"returnRate,omitempty"`
	FutureDate  string   `json:"futureDate,omitempty"`
	FutureClose *float64 `json:"futureClose,omitempty"`
}

// HitRow is one stock on the observation table.
type HitRow struct {
	Code            string        `json:"code"`
	Name            string        `json:"name"`
	StrategyID      string        `json:"strategyId"`
	StrategyName    string        `json:"strategyName"`
	AsOfDate        string        `json:"asOfDate"`
	Close           *float64      `json:"close,omitempty"`
	CloseText       string        `json:"closeText"`
	PriceBasis      string        `json:"priceBasis"`
	PriceBasisLabel string        `json:"priceBasisLabel"`
	Horizons        []HorizonCell `json:"horizons"`
}

// HorizonStat aggregates one horizon across the whole snapshot, not the current page.
type HorizonStat struct {
	Horizon    int      `json:"horizon"`
	Complete   int      `json:"complete"`
	Mean       *float64 `json:"mean,omitempty"`
	Median     *float64 `json:"median,omitempty"`
	MeanText   string   `json:"meanText"`
	MedianText string   `json:"medianText"`
}

// Summary is a research count. It is not an alpha claim.
type Summary struct {
	HitCount    int           `json:"hitCount"`
	CompleteAll int           `json:"completeAll"`
	Horizons    []HorizonStat `json:"horizons"`
	Label       string        `json:"label"`
}

// View is the attribution table returned to the UI.
type View struct {
	OK                bool     `json:"ok"`
	Message           string   `json:"message,omitempty"`
	Disclaimer        string   `json:"disclaimer"`
	ScopeNote         string   `json:"scopeNote"`
	CalendarNote      string   `json:"calendarNote"`
	BarNote           string   `json:"barNote"`
	ResearchStatLabel string   `json:"researchStatLabel"`
	SnapshotID        uint     `json:"snapshotId"`
	TradeDate         string   `json:"tradeDate"`
	Session           string   `json:"session"`
	StrategyID        string   `json:"strategyId"`
	StrategyName      string   `json:"strategyName"`
	Page              int      `json:"page"`
	PageSize          int      `json:"pageSize"`
	Total             int      `json:"total"`
	Rows              []HitRow `json:"rows"`
	Summary           Summary  `json:"summary"`
}

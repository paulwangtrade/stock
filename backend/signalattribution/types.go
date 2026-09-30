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
	ToDateNote        = "迄今：对照日收盘到本地日线里沿交易日连续走到的最后一根收盘。中间有未确认缺口时该列显示数据不足，不用缺口之后的价格。"

	LargeSampleLimit   = 100
	LargeSampleWarning = "样本过大，先收窄信号再归因"
	SmallSampleLimit   = 8

	CohortNote             = "以下差异只说明该快照里、信号发生前已经知道的特征，在 +1 上涨与下跌两组之间有什么不同。只供研究过滤参考，不是买入或卖出建议。"
	CohortSingleDayWarning = "样本来自单个对照日。组内数量少时，差异不稳定，不能当成稳定规律。"
	CohortSmallSampleNote  = "上涨或下跌组不足 8 只。"
	CohortEmptyBoth        = "没有完整的 +1 对照，无法分成上涨组和下跌组"
	CohortEmptySide        = "只有上涨或只有下跌，没有可对照的两组"
	CohortSizeNote         = "快照没有流通市值字段，未做市值分档。"
	CohortLargeBrowseNote  = "样本过大，下面的差额只供浏览，不是共性结论。"

	WhatIfNote            = "对照实验，不是买卖指令。勾选分组共性里、信号发生前已经知道的特征，对照「全部命中等权」和「只留这些命中的等权」。明细表仍是全部命中。不写入交易计划，也不下单。"
	WhatIfInSampleNote    = "+1 分组用来写出过滤规则，所以 +1 列是同一样本里的对照，不是事先能知道的结果。"
	WhatIfWeightNote      = "等权：每一列里，数据完整的股票权重相同。缺数据不计入，也不用 0 填充。"
	WhatIfSmallSampleNote = "假设子集不足 8 只，差额不稳定，不能当成规律。"
	WhatIfNoCohort        = "没有上涨与下跌两组，无法从共性写出过滤规则"
	WhatIfNoSeparation    = "上涨组和下跌组的已知特征没有可分开的差异，无法写出过滤规则"
	WhatIfNoneSelected    = "未勾选特征，不生成假设子集"
	WhatIfLargeDefault    = "样本过大，未自动套用共性过滤。勾选后只供浏览对照，不是共性结论。"
	WhatIfNoMatch         = "没有股票同时满足勾选的特征。"
	WhatIfBaselineLabel   = "基线 · 全部命中等权"
	WhatIfScenarioLabel   = "假设 · 勾选特征等权"
)

// Horizons are trading-day offsets after the snapshot as-of day.
var Horizons = []int{1, 3, 10}

// Query selects one research snapshot. SnapshotID wins when set.
// SignalTags narrow the snapshot before returns, summary, cohort, and what-if are computed.
// An empty tag list keeps every hit. SortKey is "1", "3", "10", or "toDate".
type Query struct {
	SnapshotID    uint     `json:"snapshotId"`
	TradeDate     string   `json:"tradeDate"`
	Session       string   `json:"session"`
	StrategyID    string   `json:"strategyId"`
	Page          int      `json:"page"`
	PageSize      int      `json:"pageSize"`
	SignalTags    []string `json:"signalTags"`
	ReboundMaxRsi *float64 `json:"reboundMaxRsi,omitempty"`
	SortKey       string   `json:"sortKey"`
	SortDesc      bool     `json:"sortDesc"`
	// WhatIfSet reports that the user chose feature keys. When false, highlighted
	// contrasts are the default subset. An explicit empty key list builds no subset.
	WhatIfSet  bool     `json:"whatIfSet,omitempty"`
	WhatIfKeys []string `json:"whatIfKeys,omitempty"`
}

// DayBar is one local daily close. Date is YYYY-MM-DD.
type DayBar struct {
	Date  string
	Close float64
}

// HitInput is one snapshot hit plus fields already known at the snapshot as-of.
// BarKey is the normalized ts_code used to look up local bars.
// KlineCode is the East-money code for the K-line modal.
type HitInput struct {
	Code           string
	Name           string
	SnapshotPrice  float64
	BarKey         string
	KlineCode      string
	Tag            string
	Industry       string
	Market         string
	VolumeRatio    float64
	HasVolumeRatio bool
	RSI            float64
	HasRSI         bool
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

// AsOfFeatures are known on the snapshot day. They never use a bar after as-of.
type AsOfFeatures struct {
	Industry      string   `json:"industry,omitempty"`
	Market        string   `json:"market,omitempty"`
	Tag           string   `json:"tag,omitempty"`
	VolumeRatio   *float64 `json:"volumeRatio,omitempty"`
	RSI           *float64 `json:"rsi,omitempty"`
	Prior20Return *float64 `json:"prior20Return,omitempty"`
	DistMA20      *float64 `json:"distMa20,omitempty"`
}

// HitRow is one stock on the observation table.
type HitRow struct {
	Code            string        `json:"code"`
	Name            string        `json:"name"`
	KlineCode       string        `json:"klineCode,omitempty"`
	Tag             string        `json:"tag,omitempty"`
	StrategyID      string        `json:"strategyId"`
	StrategyName    string        `json:"strategyName"`
	AsOfDate        string        `json:"asOfDate"`
	Close           *float64      `json:"close,omitempty"`
	CloseText       string        `json:"closeText"`
	PriceBasis      string        `json:"priceBasis"`
	PriceBasisLabel string        `json:"priceBasisLabel"`
	Horizons        []HorizonCell `json:"horizons"`
	ToDate          HorizonCell   `json:"toDate"`
	Features        AsOfFeatures  `json:"features"`
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
// HitCount is the filtered subset. SnapshotHitCount is the snapshot before the signal-tag filter.
type Summary struct {
	HitCount         int           `json:"hitCount"`
	SnapshotHitCount int           `json:"snapshotHitCount"`
	CompleteAll      int           `json:"completeAll"`
	Horizons         []HorizonStat `json:"horizons"`
	ToDate           HorizonStat   `json:"toDate"`
	Label            string        `json:"label"`
}

// CohortGroup is one +1 direction. Feature texts use only as-of-known fields.
type CohortGroup struct {
	Key                  string `json:"key"`
	Label                string `json:"label"`
	Count                int    `json:"count"`
	VolumeRatioText      string `json:"volumeRatioText"`
	Prior20Text          string `json:"prior20Text"`
	DistMA20Text         string `json:"distMa20Text"`
	RSIText              string `json:"rsiText"`
	TopIndustry          string `json:"topIndustry"`
	TopIndustryShareText string `json:"topIndustryShareText"`
}

// CohortContrast is one up-versus-down difference. Highlight is off when the sample is too large.
type CohortContrast struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	UpText    string `json:"upText"`
	DownText  string `json:"downText"`
	DiffText  string `json:"diffText"`
	Highlight bool   `json:"highlight"`
}

// CohortPanel compares +1 up and +1 down. It is research description, not a trade signal.
type CohortPanel struct {
	OK          bool             `json:"ok"`
	Message     string           `json:"message,omitempty"`
	Note        string           `json:"note"`
	Warning     string           `json:"warning,omitempty"`
	SizeNote    string           `json:"sizeNote"`
	BrowseNote  string           `json:"browseNote,omitempty"`
	Excluded    int              `json:"excluded"`
	Up          CohortGroup      `json:"up"`
	Down        CohortGroup      `json:"down"`
	Flat        CohortGroup      `json:"flat"`
	Contrasts   []CohortContrast `json:"contrasts"`
	LargeSample bool             `json:"largeSample"`
}

// View is the attribution table returned to the UI.
type View struct {
	OK                 bool        `json:"ok"`
	Message            string      `json:"message,omitempty"`
	Disclaimer         string      `json:"disclaimer"`
	ScopeNote          string      `json:"scopeNote"`
	CalendarNote       string      `json:"calendarNote"`
	BarNote            string      `json:"barNote"`
	ResearchStatLabel  string      `json:"researchStatLabel"`
	SnapshotID         uint        `json:"snapshotId"`
	TradeDate          string      `json:"tradeDate"`
	Session            string      `json:"session"`
	StrategyID         string      `json:"strategyId"`
	StrategyName       string      `json:"strategyName"`
	Page               int         `json:"page"`
	PageSize           int         `json:"pageSize"`
	Total              int         `json:"total"`
	Rows               []HitRow    `json:"rows"`
	Summary            Summary     `json:"summary"`
	ToDateNote         string      `json:"toDateNote"`
	LargeSample        bool        `json:"largeSample"`
	LargeSampleWarning string      `json:"largeSampleWarning,omitempty"`
	Cohort             CohortPanel `json:"cohort"`
	WhatIf             WhatIfPanel `json:"whatIf"`
}

// WhatIfFilter is one as-of feature rule taken from the +1 cohort contrast.
// The predicate never reads a forward return.
type WhatIfFilter struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Rule    string `json:"rule"`
	Enabled bool   `json:"enabled"`
}

// WhatIfArm is an equal-weight summary. HitCount is the subset size.
// Horizon means skip incomplete rows instead of filling them with zero.
type WhatIfArm struct {
	Label       string        `json:"label"`
	HitCount    int           `json:"hitCount"`
	CompleteAll int           `json:"completeAll"`
	Horizons    []HorizonStat `json:"horizons"`
	ToDate      HorizonStat   `json:"toDate"`
}

// WhatIfPanel compares the filtered snapshot with a feature subset.
// It is a research control, not a trade instruction.
type WhatIfPanel struct {
	OK           bool           `json:"ok"`
	Message      string         `json:"message,omitempty"`
	Note         string         `json:"note"`
	Warning      string         `json:"warning,omitempty"`
	InSampleNote string         `json:"inSampleNote"`
	WeightNote   string         `json:"weightNote"`
	Filters      []WhatIfFilter `json:"filters"`
	Baseline     WhatIfArm      `json:"baseline"`
	Scenario     WhatIfArm      `json:"scenario"`
}

// AssembleOptions controls sort and the unfiltered snapshot count.
// SortKey is "1", "3", "10", or "toDate". Unknown keys keep snapshot order.
// Rows with a missing return stay at the end in both directions.
type AssembleOptions struct {
	SortKey          string
	SortDesc         bool
	SnapshotHitCount int
	// WhatIfSet distinguishes "use highlighted defaults" from "user cleared every feature".
	WhatIfSet  bool
	WhatIfKeys []string
}

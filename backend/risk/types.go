package risk

type ReasonCode string

const (
	ReasonApproved               ReasonCode = "APPROVED"
	ReasonInvalidOrder           ReasonCode = "INVALID_ORDER"
	ReasonCashInsufficient       ReasonCode = "CASH_INSUFFICIENT"
	ReasonPositionInsufficient   ReasonCode = "POSITION_INSUFFICIENT"
	ReasonMarginModeRequired     ReasonCode = "MARGIN_MODE_REQUIRED"
	ReasonFinanceCreditExceeded  ReasonCode = "FINANCE_CREDIT_EXCEEDED"
	ReasonSecurityCreditExceeded ReasonCode = "SECURITY_CREDIT_EXCEEDED"
	ReasonMarginInsufficient     ReasonCode = "MARGIN_INSUFFICIENT"
	ReasonBorrowUnavailable      ReasonCode = "BORROW_UNAVAILABLE"
	ReasonDebtNotFound           ReasonCode = "DEBT_NOT_FOUND"
	ReasonCloseoutTriggered      ReasonCode = "CLOSEOUT_TRIGGERED"
	ReasonWarningTriggered       ReasonCode = "WARNING_TRIGGERED"
	ReasonMarketLevelBlocked     ReasonCode = "MARKET_LEVEL_BLOCKED"
	ReasonGrossExposureExceeded  ReasonCode = "GROSS_EXPOSURE_EXCEEDED"
	ReasonSingleNameExceeded     ReasonCode = "SINGLE_NAME_EXCEEDED"
	ReasonDailyLossHalt          ReasonCode = "DAILY_LOSS_HALT"
)

const (
	OrderNormalBuy  = "normal_buy"
	OrderNormalSell = "normal_sell"
	OrderMarginBuy  = "margin_buy"
	OrderSellRepay  = "sell_repay"
	OrderShortSell  = "short_sell"
	OrderBuyReturn  = "buy_return"
)

type RiskOrder struct {
	Kind      string  `json:"kind"`
	StockCode string  `json:"stockCode"`
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
}

// RiskContext 只包含风控计算所需的稳定快照，避免风控包依赖数据库或执行实现。
// 市场等级语义：1=最防守，5=最进攻；与两融杠杆指标相互独立。
type RiskContext struct {
	AccountID                 uint      `json:"accountId"`
	AccountMode               string    `json:"accountMode"`
	Cash                      float64   `json:"cash"`
	CollateralValue           float64   `json:"collateralValue"`
	LongMarketValue           float64   `json:"longMarketValue"`
	ShortMarketValue          float64   `json:"shortMarketValue"`
	FinancePrincipal          float64   `json:"financePrincipal"`
	FinanceInterest           float64   `json:"financeInterest"`
	SecuritiesFee             float64   `json:"securitiesFee"`
	FinanceCreditAvailable    float64   `json:"financeCreditAvailable"`
	SecuritiesCreditAvailable float64   `json:"securitiesCreditAvailable"`
	MarginAvailable           float64   `json:"marginAvailable"`
	PositionSellable          int64     `json:"positionSellable"`
	FinanceDebt               float64   `json:"financeDebt"`
	SecuritiesDebtQuantity    int64     `json:"securitiesDebtQuantity"`
	BorrowAvailable           int64     `json:"borrowAvailable"`
	FinanceMarginRatio        float64   `json:"financeMarginRatio"`
	SecuritiesMarginRatio     float64   `json:"securitiesMarginRatio"`
	WarningRatio              float64   `json:"warningRatio"`
	CloseoutRatio             float64   `json:"closeoutRatio"`

	// 市场层（与两融杠杆分开）
	MarketLevel      int     `json:"marketLevel"`
	BlockNewEntries  bool    `json:"blockNewEntries"`
	MaxExposurePct   float64 `json:"maxExposurePct"`

	// 组合层：下单前/后估计值由调用方填入，引擎按阈值裁决
	EquityBase          float64 `json:"equityBase"`
	CurrentNameValue    float64 `json:"currentNameValue"`
	PostGrossExposure   float64 `json:"postGrossExposure"`
	PostNameExposure    float64 `json:"postNameExposure"`
	MaxSingleNamePct    float64 `json:"maxSingleNamePct"`
	MaxGrossExposurePct float64 `json:"maxGrossExposurePct"`
	MaxDailyLossPct     float64 `json:"maxDailyLossPct"`
	CurrentDailyPnlPct  float64 `json:"currentDailyPnlPct"`

	Order RiskOrder `json:"order"`
}

type RiskDecision struct {
	Allowed bool       `json:"allowed"`
	Code    ReasonCode `json:"code"`
	Message string     `json:"message"`
	Metrics Metrics    `json:"metrics"`
}

type Metrics struct {
	TotalAssets       float64 `json:"totalAssets"`
	TotalLiabilities  float64 `json:"totalLiabilities"`
	NetExposure       float64 `json:"netExposure"`
	GrossExposure     float64 `json:"grossExposure"`
	MaintenanceRatio  float64 `json:"maintenanceRatio"`
	MarginAvailable   float64 `json:"marginAvailable"`
	RequiredOrderBond float64 `json:"requiredOrderBond"`
	GrossExposurePct  float64 `json:"grossExposurePct"`
	SingleNamePct     float64 `json:"singleNamePct"`
}

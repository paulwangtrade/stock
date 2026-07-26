// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A1 Market Data Intelligence Layer 只读接口与最小公共模型

// Package marketdata 定义 Market Data Intelligence Layer（行情智能层）的只读接口。
//
// Phase7-A1 只提供「接口 + 最小模型 + 参数规范化辅助」，具体实现由 marketdata/adapter
// 委托给现有行情实现（backend/data 中的 EastMoneyKLineApi / StockDataApi），不迁移任何
// 现有调用方。
//
// 领域边界（不可越界）：
//   - 本层只负责行情读取（K 线、实时快照），只读、无副作用；
//   - 本层模型只含市场数据字段，禁止出现 TradePlan / Risk / Position / Order 字段；
//   - 交易价量的唯一来源仍是 Frozen Spec，本层不参与下单、成交、风控决策。
package marketdata

import (
	"errors"
	"time"
)

// 统一错误：调用方可用 errors.Is 判定失败原因，避免各消费者各写一套字符串判断。
var (
	// ErrEmptyCode 股票代码为空。
	ErrEmptyCode = errors.New("marketdata: empty stock code")
	// ErrEmptyCodes 批量查询未提供任何有效代码。
	ErrEmptyCodes = errors.New("marketdata: empty stock codes")
	// ErrInvalidLimit limit 必须为正整数。
	ErrInvalidLimit = errors.New("marketdata: limit must be positive")
	// ErrUnsupportedPeriod 不支持的 K 线周期。
	ErrUnsupportedPeriod = errors.New("marketdata: unsupported period")
	// ErrUnsupportedAdjust 不支持的复权方式。
	ErrUnsupportedAdjust = errors.New("marketdata: unsupported adjust")
	// ErrNoData 上游返回空数据（区别于网络/解析错误）。
	ErrNoData = errors.New("marketdata: no data")
	// ErrProviderUnavailable 底层 provider 未初始化或不可用。
	ErrProviderUnavailable = errors.New("marketdata: provider unavailable")
)

// Bar 一根 K 线，仅包含市场数据字段。
//
// TimeText 保留上游原始时间文本（日线为 2006-01-02，分钟线为 2006-01-02 15:04），
// Time 为解析后的本地时间；解析失败时 Time 为零值而 TimeText 仍保留，便于排查。
type Bar struct {
	Code   string `json:"code"`   // 请求时使用的股票代码（未做市场改写）
	Period string `json:"period"` // 规范化后的周期码，见 Period* 常量
	Adjust string `json:"adjust"` // 规范化后的复权方式，见 Adjust* 常量

	Time     time.Time `json:"time"`
	TimeText string    `json:"timeText"`

	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`

	Volume float64 `json:"volume"` // 成交量（手）
	Amount float64 `json:"amount"` // 成交额（元）

	Amplitude     float64 `json:"amplitude"`     // 振幅（%）
	ChangePercent float64 `json:"changePercent"` // 涨跌幅（%）
	ChangeValue   float64 `json:"changeValue"`   // 涨跌额（元）
	TurnoverRate  float64 `json:"turnoverRate"`  // 换手率（%）
}

// Quote 一只标的的实时行情快照，仅包含市场数据字段。
//
// 明确不包含（避免领域泄漏）：成本价、持仓量、盈亏、关注价、报警阈值、订单/计划字段。
type Quote struct {
	Code string `json:"code"`
	Name string `json:"name"`

	Price    float64 `json:"price"`
	Open     float64 `json:"open"`
	PreClose float64 `json:"preClose"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`

	Volume float64 `json:"volume"` // 成交量（股）
	Amount float64 `json:"amount"` // 成交额（元）

	ChangePercent float64 `json:"changePercent"` // 涨跌幅（%）
	ChangeValue   float64 `json:"changeValue"`   // 涨跌额（元）

	Bid float64 `json:"bid"` // 竞买价 / 买一
	Ask float64 `json:"ask"` // 竞卖价 / 卖一

	Market string `json:"market"`
	Date   string `json:"date"` // 上游返回的日期文本
	Time   string `json:"time"` // 上游返回的时间文本

	FetchedAt time.Time `json:"fetchedAt"` // 本层取到快照的时刻
}

// KlineService 统一 K 线读取入口。
//
// endTime 为零值表示「取到最新」；否则返回 endTime 之前（含）的最多 limit 根 K 线。
// 返回切片按时间升序，len 可能小于 limit（上游数据不足）。
type KlineService interface {
	GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]Bar, error)
}

// QuoteService 统一实时行情读取入口，只读。
type QuoteService interface {
	GetQuote(code string) (*Quote, error)
	GetQuotes(codes []string) ([]Quote, error)
}
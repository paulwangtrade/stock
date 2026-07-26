// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A1 KlineService Legacy Adapter：委托 EastMoneyKLineApi，不改旧实现

// Package adapter 把 marketdata 只读接口桥接到现有行情实现。
//
// Phase7-A1 约束：
//   - 只做委托与字段映射，不新增缓存、不改变复权/切片策略（属 Phase7-A2）；
//   - 不修改 backend/data 中任何旧 API，不删除 GetKLineData；
//   - 不迁移任何现有调用方，本包目前零生产调用方。
package adapter

import (
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

// eastMoneyKlineFetcher 旧实现中本 Adapter 实际依赖的最小方法集，便于单测注入 fake。
type eastMoneyKlineFetcher interface {
	GetKLineDataBefore(stockCode, kLineType, adjustFlag string, limit int, end string) *[]data.KLineData
}

// EastMoneyKlineAdapter marketdata.KlineService 的东财实现（纯委托）。
type EastMoneyKlineAdapter struct {
	api eastMoneyKlineFetcher
}

var _ marketdata.KlineService = (*EastMoneyKlineAdapter)(nil)

// NewEastMoneyKlineAdapter 使用当前配置创建 Adapter（与旧调用方同源配置）。
func NewEastMoneyKlineAdapter() *EastMoneyKlineAdapter {
	return &EastMoneyKlineAdapter{api: data.NewEastMoneyKLineApi(data.GetSettingConfig())}
}

// NewEastMoneyKlineAdapterWith 注入自定义 fetcher，供测试或未来替换 provider 使用。
func NewEastMoneyKlineAdapterWith(api eastMoneyKlineFetcher) *EastMoneyKlineAdapter {
	return &EastMoneyKlineAdapter{api: api}
}

// GetBars 实现 marketdata.KlineService：校验入参后委托 GetKLineDataBefore 并做字段映射。
func (a *EastMoneyKlineAdapter) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	req, err := marketdata.NormalizeBarsRequest(code, period, adjust, limit, endTime)
	if err != nil {
		return nil, err
	}
	if a == nil || a.api == nil {
		return nil, marketdata.ErrProviderUnavailable
	}

	raw := a.api.GetKLineDataBefore(req.Code, req.Period, req.Adjust, req.Limit, req.End)
	if raw == nil || len(*raw) == 0 {
		return nil, marketdata.ErrNoData
	}

	bars := make([]marketdata.Bar, 0, len(*raw))
	for _, k := range *raw {
		bars = append(bars, klineToBar(req, k))
	}
	return bars, nil
}

// klineToBar 把旧的字符串 K 线结构映射为 marketdata.Bar；MA 等派生字段不搬入本层。
func klineToBar(req marketdata.BarsRequest, k data.KLineData) marketdata.Bar {
	return marketdata.Bar{
		Code:          req.Code,
		Period:        req.Period,
		Adjust:        req.Adjust,
		Time:          marketdata.ParseBarTime(k.Day),
		TimeText:      strings.TrimSpace(k.Day),
		Open:          parseFloat(k.Open),
		High:          parseFloat(k.High),
		Low:           parseFloat(k.Low),
		Close:         parseFloat(k.Close),
		Volume:        parseFloat(k.Volume),
		Amount:        parseFloat(k.Amount),
		Amplitude:     parseFloat(k.Amplitude),
		ChangePercent: parseFloat(k.ChangePercent),
		ChangeValue:   parseFloat(k.ChangeValue),
		TurnoverRate:  parseFloat(k.TurnoverRate),
	}
}

// parseFloat 上游字段为字符串且可能是 "-" / "" / "null"，无法解析时按 0 处理。
func parseFloat(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" || s == "-" || s == "null" {
		return 0
	}
	s = strings.TrimSuffix(s, "%")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

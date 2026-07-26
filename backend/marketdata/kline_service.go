// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A1 KlineService 参数规范化与请求校验（不含任何取数实现）

package marketdata

import (
	"strings"
	"time"
)

// 周期码与 backend/data 的 KLineType 取值保持一致，便于 Adapter 直接透传给旧实现。
// 这里重新声明常量而不 import backend/data，是为了让本包保持零业务依赖。
const (
	Period1Min     = "1"   // 1 分钟
	Period5Min     = "5"   // 5 分钟
	Period15Min    = "15"  // 15 分钟
	Period30Min    = "30"  // 30 分钟
	Period60Min    = "60"  // 60 分钟
	Period120Min   = "120" // 120 分钟
	PeriodDay      = "101" // 日 K
	PeriodWeek     = "102" // 周 K
	PeriodMonth    = "103" // 月 K
	PeriodQuarter  = "104" // 季 K
	PeriodHalfYear = "105" // 半年 K
	PeriodYear     = "106" // 年 K
)

// 复权方式，取值与旧实现 getAdjustType 接受的 adjustFlag 一致。
const (
	AdjustNone     = ""    // 不复权
	AdjustForward  = "qfq" // 前复权
	AdjustBackward = "hfq" // 后复权
)

// LatestEndFlag 旧实现约定的「取到最新」结束时间标记。
const LatestEndFlag = "20500101"

// periodAliases 把外部（前端 / Agent / 自然语言）常见写法映射为标准周期码。
var periodAliases = map[string]string{
	"1": Period1Min, "1m": Period1Min, "1min": Period1Min,
	"5": Period5Min, "5m": Period5Min, "5min": Period5Min,
	"15": Period15Min, "15m": Period15Min, "15min": Period15Min,
	"30": Period30Min, "30m": Period30Min, "30min": Period30Min,
	"60": Period60Min, "60m": Period60Min, "60min": Period60Min, "1h": Period60Min,
	"120": Period120Min, "120m": Period120Min, "120min": Period120Min, "2h": Period120Min,
	"101": PeriodDay, "d": PeriodDay, "1d": PeriodDay, "day": PeriodDay, "daily": PeriodDay,
	"102": PeriodWeek, "w": PeriodWeek, "1w": PeriodWeek, "week": PeriodWeek, "weekly": PeriodWeek,
	"103": PeriodMonth, "mo": PeriodMonth, "1mo": PeriodMonth, "month": PeriodMonth, "monthly": PeriodMonth,
	"104": PeriodQuarter, "q": PeriodQuarter, "quarter": PeriodQuarter,
	"105": PeriodHalfYear, "halfyear": PeriodHalfYear,
	"106": PeriodYear, "y": PeriodYear, "1y": PeriodYear, "year": PeriodYear, "yearly": PeriodYear,
}

// minutePeriods 分钟级周期集合，决定 endTime 的格式化方式。
var minutePeriods = map[string]bool{
	Period1Min:   true,
	Period5Min:   true,
	Period15Min:  true,
	Period30Min:  true,
	Period60Min:  true,
	Period120Min: true,
}

// NormalizePeriod 规范化周期；空字符串视为日 K（与多数旧调用方默认一致）。
func NormalizePeriod(period string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(period))
	if key == "" {
		return PeriodDay, nil
	}
	if std, ok := periodAliases[key]; ok {
		return std, nil
	}
	return "", ErrUnsupportedPeriod
}

// NormalizeAdjust 规范化复权方式；空字符串表示不复权。
func NormalizeAdjust(adjust string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(adjust)) {
	case "", "none", "0", "bfq":
		return AdjustNone, nil
	case AdjustForward, "1", "forward", "前复权":
		return AdjustForward, nil
	case AdjustBackward, "2", "backward", "后复权":
		return AdjustBackward, nil
	default:
		return "", ErrUnsupportedAdjust
	}
}

// IsMinutePeriod 判断是否分钟级周期。
func IsMinutePeriod(period string) bool {
	std, err := NormalizePeriod(period)
	if err != nil {
		return false
	}
	return minutePeriods[std]
}

// BarsRequest 规范化后的 K 线请求参数。
type BarsRequest struct {
	Code   string
	Period string
	Adjust string
	Limit  int
	// End 上游格式的结束时间：分钟线 YYYYMMDDHHmmss，其余 YYYYMMDD；取最新为 LatestEndFlag。
	End string
}

// NormalizeBarsRequest 校验并规范化 GetBars 入参，供各 Adapter 复用，保证行为一致。
func NormalizeBarsRequest(code string, period string, adjust string, limit int, endTime time.Time) (BarsRequest, error) {
	req := BarsRequest{}

	req.Code = strings.TrimSpace(code)
	if req.Code == "" {
		return BarsRequest{}, ErrEmptyCode
	}
	if limit <= 0 {
		return BarsRequest{}, ErrInvalidLimit
	}
	req.Limit = limit

	std, err := NormalizePeriod(period)
	if err != nil {
		return BarsRequest{}, err
	}
	req.Period = std

	adj, err := NormalizeAdjust(adjust)
	if err != nil {
		return BarsRequest{}, err
	}
	req.Adjust = adj
	req.End = FormatEndTime(std, endTime)

	return req, nil
}

// FormatEndTime 把 endTime 转成上游约定的结束时间字符串；零值表示取最新。
func FormatEndTime(period string, endTime time.Time) string {
	if endTime.IsZero() {
		return LatestEndFlag
	}
	if minutePeriods[period] {
		return endTime.Format("20060102150405")
	}
	return endTime.Format("20060102")
}

// ParseBarTime 解析上游 K 线时间文本；无法识别时返回零值，由调用方保留原始文本。
func ParseBarTime(text string) time.Time {
	s := strings.TrimSpace(text)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"20060102150405",
		"20060102",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

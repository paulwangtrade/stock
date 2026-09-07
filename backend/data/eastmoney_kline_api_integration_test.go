//go:build integration

package data

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/util"
	"testing"
)

// @Author spark
// @Date 2026/3/15
// @Desc 东方财富 K 线数据 API 集成测试（外网）

func init() {
	db.Init("../../data/stock.db")
}

func TestEastMoneyKLineApi_GetDayKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试获取日 K 线数据
	stockCode := "601857.SH"
	kLines := api.GetDayKLine(stockCode, 30)

	logger.SugaredLogger.Infof("获取到 %d 条日 K 线数据", len(*kLines))

	if len(*kLines) == 0 {
		t.Error("获取日 K 线数据失败")
		return
	}

	// 打印前 5 条数据
	for i := 0; i < len(*kLines) && i < 5; i++ {
		kline := (*kLines)[i]
		logger.SugaredLogger.Infof("第%d天 - 日期：%s, 开盘:%s, 收盘:%s, 最高:%s, 最低:%s, 成交量:%s 成交额：%s 振幅:%s 涨跌幅:%s 涨跌额:%s 换手率:%s",
			i+1, kline.Day, kline.Open, kline.Close, kline.High, kline.Low, kline.Volume, kline.Amount, kline.Amplitude, kline.ChangePercent, kline.ChangeValue, kline.TurnoverRate)
	}
}

func TestEastMoneyKLineApi_GetWeekKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试获取周 K 线数据
	stockCode := "600519.SH" // 贵州茅台
	kLines := api.GetWeekKLine(stockCode, 10)

	logger.SugaredLogger.Infof("获取到 %d 条周 K 线数据", len(*kLines))

	if len(*kLines) == 0 {
		t.Error("获取周 K 线数据失败")
		return
	}

	// 打印前 3 条数据
	for i := 0; i < len(*kLines) && i < 3; i++ {
		kline := (*kLines)[i]
		logger.SugaredLogger.Infof("第%d周 - 日期：%s, 开盘:%s, 收盘:%s, 最高:%s, 最低:%s, 成交量:%s 成交额：%s 振幅:%s 涨跌幅:%s 涨跌额:%s 换手率:%s",
			i+1, kline.Day, kline.Open, kline.Close, kline.High, kline.Low, kline.Volume, kline.Amount, kline.Amplitude, kline.ChangePercent, kline.ChangeValue, kline.TurnoverRate)
	}

}

func TestEastMoneyKLineApi_GetMonthKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试获取月 K 线数据
	stockCode := "100.HSI" // 长和
	kLines := api.GetMonthKLine(stockCode, 12)

	logger.SugaredLogger.Infof("获取到 %d 条月 K 线数据", len(*kLines))

	if len(*kLines) == 0 {
		t.Error("获取月 K 线数据失败")
		return
	}

	// 打印前 3 条数据
	for i := 0; i < len(*kLines) && i < 3; i++ {
		kline := (*kLines)[i]
		logger.SugaredLogger.Infof("第%d月 - 日期：%s, 开盘:%s, 收盘:%s, 最高:%s, 最低:%s, 成交量:%s 成交额：%s 振幅:%s 涨跌幅:%s 涨跌额:%s 换手率:%s",
			i+1, kline.Day, kline.Open, kline.Close, kline.High, kline.Low, kline.Volume, kline.Amount, kline.Amplitude, kline.ChangePercent, kline.ChangeValue, kline.TurnoverRate)
	}
}

func TestEastMoneyKLineApi_GetAdjustedKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试获取前复权 K 线数据
	stockCode := "300750.SZ" // 宁德时代
	kLines := api.GetAdjustedKLine(stockCode, "qfq", 30)

	logger.SugaredLogger.Infof("获取到 %d 条前复权日 K 线数据", len(*kLines))

	if len(*kLines) == 0 {
		t.Error("获取前复权 K 线数据失败")
		return
	}

	// 打印前 3 条数据
	for i := 0; i < len(*kLines) && i < 3; i++ {
		kline := (*kLines)[i]
		logger.SugaredLogger.Infof("第%d天 - 日期：%s, 开盘:%s, 收盘:%s, 最高:%s, 最低:%s",
			i+1, kline.Day, kline.Open, kline.Close, kline.High, kline.Low)
	}
}

func TestEastMoneyKLineApi_GetMinuteKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试获取 5 分钟 K 线数据
	stockCode := "000001.SZ" // 平安银行
	kLines := api.GetMinuteKLine(stockCode, KLineType1Min, 10)

	logger.SugaredLogger.Infof("获取到 %d 条 5 分钟 K 线数据", len(*kLines))

	if len(*kLines) == 0 {
		t.Error("获取 5 分钟 K 线数据失败")
		return
	}

	// 打印前 5 条数据
	for i := 0; i < len(*kLines) && i < 5; i++ {
		kline := (*kLines)[i]
		logger.SugaredLogger.Infof("第%d条 - 时间：%s, 开盘:%s, 收盘:%s, 最高:%s, 最低:%s, 成交量:%s",
			i+1, kline.Day, kline.Open, kline.Close, kline.High, kline.Low, kline.Volume)
	}
}

func TestEastMoneyKLineApi_GetLatestKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试获取最新 K 线
	stockCode := "000001.SZ"
	latestKLine := api.GetLatestKLine(stockCode, "101")

	if latestKLine == nil {
		t.Error("获取最新 K 线失败")
		return
	}

	logger.SugaredLogger.Infof("最新 K 线 - 日期：%s, 开盘:%s, 收盘:%s, 最高:%s, 最低:%s, 成交量:%s",
		latestKLine.Day, latestKLine.Open, latestKLine.Close, latestKLine.High, latestKLine.Low, latestKLine.Volume)
}

func TestEastMoneyKLineApi_GetBatchKLineData(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)

	// 测试批量获取多只股票的 K 线数据
	stockCodes := []string{
		"000001.SZ", // 平安银行
		"600519.SH", // 贵州茅台
		"300750.SZ", // 宁德时代
	}

	result := api.GetBatchKLineData(stockCodes, "101", 10)

	if len(result) != len(stockCodes) {
		t.Errorf("批量获取 K 线数据失败，期望获取%d只股票的数据，实际获取%d只", len(stockCodes), len(result))
		return
	}

	for code, kLines := range result {
		logger.SugaredLogger.Infof("股票%s获取到%d条 K 线数据", code, len(*kLines))
		if len(*kLines) == 0 {
			t.Errorf("股票%s的 K 线数据为空", code)
		}
	}
}

func TestGetKLineWithMA(t *testing.T) {
	config := GetSettingConfig()
	api := NewEastMoneyKLineApi(config)
	kLines, err := api.GetKLineWithMA("000001.SZ", "101", 10, 5, 10, 20, 60, 120)
	if err != nil {
		t.Errorf("GetKLineWithMA() error = %v", err)
		return
	}
	logger.SugaredLogger.Infof("GetKLineWithMA() = %v", util.MarkdownTableWithTitle("K 线数据", kLines))
}

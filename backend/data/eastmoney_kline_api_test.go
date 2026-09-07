package data

import (
	"go-stock/backend/logger"
	"testing"
)

// @Author spark
// @Date 2026/3/15
// @Desc 东方财富 K 线数据 API 单元测试（无外网）

func TestEastMoneyKLineApi_ConvertStockCode(t *testing.T) {
	api := NewEastMoneyKLineApi(&SettingConfig{})

	testCases := []struct {
		input    string
		expected string
	}{
		{"000001.SZ", "0.000001"},
		{"600000.SH", "1.600000"},
		{"00700.HK", "128.00700"},
		{"sz000001", "0.000001"},
		{"sh600000", "1.600000"},
		// 纯数字无市场后缀：当前实现仅在 validator.IsNumber 判定为数字时映射市场
		{"000001", "000001"},
		{"600000", "600000"},
	}

	for _, tc := range testCases {
		result := api.convertStockCode(tc.input)
		if result != tc.expected {
			t.Errorf("convertStockCode(%s) = %s, expected %s", tc.input, result, tc.expected)
		} else {
			logger.SugaredLogger.Infof("convertStockCode(%s) = %s ✓", tc.input, result)
		}
	}
}

func TestEastMoneyKLineApi_ValidateStockCode(t *testing.T) {
	api := NewEastMoneyKLineApi(&SettingConfig{})

	testCases := []struct {
		code     string
		expected bool
	}{
		{"000001.SZ", true},
		{"600000.SH", true},
		{"00700.HK", true},
		// ValidateStockCode 仅检查 convert 结果非空，非真实有效性校验
		{"invalid", true},
		{"", false},
	}

	for _, tc := range testCases {
		result := api.ValidateStockCode(tc.code)
		if result != tc.expected {
			t.Errorf("ValidateStockCode(%s) = %v, expected %v", tc.code, result, tc.expected)
		} else {
			logger.SugaredLogger.Infof("ValidateStockCode(%s) = %v ✓", tc.code, result)
		}
	}
}

package data

import (
	"strconv"

	"github.com/duke-git/lancet/v2/convertor"

	"go-stock/backend/models"
)

// NormalizeSignalScanHit 补齐 signal_price_status，且禁止用 NEW_PRICE 回填 signal_price。
func NormalizeSignalScanHit(h *models.SignalScanHit) {
	if h == nil {
		return
	}
	if h.SignalPrice > 0 {
		if h.SignalPriceStatus == "" {
			h.SignalPriceStatus = models.SignalPriceStatusFrozen
		}
		return
	}
	h.SignalPrice = 0
	h.SignalPriceStatus = models.SignalPriceStatusMissing
}

// NormalizeSignalScanHits 批量规范化快照 hit（读取路径）。
func NormalizeSignalScanHits(hits []models.SignalScanHit) []models.SignalScanHit {
	for i := range hits {
		NormalizeSignalScanHit(&hits[i])
	}
	return hits
}

func mapOptionalIntField(m map[string]any, keys ...string) *int {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		d, err := strconv.Atoi(convertor.ToString(v))
		if err != nil {
			continue
		}
		return &d
	}
	return nil
}

func mapSignalPriceFields(h *models.SignalScanHit, m map[string]any) {
	h.SchemaVersion = convertor.ToString(m["schema_version"])
	h.SignalTime = convertor.ToString(m["signal_time"])
	h.SignalPriceSource = convertor.ToString(m["signal_price_source"])
	h.SignalBarRole = convertor.ToString(m["signal_bar_role"])
	h.SignalPriceStatus = convertor.ToString(m["signal_price_status"])
	if v, ok := m["signal_price"]; ok {
		h.SignalPrice, _ = convertor.ToFloat(v)
	}
	h.SignalDaysAgo = mapOptionalIntField(m, "signal_days_ago")
	h.SignalBarIndex = mapOptionalIntField(m, "signal_bar_index")
	h.ConfirmBarIndex = mapOptionalIntField(m, "confirm_bar_index")
}

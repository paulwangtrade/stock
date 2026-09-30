package data

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/signalattribution"
	"go-stock/backend/tradingcalendar"
)

// BuildSignalScanAttribution reads one signal-scan snapshot and local day bars.
// It never calls a quote HTTP API and never writes trade plans, candidate pools, or broker state.
func BuildSignalScanAttribution(q *signalattribution.Query) *signalattribution.View {
	if q == nil {
		q = &signalattribution.Query{}
	}
	if db.Dao == nil {
		return signalattribution.EmptyView("本地数据库不可用")
	}
	snap, msg := resolveAttributionSnapshot(q)
	if snap == nil {
		return signalattribution.EmptyView(msg)
	}
	hits, payload, errMsg := attributionHits(snap)
	strategyID, strategyName := resolveAttributionStrategy(snap, payload)
	if errMsg != "" {
		view := signalattribution.EmptyView(errMsg)
		view.SnapshotID = snap.ID
		view.TradeDate = signalattribution.NormalizeDay(snap.TradeDate)
		view.Session = snap.Session
		view.StrategyID = strategyID
		view.StrategyName = strategyName
		return view
	}
	filtered := signalattribution.FilterHits(hits, q.SignalTags, q.ReboundMaxRsi)
	asOf := signalattribution.NormalizeDay(snap.TradeDate)
	bars := loadAttributionDayBars(filtered, asOf)
	// Weekend-only calendar: holiday weeks fail closed until a holiday table is wired.
	return signalattribution.Assemble(signalattribution.SnapshotMeta{
		ID:           snap.ID,
		TradeDate:    snap.TradeDate,
		Session:      snap.Session,
		StrategyID:   strategyID,
		StrategyName: strategyName,
	}, filtered, bars, tradingcalendar.Calendar{}, q.Page, q.PageSize, signalattribution.AssembleOptions{
		SortKey:          q.SortKey,
		SortDesc:         q.SortDesc,
		SnapshotHitCount: len(hits),
		WhatIfSet:        q.WhatIfSet,
		WhatIfKeys:       q.WhatIfKeys,
	})
}

// resolveAttributionStrategy prefers snapshot columns, then the result payload.
// A blank id is the historical default preset (same contract as the snapshot list).
func resolveAttributionStrategy(snap *models.SignalScanSnapshot, payload models.SignalScanResultPayload) (string, string) {
	id := strings.TrimSpace(snap.StrategyID)
	name := strings.TrimSpace(snap.StrategyName)
	if id == "" {
		id = strings.TrimSpace(payload.StrategyID)
	}
	if name == "" {
		name = strings.TrimSpace(payload.StrategyName)
	}
	if id == "" && payload.Config != nil {
		id = strings.TrimSpace(payload.Config.StrategyKey)
	}
	if id == "" {
		id = signalScanDefaultStrategyID
	}
	return id, name
}

func resolveAttributionSnapshot(q *signalattribution.Query) (*models.SignalScanSnapshot, string) {
	api := NewSignalScanApi()
	if q.SnapshotID > 0 {
		snap, err := api.GetSnapshotByID(q.SnapshotID)
		if err != nil || snap == nil {
			return nil, "未找到该快照"
		}
		if st := strings.TrimSpace(snap.Status); st != "" && st != "done" {
			return nil, "快照尚未完成，不能做对照"
		}
		return snap, ""
	}
	if strings.TrimSpace(q.TradeDate) == "" && strings.TrimSpace(q.StrategyID) == "" {
		return nil, "请选择一条快照，或指定交易日与策略"
	}
	snap, err := api.GetLatestSnapshotByStrategy(q.TradeDate, q.Session, q.StrategyID)
	if err != nil || snap == nil {
		return nil, "未找到符合条件的已完成快照"
	}
	return snap, ""
}

func attributionHits(snap *models.SignalScanSnapshot) ([]signalattribution.HitInput, models.SignalScanResultPayload, string) {
	raw := strings.TrimSpace(snap.ResultJSON)
	if raw == "" {
		return nil, models.SignalScanResultPayload{}, "快照没有结果明细，已停止对照"
	}
	var payload models.SignalScanResultPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, models.SignalScanResultPayload{}, "快照结果无法读取，已停止对照"
	}
	out := make([]signalattribution.HitInput, 0, len(payload.Items))
	for _, hit := range payload.Items {
		code := strings.TrimSpace(hit.SECURITY_CODE)
		if code == "" {
			code = strings.TrimSpace(hit.SECUCODE)
		}
		barKey := ""
		if norm, err := NormalizeStockCode(firstNonEmpty(hit.SECUCODE, hit.SECURITY_CODE, code)); err == nil {
			barKey = norm.TSCode
			if code == "" {
				code = norm.TSCode
			}
		}
		item := signalattribution.HitInput{
			Code:          code,
			Name:          strings.TrimSpace(hit.SECURITY_NAME_ABBR),
			SnapshotPrice: snapshotRecordedPrice(hit),
			BarKey:        barKey,
			KlineCode:     barKey,
			Tag:           strings.TrimSpace(hit.Tag),
			Industry:      strings.TrimSpace(hit.INDUSTRY),
			Market:        strings.TrimSpace(hit.MARKET),
		}
		if v, ok := parseOptionalPositive(hit.VOLUME_RATIO); ok {
			item.VolumeRatio = v
			item.HasVolumeRatio = true
		}
		if hit.RSI > 0 && !math.IsNaN(hit.RSI) && !math.IsInf(hit.RSI, 0) && hit.RSI <= 100 {
			item.RSI = hit.RSI
			item.HasRSI = true
		}
		out = append(out, item)
	}
	return out, payload, ""
}

func parseOptionalPositive(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func snapshotRecordedPrice(hit models.SignalScanHit) float64 {
	if hit.SignalPrice > 0 && !math.IsNaN(hit.SignalPrice) && !math.IsInf(hit.SignalPrice, 0) {
		return hit.SignalPrice
	}
	p, err := strconv.ParseFloat(strings.TrimSpace(hit.NEW_PRICE), 64)
	if err != nil || p <= 0 || math.IsNaN(p) || math.IsInf(p, 0) {
		return 0
	}
	return p
}

type closeBook struct {
	value    float64
	seen     bool
	conflict bool
}

func (b *closeBook) add(v float64) {
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	if !b.seen {
		b.value = v
		b.seen = true
		return
	}
	if math.Abs(b.value-v) > 0.0001 {
		b.conflict = true
	}
}

func loadAttributionDayBars(hits []signalattribution.HitInput, asOf string) map[string][]signalattribution.DayBar {
	out := map[string][]signalattribution.DayBar{}
	if asOf == "" || len(hits) == 0 || db.Dao == nil {
		return out
	}
	start, err := tradingcalendar.ParseDate(asOf)
	if err != nil {
		return out
	}
	// Look back for as-of MA / prior return, and forward for 迄今 through the latest local bar.
	lookback := start.AddDate(0, 0, -180)
	end := start.AddDate(3, 0, 0)
	minDay := lookback.Format("2006-01-02")
	maxDay := end.Format("2006-01-02")

	keys := make([]string, 0, len(hits))
	secIDs := make([]string, 0, len(hits))
	secToKey := map[string]string{}
	seenKey := map[string]bool{}
	seenSec := map[string]bool{}
	for _, hit := range hits {
		key := strings.TrimSpace(hit.BarKey)
		if key == "" || seenKey[key] {
			continue
		}
		seenKey[key] = true
		keys = append(keys, key)
		if norm, nerr := NormalizeStockCode(key); nerr == nil && norm.EmSecID != "" && !seenSec[norm.EmSecID] {
			seenSec[norm.EmSecID] = true
			secIDs = append(secIDs, norm.EmSecID)
			secToKey[norm.EmSecID] = norm.TSCode
		}
	}

	formal := map[string]map[string]float64{}
	if len(keys) > 0 && db.Dao.Migrator().HasTable(&StockKLineDay{}) {
		var rows []StockKLineDay
		qerr := db.Dao.Where(
			"period = ? AND adjust_type = ? AND ts_code IN ? AND bar_time >= ? AND bar_time <= ?",
			KLinePeriod1D, KLineAdjustNone, keys, lookback, end,
		).Find(&rows).Error
		if qerr != nil {
			logger.SugaredLogger.Warnf("attribution day bars: %v", qerr)
		}
		for _, row := range rows {
			if row.Close <= 0 {
				continue
			}
			day := signalattribution.NormalizeDay(row.BarTime.Format("2006-01-02"))
			if day == "" {
				continue
			}
			book := formal[row.TSCode]
			if book == nil {
				book = map[string]float64{}
				formal[row.TSCode] = book
			}
			book[day] = row.Close
		}
	}

	cacheBooks := map[string]map[string]*closeBook{}
	if len(secIDs) > 0 {
		ensureKLineCacheTable()
		var rows []KLineCacheRecord
		if err := db.Dao.Where("klt = ? AND adjust_flag = ? AND stock_sec_id IN ?", "101", "", secIDs).Find(&rows).Error; err != nil {
			logger.SugaredLogger.Warnf("attribution kline cache: %v", err)
		}
		for _, row := range rows {
			key := secToKey[row.StockSecID]
			if key == "" || strings.TrimSpace(row.Payload) == "" {
				continue
			}
			var bars []KLineData
			if json.Unmarshal([]byte(row.Payload), &bars) != nil {
				continue
			}
			book := cacheBooks[key]
			if book == nil {
				book = map[string]*closeBook{}
				cacheBooks[key] = book
			}
			for _, bar := range bars {
				day := signalattribution.NormalizeDay(bar.Day)
				if day == "" || day < minDay || day > maxDay {
					continue
				}
				px, perr := strconv.ParseFloat(strings.TrimSpace(bar.Close), 64)
				if perr != nil {
					continue
				}
				slot := book[day]
				if slot == nil {
					slot = &closeBook{}
					book[day] = slot
				}
				slot.add(px)
			}
		}
	}

	for _, key := range keys {
		merged := map[string]float64{}
		for day, slot := range cacheBooks[key] {
			if slot == nil || !slot.seen || slot.conflict {
				continue
			}
			merged[day] = slot.value
		}
		for day, px := range formal[key] {
			merged[day] = px
		}
		if len(merged) == 0 {
			continue
		}
		list := make([]signalattribution.DayBar, 0, len(merged))
		for day, px := range merged {
			list = append(list, signalattribution.DayBar{Date: day, Close: px})
		}
		out[key] = list
	}
	return out
}

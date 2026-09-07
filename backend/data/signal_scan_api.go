package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"gorm.io/gorm"
)

const (
	signalScanKlineBars       = 120
	signalScanFetchPageSize   = 500
	signalScanKlineConcurrency = 32
	signalScanJSChunkSize     = 400
	signalScanDefaultStrategyID = "default"
)

type SignalScanProgress struct {
	Phase   string `json:"phase"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Session string `json:"session"`
}

type SignalScanProgressFn func(SignalScanProgress)

type SignalScanApi struct {
	kline *EastMoneyKLineApi
	stock *StockDataApi
}

func NewSignalScanApi() *SignalScanApi {
	cfg := GetSettingConfig()
	return &SignalScanApi{
		kline: NewEastMoneyKLineApi(cfg),
		stock: NewStockDataApi(),
	}
}

var (
	signalScanRunning atomic.Bool
)

func (a *SignalScanApi) IsRunning() bool {
	return signalScanRunning.Load()
}

func isScannableAShareCode(code string) bool {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return false
	}
	if strings.HasPrefix(c, "hk") || strings.HasPrefix(c, "gb_") || strings.HasPrefix(c, "us") {
		return false
	}
	return strings.HasPrefix(c, "sh") || strings.HasPrefix(c, "sz") || strings.HasPrefix(c, "bj") ||
		len(c) >= 6
}

func emCodeFromSecucode(secucode, securityCode string) string {
	s := strings.ToLower(strings.TrimSpace(secucode))
	if s != "" {
		if idx := strings.Index(s, "."); idx > 0 {
			num := s[:idx]
			suf := strings.ToUpper(s[idx+1:])
			if suf == "SH" {
				return "sh" + num
			}
			if suf == "SZ" {
				return "sz" + num
			}
			if suf == "BJ" {
				return "bj" + num
			}
		}
		if strings.HasPrefix(s, "sh") || strings.HasPrefix(s, "sz") || strings.HasPrefix(s, "bj") {
			return s
		}
	}
	sym := strings.TrimSpace(securityCode)
	if sym != "" {
		if len(sym) == 6 {
			if strings.HasPrefix(sym, "6") {
				return "sh" + sym
			}
			if strings.HasPrefix(sym, "4") || strings.HasPrefix(sym, "8") {
				return "bj" + sym
			}
			return "sz" + sym
		}
	}
	return s
}

func parseKlineFloat(s string) float64 {
	f, _ := convertor.ToFloat(strings.TrimSpace(s))
	return f
}

func (a *SignalScanApi) fetchAllMarketStocks(onProgress SignalScanProgressFn, session string) ([]models.StockInfo, error) {
	var all []models.StockInfo
	page := 1
	total := 0
	for page <= 100 {
		resp := a.stock.GetAllStocks(page, signalScanFetchPageSize, "", "", "", "", models.TechnicalIndicators{})
		if resp == nil || len(resp.Result.Data) == 0 {
			break
		}
		if resp.Result.Count > 0 {
			total = resp.Result.Count
		}
		all = append(all, resp.Result.Data...)
		onProgress(SignalScanProgress{Phase: "fetch", Done: len(all), Total: total, Session: session})
		if total > 0 && len(all) >= total {
			break
		}
		if len(resp.Result.Data) < signalScanFetchPageSize {
			break
		}
		page++
	}
	return all, nil
}

func (a *SignalScanApi) buildIndexCloseMap() map[string]float64 {
	out := map[string]float64{}
	kl := a.kline.GetKLineData("000001.SH", "101", "", signalScanKlineBars)
	if kl == nil {
		return out
	}
	for _, r := range *kl {
		k := normalizeScanDayKey(r.Day)
		if k == "" {
			continue
		}
		c := parseKlineFloat(r.Close)
		if c > 0 {
			out[k] = c
		}
	}
	return out
}

type preparedScanStock struct {
	Input signalScanStockInput
}

func (a *SignalScanApi) prepareStockBars(row models.StockInfo, now time.Time) *preparedScanStock {
	code := emCodeFromSecucode(row.SECUCODE, row.SECURITYCODE)
	if !isScannableAShareCode(code) {
		return nil
	}
	kl := a.kline.GetKLineData(code, "101", "", signalScanKlineBars)
	if kl == nil || len(*kl) < 20 {
		return nil
	}
	closes := make([]float64, 0, len(*kl))
	opens := make([]float64, 0, len(*kl))
	highs := make([]float64, 0, len(*kl))
	lows := make([]float64, 0, len(*kl))
	volumes := make([]float64, 0, len(*kl))
	dayKeys := make([]string, 0, len(*kl))
	for _, r := range *kl {
		c := parseKlineFloat(r.Close)
		if c <= 0 {
			continue
		}
		o := parseKlineFloat(r.Open)
		h := parseKlineFloat(r.High)
		l := parseKlineFloat(r.Low)
		v := parseKlineFloat(r.Volume)
		if o <= 0 {
			o = c
		}
		if h <= 0 {
			h = c
		}
		if l <= 0 {
			l = c
		}
		closes = append(closes, c)
		opens = append(opens, o)
		highs = append(highs, h)
		lows = append(lows, l)
		volumes = append(volumes, v)
		dayKeys = append(dayKeys, normalizeScanDayKey(r.Day))
	}
	if len(closes) < 20 {
		return nil
	}
	lastIdx := ResolveSignalLastBarIndex(dayKeys, now)
	rowMap := stockQuoteRowMap(row)
	return &preparedScanStock{
		Input: signalScanStockInput{
			Code:         code,
			Name:         row.SECURITYNAMEABBR,
			Secucode:     row.SECUCODE,
			Closes:       closes,
			Opens:        opens,
			Highs:        highs,
			Lows:         lows,
			Volumes:      volumes,
			DayKeys:      dayKeys,
			LastBarIndex: lastIdx,
			Row:          rowMap,
		},
	}
}

// RunFullMarketSnapshot 全市场信号扫描并落库
func (a *SignalScanApi) RunFullMarketSnapshot(session string, signalParamsOverride string, strategyID string, strategyName string, onProgress SignalScanProgressFn) (*models.SignalScanSnapshot, error) {
	session = strings.TrimSpace(session)
	if session == "" {
		session = models.SignalScanSessionClose
	}
	if session != models.SignalScanSessionMid && session != models.SignalScanSessionClose {
		return nil, fmt.Errorf("invalid session: %s", session)
	}
	if !signalScanRunning.CompareAndSwap(false, true) {
		return nil, errors.New("信号扫描正在进行中")
	}
	defer signalScanRunning.Store(false)

	start := time.Now()
	now := shanghaiNow()
	tradeDate := EffectiveSignalTradeDate(session, now)

	signalParams := ""
	if strings.TrimSpace(signalParamsOverride) != "" {
		signalParams = strings.TrimSpace(signalParamsOverride)
	} else if cfg := GetSettingConfig(); cfg != nil {
		signalParams = strings.TrimSpace(cfg.SignalParams)
	}
	strategyID = strings.TrimSpace(strategyID)
	strategyName = strings.TrimSpace(strategyName)

	rows, err := a.fetchAllMarketStocks(onProgress, session)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("未获取到股票名单")
	}

	indexClose := a.buildIndexCloseMap()
	prepared := make([]preparedScanStock, 0, len(rows))
	var prepMu sync.Mutex
	var prepDone int32
	total := len(rows)

	sem := make(chan struct{}, signalScanKlineConcurrency)
	var wg sync.WaitGroup
	for _, row := range rows {
		row := row
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p := a.prepareStockBars(row, now)
			if p != nil {
				prepMu.Lock()
				prepared = append(prepared, *p)
				prepMu.Unlock()
			}
			done := int(atomic.AddInt32(&prepDone, 1))
			if onProgress != nil && (done%20 == 0 || done == total) {
				onProgress(SignalScanProgress{Phase: "scan", Done: done, Total: total, Session: session})
			}
		}()
	}
	wg.Wait()

	allItems := make([]map[string]any, 0, 256)
	for i := 0; i < len(prepared); i += signalScanJSChunkSize {
		end := i + signalScanJSChunkSize
		if end > len(prepared) {
			end = len(prepared)
		}
		chunk := make([]signalScanStockInput, 0, end-i)
		for _, p := range prepared[i:end] {
			chunk = append(chunk, p.Input)
		}
		out, err := RunSignalScanBatchJS(signalScanBatchInput{
			Stocks:           chunk,
			IndexClose:       indexClose,
			SignalParamsJSON: signalParams,
			IncludeSell:      true,
		})
		if err != nil {
			logger.SugaredLogger.Errorf("signal scan js chunk %d: %v", i/signalScanJSChunkSize, err)
			return nil, err
		}
		allItems = append(allItems, out.Items...)
		if onProgress != nil {
			onProgress(SignalScanProgress{Phase: "compute", Done: end, Total: len(prepared), Session: session})
		}
	}

	payload := models.SignalScanResultPayload{
		Items:        mapSliceToHits(allItems),
		ScannedTotal: len(rows),
		HitTotal:     len(allItems),
		TradeDate:    tradeDate,
		Session:      session,
		StrategyID:   strategyID,
		StrategyName: strategyName,
		CompletedAt:  FormatShanghaiTime(time.Now()),
	}
	resultJSON, _ := json.Marshal(payload)

	snap := &models.SignalScanSnapshot{
		TradeDate:        tradeDate,
		Session:          session,
		Scope:            models.SignalScanScopeAll,
		StrategyID:       strategyID,
		StrategyName:     strategyName,
		SignalParamsJSON: signalParams,
		ScannedTotal:     len(rows),
		HitTotal:         len(allItems),
		Status:           "done",
		Message:          fmt.Sprintf("扫描完成：候选 %d 只，有信号 %d 只", len(rows), len(allItems)),
		ResultJSON:       string(resultJSON),
		DurationMs:       time.Since(start).Milliseconds(),
	}

	// 同交易日 + 时段 + 策略覆盖；默认策略兼容旧版未写 strategy_id 的快照。
	deleteQuery := db.Dao.Where("trade_date = ? AND session = ? AND scope = ?", tradeDate, session, models.SignalScanScopeAll)
	if strategyID == signalScanDefaultStrategyID {
		deleteQuery = deleteQuery.Where("(strategy_id = ? OR strategy_id = '' OR strategy_id IS NULL)", strategyID)
	} else {
		deleteQuery = deleteQuery.Where("strategy_id = ?", strategyID)
	}
	deleteQuery.Delete(&models.SignalScanSnapshot{})
	if err := db.Dao.Create(snap).Error; err != nil {
		return nil, err
	}
	if onProgress != nil {
		onProgress(SignalScanProgress{Phase: "done", Done: len(rows), Total: len(rows), Session: session})
	}
	return snap, nil
}

func formatSnapshotConcept(concept any) string {
	if concept == nil {
		return ""
	}
	switch v := concept.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		if strings.HasPrefix(s, "[") {
			var arr []string
			if json.Unmarshal([]byte(s), &arr) == nil {
				return strings.Join(arr, "、")
			}
		}
		return s
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if p := strings.TrimSpace(convertor.ToString(item)); p != "" {
				parts = append(parts, p)
			}
		}
		return strings.Join(parts, "、")
	case []string:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if p := strings.TrimSpace(item); p != "" {
				parts = append(parts, p)
			}
		}
		return strings.Join(parts, "、")
	default:
		raw := strings.TrimSpace(convertor.ToString(concept))
		if strings.HasPrefix(raw, "[") {
			var arr []string
			if json.Unmarshal([]byte(raw), &arr) == nil {
				return strings.Join(arr, "、")
			}
		}
		return raw
	}
}

func stockQuoteRowMap(row models.StockInfo) map[string]any {
	return map[string]any{
		"SECUCODE":           row.SECUCODE,
		"SECURITY_CODE":      row.SECURITYCODE,
		"SECURITY_NAME_ABBR": row.SECURITYNAMEABBR,
		"NEW_PRICE":          convertor.ToString(row.NEWPRICE),
		"CHANGE_RATE":        convertor.ToString(row.CHANGERATE),
		"HIGH_PRICE":         convertor.ToString(row.HIGHPRICE),
		"LOW_PRICE":          convertor.ToString(row.LOWPRICE),
		"PRE_CLOSE_PRICE":    convertor.ToString(row.PRECLOSEPRICE),
		"VOLUME":             convertor.ToString(row.VOLUME),
		"DEAL_AMOUNT":        convertor.ToString(row.DEALAMOUNT),
		"TURNOVERRATE":       convertor.ToString(row.TURNOVERRATE),
		"VOLUME_RATIO":       convertor.ToString(row.VOLUMERATIO),
		"INDUSTRY":           row.INDUSTRY,
		"CONCEPT":            formatSnapshotConcept(row.CONCEPT),
		"MARKET":             row.MARKET,
	}
}

func mapSliceToHits(items []map[string]any) []models.SignalScanHit {
	out := make([]models.SignalScanHit, 0, len(items))
	for _, m := range items {
		h := models.SignalScanHit{
			SECUCODE:           convertor.ToString(m["SECUCODE"]),
			SECURITY_CODE:      convertor.ToString(m["SECURITY_CODE"]),
			SECURITY_NAME_ABBR: convertor.ToString(m["SECURITY_NAME_ABBR"]),
			NEW_PRICE:          convertor.ToString(m["NEW_PRICE"]),
			CHANGE_RATE:        convertor.ToString(m["CHANGE_RATE"]),
			HIGH_PRICE:         convertor.ToString(m["HIGH_PRICE"]),
			LOW_PRICE:          convertor.ToString(m["LOW_PRICE"]),
			PRE_CLOSE_PRICE:    convertor.ToString(m["PRE_CLOSE_PRICE"]),
			VOLUME:             convertor.ToString(m["VOLUME"]),
			DEAL_AMOUNT:        convertor.ToString(m["DEAL_AMOUNT"]),
			TURNOVERRATE:       convertor.ToString(m["TURNOVERRATE"]),
			VOLUME_RATIO:       convertor.ToString(m["VOLUME_RATIO"]),
			INDUSTRY:           convertor.ToString(m["INDUSTRY"]),
			CONCEPT:            formatSnapshotConcept(m["CONCEPT"]),
			MARKET:             convertor.ToString(m["MARKET"]),
			Tag:                convertor.ToString(m["tag"]),
			StatusText:         convertor.ToString(m["statusText"]),
		}
		if v, ok := m["sortRank"]; ok {
			h.SortRank, _ = strconv.Atoi(convertor.ToString(v))
		}
		if v, ok := m["recentSignalDaysAgo"]; ok && v != nil {
			d, _ := strconv.Atoi(convertor.ToString(v))
			h.DaysAgo = &d
		}
		if v, ok := m["rsi"]; ok {
			h.RSI, _ = convertor.ToFloat(v)
		}
		mapSignalPriceFields(&h, m)
		NormalizeSignalScanHit(&h)
		out = append(out, h)
	}
	return out
}

func (a *SignalScanApi) ensureSnapshotTable() {
	if db.Dao == nil {
		return
	}
	if db.Dao.Migrator().HasTable(&models.SignalScanSnapshot{}) {
		return
	}
	_ = db.Dao.AutoMigrate(&models.SignalScanSnapshot{})
}

func (a *SignalScanApi) GetLatestSnapshot(tradeDate, session string) (*models.SignalScanSnapshot, error) {
	a.ensureSnapshotTable()
	q := db.Dao.Model(&models.SignalScanSnapshot{}).Where("status = ?", "done")
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("trade_date = ?", tradeDate)
	}
	if strings.TrimSpace(session) != "" {
		q = q.Where("session = ?", session)
	}
	var snap models.SignalScanSnapshot
	if err := q.Order("created_at desc").First(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

func (a *SignalScanApi) GetLatestSnapshotByStrategy(tradeDate, session string, strategyID string) (*models.SignalScanSnapshot, error) {
	a.ensureSnapshotTable()
	q := db.Dao.Model(&models.SignalScanSnapshot{}).Where("status = ?", "done")
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("trade_date = ?", tradeDate)
	}
	if strings.TrimSpace(session) != "" {
		q = q.Where("session = ?", session)
	}
	if sid := strings.TrimSpace(strategyID); sid != "" {
		if sid == signalScanDefaultStrategyID {
			q = q.Where("(strategy_id = ? OR strategy_id = '' OR strategy_id IS NULL)", sid)
		} else {
			q = q.Where("strategy_id = ?", sid)
		}
	}
	var snap models.SignalScanSnapshot
	if err := q.Order("created_at desc").First(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

// latestSnapshotMetaQuery 构建只选元数据字段的查询（不含 result_json / signal_params_json）。
func (a *SignalScanApi) latestSnapshotMetaQuery() *gorm.DB {
	return db.Dao.Model(&models.SignalScanSnapshot{}).Select(
		"id", "created_at", "trade_date", "session", "scope",
		"strategy_id", "strategy_name", "scanned_total", "hit_total",
		"status", "message", "duration_ms",
	)
}

// GetLatestSnapshotMeta 最新快照元数据（列表/横幅场景；不含 ResultJSON）。
func (a *SignalScanApi) GetLatestSnapshotMeta(tradeDate, session string) (*models.SignalScanSnapshot, error) {
	a.ensureSnapshotTable()
	q := a.latestSnapshotMetaQuery().Where("status = ?", "done")
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("trade_date = ?", tradeDate)
	}
	if strings.TrimSpace(session) != "" {
		q = q.Where("session = ?", session)
	}
	var snap models.SignalScanSnapshot
	if err := q.Order("created_at desc").First(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

// GetLatestSnapshotMetaByStrategy 按策略取最新快照元数据。
func (a *SignalScanApi) GetLatestSnapshotMetaByStrategy(tradeDate, session string, strategyID string) (*models.SignalScanSnapshot, error) {
	a.ensureSnapshotTable()
	q := a.latestSnapshotMetaQuery().Where("status = ?", "done")
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("trade_date = ?", tradeDate)
	}
	if strings.TrimSpace(session) != "" {
		q = q.Where("session = ?", session)
	}
	if sid := strings.TrimSpace(strategyID); sid != "" {
		if sid == signalScanDefaultStrategyID {
			q = q.Where("(strategy_id = ? OR strategy_id = '' OR strategy_id IS NULL)", sid)
		} else {
			q = q.Where("strategy_id = ?", sid)
		}
	}
	var snap models.SignalScanSnapshot
	if err := q.Order("created_at desc").First(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

func (a *SignalScanApi) ListSnapshots(q *models.SignalScanSnapshotQuery) *models.SignalScanSnapshotPageResp {
	a.ensureSnapshotTable()
	if q == nil {
		q = &models.SignalScanSnapshotQuery{}
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	query := db.Dao.Model(&models.SignalScanSnapshot{})
	if strings.TrimSpace(q.TradeDate) != "" {
		query = query.Where("trade_date = ?", q.TradeDate)
	}
	if strings.TrimSpace(q.Session) != "" {
		query = query.Where("session = ?", q.Session)
	}
	if sid := strings.TrimSpace(q.StrategyID); sid != "" {
		if sid == signalScanDefaultStrategyID {
			query = query.Where("(strategy_id = ? OR strategy_id = '' OR strategy_id IS NULL)", sid)
		} else {
			query = query.Where("strategy_id = ?", sid)
		}
	}
	var total int64
	query.Count(&total)
	var list []models.SignalScanSnapshot
	// 列表只返回元数据，不带 ResultJSON / SignalParamsJSON，避免数百 KB 拖慢下拉渲染
	query.Select(
		"id", "created_at", "trade_date", "session", "scope",
		"strategy_id", "strategy_name", "scanned_total", "hit_total",
		"status", "message", "duration_ms",
	).
		Order("trade_date desc, session desc, created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list)
	return &models.SignalScanSnapshotPageResp{Total: int(total), Data: list}
}

func (a *SignalScanApi) GetSnapshotByID(id uint) (*models.SignalScanSnapshot, error) {
	var snap models.SignalScanSnapshot
	if err := db.Dao.First(&snap, id).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

func FormatShanghaiTime(t time.Time) string {
	return t.In(shanghaiLoc).Format("2006-01-02 15:04:05")
}

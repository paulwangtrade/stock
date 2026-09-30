package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"strings"
)

const (
	// ObservationQueryType 内置观察策略。只产生观察名单，默认不进入模拟交易计划。
	ObservationQueryType = "observation"
	// defaultObservationCron 定时观察的默认表达式（交易日 15:05）。打开定时不会打开 feedsTradePlan。
	defaultObservationCron = "0 5 15 * * 1-5"
	observationScanCap     = 40
	observationMaPeriod    = 20
	observationMaTouchPct  = 0.02
	observationVolLookback = 20
	observationVolMult     = 1.5
	observationDdLookback  = 20
	observationDdMin       = 0.08
	observationDdMax       = 0.18
	observationNewLowBars  = 60
	observationRSIPeriod   = 14
	observationRSIMin      = 30.0
	observationHistoryBars = 80
)

// observationDef 三条内置日 K 观察规则。
// 本分支 SignalScan 的 RunSignalScanBatchJS 只计算冰点标签（强/趋/转/突/弹/买），
// 不识别 ext_ma_pullback / ext_vol_breakout / ext_dd_bounce。规则只放在这里，
// 由 RunStrategy 的 observation 分支调用，避免再做一套冰点引擎。
type observationDef struct {
	ID    string
	Name  string
	Blurb string
}

var observationCatalog = []observationDef{
	{
		ID:    "ext_ma_pullback",
		Name:  "均线趋势回踩",
		Blurb: "收盘在抬头的20日均线之上；近3日低点贴近均线后以阳线收回（有开盘价则收盘>开盘，否则收盘>前收）。只作观察名单，不是买卖指令。K线不足、价格无效或均线未抬头时跳过。",
	},
	{
		ID:    "ext_vol_breakout",
		Name:  "放量突破确认",
		Blurb: "收盘价突破此前20日最高价，且成交量不低于此前20日均量的1.5倍。只作观察名单，不是买卖指令。成交量缺失或为0、K线不足时跳过。",
	},
	{
		ID:    "ext_dd_bounce",
		Name:  "受控回撤反弹",
		Blurb: "自近20日高点回撤约8%–18%后出现阳线反弹；不是60日新低，且RSI不低于30（避开冰点式新低/超卖）。只作观察名单，不是买卖指令。K线或价格不足时跳过。",
	},
}

type observationMeta struct {
	StrategyID      string `json:"strategyId"`
	FeedsTradePlan  bool   `json:"feedsTradePlan"`
	ObservationOnly bool   `json:"observationOnly"`
}

type observationBar struct {
	Open   float64
	Close  float64
	High   float64
	Low    float64
	Volume float64
}

type observationSymbol struct {
	Code       string
	Name       string
	Industry   string
	Bars       []observationBar
	Price      float64
	ChangeRate float64
}

// loadObservationUniverse 可在测试里替换，避免访问行情。
var loadObservationUniverse = defaultLoadObservationUniverse

func observationDefByID(id string) (observationDef, bool) {
	id = strings.TrimSpace(id)
	for _, def := range observationCatalog {
		if def.ID == id {
			return def, true
		}
	}
	return observationDef{}, false
}

func observationStrategyID(s *models.StockStrategy) string {
	if s == nil {
		return ""
	}
	if meta, ok := parseObservationMeta(s.QueryJSON); ok && meta.StrategyID != "" {
		return meta.StrategyID
	}
	text := strings.TrimSpace(s.QueryText)
	if _, ok := observationDefByID(text); ok {
		return text
	}
	return ""
}

func parseObservationMeta(raw string) (observationMeta, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return observationMeta{}, false
	}
	var meta observationMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return observationMeta{}, false
	}
	return meta, true
}

// observationFeedsTradePlan 只有 JSON 布尔 true 才算纳入模拟交易计划。缺省、字符串、1 都视为 false。
func observationFeedsTradePlan(s *models.StockStrategy) bool {
	if s == nil || s.QueryType != ObservationQueryType {
		return false
	}
	has, val := explicitFeedsTradePlan(s.QueryJSON)
	return has && val
}

func explicitFeedsTradePlan(raw string) (present bool, value bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false, false
	}
	v, ok := m["feedsTradePlan"]
	if !ok {
		return false, false
	}
	return true, string(v) == "true"
}

func normalizeObservationStrategy(s *models.StockStrategy, prev *models.StockStrategy) {
	if s == nil {
		return
	}
	id := observationStrategyID(s)
	if id == "" && prev != nil {
		id = observationStrategyID(prev)
	}
	feeds := false
	if present, val := explicitFeedsTradePlan(s.QueryJSON); present {
		feeds = val
	} else if prev != nil {
		feeds = observationFeedsTradePlan(prev)
	}
	blob, _ := json.Marshal(observationMeta{
		StrategyID:      id,
		FeedsTradePlan:  feeds,
		ObservationOnly: true,
	})
	s.QueryType = ObservationQueryType
	s.QueryText = id
	s.QueryJSON = string(blob)
	s.Keyword = ""
	s.Industry = ""
	if s.Enable {
		if strings.TrimSpace(s.CronExpr) == "" {
			s.CronExpr = defaultObservationCron
		}
	} else {
		s.Enable = false
		s.CronExpr = ""
	}
	if s.PageSize <= 0 || s.PageSize > observationScanCap {
		s.PageSize = observationScanCap
	}
	if def, ok := observationDefByID(id); ok {
		if strings.TrimSpace(s.Name) == "" {
			s.Name = def.Name
		}
		if strings.TrimSpace(s.Description) == "" {
			s.Description = def.Blurb
		}
	}
}

func (a *StockStrategyApi) updateObservationColumns(s *models.StockStrategy) error {
	if s == nil || s.ID == 0 {
		return fmt.Errorf("观察策略不存在")
	}
	return db.Dao.Model(&models.StockStrategy{}).Where("id = ?", s.ID).Updates(map[string]any{
		"name":        s.Name,
		"query_type":  s.QueryType,
		"query_text":  s.QueryText,
		"query_json":  s.QueryJSON,
		"keyword":     "",
		"industry":    "",
		"cron_expr":   s.CronExpr,
		"enable":      s.Enable,
		"page_size":   s.PageSize,
		"description": s.Description,
	}).Error
}

// EnsureObservationStrategies 补齐三条内置观察策略。已存在的行不改开关，避免把用户打开的定时或 feedsTradePlan 写回去。
func (a *StockStrategyApi) EnsureObservationStrategies() error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if !db.Dao.Migrator().HasTable(&models.StockStrategy{}) {
		return fmt.Errorf("stock_strategies 表不存在")
	}
	for _, def := range observationCatalog {
		var n int64
		err := db.Dao.Model(&models.StockStrategy{}).
			Where("query_type = ? AND (query_text = ? OR query_json LIKE ?)", ObservationQueryType, def.ID, `%"strategyId":"`+def.ID+`"%`).
			Count(&n).Error
		if err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := &models.StockStrategy{
			Name:        def.Name,
			QueryType:   ObservationQueryType,
			QueryText:   def.ID,
			Description: def.Blurb,
			PageSize:    observationScanCap,
			Enable:      false,
		}
		if err := a.Create(row); err != nil {
			return err
		}
	}
	return nil
}

// ListObservationFeedingTradePlan 返回显式纳入模拟交易计划的观察策略。定时观察不会出现在这里。
func (a *StockStrategyApi) ListObservationFeedingTradePlan() []models.StockStrategy {
	if db.Dao == nil || !db.Dao.Migrator().HasTable(&models.StockStrategy{}) {
		return nil
	}
	var list []models.StockStrategy
	if err := db.Dao.Where("query_type = ?", ObservationQueryType).Order("id ASC").Find(&list).Error; err != nil {
		return nil
	}
	out := make([]models.StockStrategy, 0, len(list))
	for i := range list {
		if observationFeedsTradePlan(&list[i]) {
			out = append(out, list[i])
		}
	}
	return out
}

func (a *StockStrategyApi) runObservationStrategy(s *models.StockStrategy) *models.StockStrategyRunView {
	view := &models.StockStrategyRunView{
		StrategyID: s.ID,
		QueryType:  ObservationQueryType,
		Code:       -1,
		Message:    "观察策略无法执行",
	}
	id := observationStrategyID(s)
	def, ok := observationDefByID(id)
	if !ok {
		view.Message = "不支持的观察策略: " + id
		a.saveRun(s, view)
		return view
	}
	view.TraceInfo = def.Blurb + " 观察名单不是交易指令，不会自动下单，默认不进入模拟交易计划。"
	limit := s.PageSize
	if limit <= 0 || limit > observationScanCap {
		limit = observationScanCap
	}
	symbols := loadObservationSymbols(limit)
	hits := make([]map[string]any, 0)
	scanned := 0
	for _, sym := range symbols {
		if len(sym.Bars) == 0 || isObservationST(sym.Name) {
			continue
		}
		scanned++
		if !evalObservation(id, sym.Bars) {
			continue
		}
		hits = append(hits, observationHitRow(sym))
	}
	view.Code = 0
	view.DataList = hits
	view.StockCount = len(hits)
	switch {
	case len(symbols) == 0:
		view.Message = "没有可扫描的股票（自选与本地股票库为空），观察名单为空。不是买卖指令，未进入模拟交易计划。"
	case len(hits) == 0:
		view.Message = fmt.Sprintf("观察名单为空：已检查 %d 只，K线不足、价格或成交量无效的标的已跳过。不是买卖指令，未进入模拟交易计划。", scanned)
	default:
		view.Message = "success"
		view.TraceInfo = fmt.Sprintf("%s 已检查 %d 只，命中 %d 只。观察名单不是交易指令，不会自动下单。", def.Blurb, scanned, len(hits))
	}
	a.saveRun(s, view)
	return view
}

func loadObservationSymbols(limit int) []observationSymbol {
	symbols := loadObservationUniverse(limit)
	if len(symbols) == 0 {
		return nil
	}
	return fillObservationBars(symbols)
}

func defaultLoadObservationUniverse(limit int) []observationSymbol {
	if limit <= 0 || limit > observationScanCap {
		limit = observationScanCap
	}
	if db.Dao == nil {
		return nil
	}
	out := make([]observationSymbol, 0, limit)
	if db.Dao.Migrator().HasTable(&FollowedStock{}) {
		var follows []FollowedStock
		_ = db.Dao.Model(&FollowedStock{}).Order("sort asc, time desc").Limit(limit).Find(&follows).Error
		for _, f := range follows {
			if isObservationST(f.Name) {
				continue
			}
			code := strings.TrimSpace(f.StockCode)
			if code == "" {
				continue
			}
			out = append(out, observationSymbol{Code: code, Name: f.Name})
			if len(out) >= limit {
				return out
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	if db.Dao.Migrator().HasTable(&models.AllStockInfo{}) {
		var rows []models.AllStockInfo
		_ = db.Dao.Order("id asc").Limit(limit).Find(&rows).Error
		for _, row := range rows {
			if isObservationST(row.SECURITYNAMEABBR) {
				continue
			}
			code := strings.TrimSpace(row.SECUCODE)
			if code == "" {
				code = strings.TrimSpace(row.SECURITYCODE)
			}
			if code == "" {
				continue
			}
			out = append(out, observationSymbol{
				Code:     code,
				Name:     row.SECURITYNAMEABBR,
				Industry: row.INDUSTRY,
			})
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func fillObservationBars(symbols []observationSymbol) []observationSymbol {
	needFetch := false
	for _, sym := range symbols {
		if len(sym.Bars) == 0 {
			needFetch = true
			break
		}
	}
	var kapi *EastMoneyKLineApi
	if needFetch {
		kapi = NewEastMoneyKLineApi(nil)
	}
	out := make([]observationSymbol, 0, len(symbols))
	for _, sym := range symbols {
		if len(sym.Bars) == 0 && kapi != nil {
			kl := kapi.GetDayKLine(sym.Code, observationHistoryBars)
			sym.Bars = barsFromKLine(kl)
			if n := len(sym.Bars); n > 0 {
				last := sym.Bars[n-1]
				sym.Price = last.Close
				if n >= 2 && sym.Bars[n-2].Close > 0 {
					sym.ChangeRate = (last.Close - sym.Bars[n-2].Close) / sym.Bars[n-2].Close * 100
				}
			}
		}
		if len(sym.Bars) == 0 {
			continue
		}
		if sym.Price <= 0 {
			sym.Price = sym.Bars[len(sym.Bars)-1].Close
		}
		out = append(out, sym)
	}
	return out
}

func barsFromKLine(kl *[]KLineData) []observationBar {
	if kl == nil {
		return nil
	}
	bars := make([]observationBar, 0, len(*kl))
	for _, k := range *kl {
		c := parseKlineFloat(k.Close)
		if c <= 0 {
			continue
		}
		h := parseKlineFloat(k.High)
		l := parseKlineFloat(k.Low)
		if h <= 0 {
			h = c
		}
		if l <= 0 {
			l = c
		}
		bars = append(bars, observationBar{
			Open:   parseKlineFloat(k.Open),
			Close:  c,
			High:   h,
			Low:    l,
			Volume: parseKlineFloat(k.Volume),
		})
	}
	return bars
}

func evalObservation(strategyID string, bars []observationBar) bool {
	switch strategyID {
	case "ext_ma_pullback":
		return evalMaPullback(bars)
	case "ext_vol_breakout":
		return evalVolBreakout(bars)
	case "ext_dd_bounce":
		return evalDdBounce(bars)
	default:
		return false
	}
}

func evalMaPullback(bars []observationBar) bool {
	n := len(bars)
	// 近 3 日里最早一根也要能算出 MA20。
	if n < observationMaPeriod+2 {
		return false
	}
	maNow := smaClose(bars, n-1, observationMaPeriod)
	maPrev := smaClose(bars, n-2, observationMaPeriod)
	if maNow <= 0 || maPrev <= 0 || bars[n-1].Close <= 0 {
		return false
	}
	if maNow <= maPrev || bars[n-1].Close <= maNow {
		return false
	}
	for i := n - 3; i < n; i++ {
		ma := smaClose(bars, i, observationMaPeriod)
		low := bars[i].Low
		if ma <= 0 || low <= 0 || bars[i].Close <= 0 {
			return false
		}
		dist := (low - ma) / ma
		if dist < -observationMaTouchPct || dist > observationMaTouchPct {
			continue
		}
		for j := i; j < n; j++ {
			if isYang(bars, j) {
				return true
			}
		}
	}
	return false
}

func isYang(bars []observationBar, i int) bool {
	if i < 0 || i >= len(bars) || bars[i].Close <= 0 {
		return false
	}
	if bars[i].Open > 0 {
		return bars[i].Close > bars[i].Open
	}
	if i == 0 || bars[i-1].Close <= 0 {
		return false
	}
	return bars[i].Close > bars[i-1].Close
}

func evalVolBreakout(bars []observationBar) bool {
	n := len(bars)
	if n < observationVolLookback+1 {
		return false
	}
	last := bars[n-1]
	if last.Close <= 0 || last.Volume <= 0 {
		return false
	}
	priorHigh := 0.0
	volSum := 0.0
	for i := n - 1 - observationVolLookback; i < n-1; i++ {
		if bars[i].High <= 0 || bars[i].Volume <= 0 {
			return false
		}
		if bars[i].High > priorHigh {
			priorHigh = bars[i].High
		}
		volSum += bars[i].Volume
	}
	if priorHigh <= 0 {
		return false
	}
	avg := volSum / float64(observationVolLookback)
	return last.Close > priorHigh && last.Volume >= avg*observationVolMult
}

func evalDdBounce(bars []observationBar) bool {
	n := len(bars)
	if n < observationNewLowBars || n < observationDdLookback+1 {
		return false
	}
	if overlapsIceNewLow(bars) {
		return false
	}
	rsi, ok := rsiAt(bars, observationRSIPeriod)
	if !ok || rsi < observationRSIMin {
		return false
	}
	peakStart := n - 1 - observationDdLookback
	peak := 0.0
	peakIdx := -1
	for i := peakStart; i < n-1; i++ {
		if bars[i].High <= 0 || bars[i].Low <= 0 || bars[i].Close <= 0 {
			return false
		}
		if bars[i].High > peak {
			peak = bars[i].High
			peakIdx = i
		}
	}
	if peak <= 0 || peakIdx < 0 {
		return false
	}
	trough := 0.0
	for i := peakIdx; i < n-1; i++ {
		if bars[i].Low <= 0 {
			return false
		}
		if trough == 0 || bars[i].Low < trough {
			trough = bars[i].Low
		}
	}
	if trough <= 0 {
		return false
	}
	dd := (peak - trough) / peak
	if dd < observationDdMin || dd > observationDdMax {
		return false
	}
	last := bars[n-1].Close
	if last <= trough || last >= peak || !isYang(bars, n-1) {
		return false
	}
	return true
}

// overlapsIceNewLow 近 20 日低点就是 60 日最低，与冰点式新低重叠，观察回撤不收。
func overlapsIceNewLow(bars []observationBar) bool {
	n := len(bars)
	if n < observationNewLowBars {
		return true
	}
	start := n - observationNewLowBars
	minLow := bars[start].Low
	minIdx := start
	for i := start + 1; i < n; i++ {
		if bars[i].Low <= 0 {
			return true
		}
		if bars[i].Low <= minLow {
			minLow = bars[i].Low
			minIdx = i
		}
	}
	return minIdx >= n-observationDdLookback
}

func rsiAt(bars []observationBar, period int) (float64, bool) {
	closes := make([]float64, len(bars))
	for i, bar := range bars {
		if bar.Close <= 0 {
			return 0, false
		}
		closes[i] = bar.Close
	}
	return rsiSMA(closes, period)
}

func smaClose(bars []observationBar, end, period int) float64 {
	if end < period-1 || end >= len(bars) {
		return 0
	}
	sum := 0.0
	for i := end - period + 1; i <= end; i++ {
		if bars[i].Close <= 0 {
			return 0
		}
		sum += bars[i].Close
	}
	return sum / float64(period)
}

// rsiSMA 与前端 calcRSI 相同：最近 period 根涨跌的简单平均。
func rsiSMA(closes []float64, period int) (float64, bool) {
	if period <= 0 || len(closes) <= period {
		return 0, false
	}
	i := len(closes) - 1
	gain := 0.0
	loss := 0.0
	for j := 0; j < period; j++ {
		ch := closes[i-j] - closes[i-j-1]
		if ch >= 0 {
			gain += ch
		} else {
			loss -= ch
		}
	}
	ag := gain / float64(period)
	al := loss / float64(period)
	if al == 0 {
		return 100, true
	}
	return 100 - 100/(1+ag/al), true
}

func observationHitRow(sym observationSymbol) map[string]any {
	secu, shortCode := observationDisplayCodes(sym.Code)
	price := sym.Price
	if price <= 0 && len(sym.Bars) > 0 {
		price = sym.Bars[len(sym.Bars)-1].Close
	}
	return map[string]any{
		"SECUCODE":           secu,
		"SECURITY_CODE":      shortCode,
		"SECURITY_NAME_ABBR": sym.Name,
		"NEW_PRICE":          price,
		"CHANGE_RATE":        sym.ChangeRate,
		"INDUSTRY":           sym.Industry,
	}
}

func observationDisplayCodes(code string) (secu string, shortCode string) {
	c := strings.TrimSpace(code)
	upper := strings.ToUpper(c)
	if strings.Contains(upper, ".") {
		parts := strings.Split(upper, ".")
		if len(parts) == 2 && len(parts[0]) == 6 {
			return upper, parts[0]
		}
		if len(parts) == 2 && len(parts[1]) == 6 {
			return parts[1] + "." + parts[0], parts[1]
		}
	}
	lower := strings.ToLower(c)
	for _, prefix := range []string{"sh", "sz", "bj"} {
		if strings.HasPrefix(lower, prefix) && len(lower) >= 8 {
			num := lower[2:]
			suf := strings.ToUpper(prefix)
			if suf == "SH" {
				return num + ".SH", num
			}
			if suf == "SZ" {
				return num + ".SZ", num
			}
			return num + ".BJ", num
		}
	}
	if len(c) == 6 {
		suf := ".SZ"
		switch c[0] {
		case '6':
			suf = ".SH"
		case '4', '8', '9':
			suf = ".BJ"
		}
		return c + suf, c
	}
	return upper, c
}

func isObservationST(name string) bool {
	u := strings.ToUpper(strings.TrimSpace(name))
	return strings.HasPrefix(u, "*ST") || strings.HasPrefix(u, "ST") || strings.HasPrefix(u, "S*ST")
}

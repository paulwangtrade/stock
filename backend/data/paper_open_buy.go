package data

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

const paperOpenBuyConfigFile = "paper_open_buy.json"

// PaperOpenBuyConfig 开盘执行开关与默认金额（选股已迁出 strategy）。
type PaperOpenBuyConfig struct {
	EnablePaperOpenBuy     bool     `json:"enablePaperOpenBuy"`
	PaperOpenBuyCodes      []string `json:"paperOpenBuyCodes"` // 仅兼容兜底，正式路径读 TradePlan
	OpenBuyAmountPerStock  float64  `json:"openBuyAmountPerStock"`
	AllowWhitelistFallback bool     `json:"allowWhitelistFallback"` // Phase1 默认 false

	// Phase1.2 计划风控（TradePlan 前置；关闭后与 Phase1 TopN 一致）
	EnableRiskFilter         bool    `json:"enableRiskFilter"`
	PlanMarketLevel          int     `json:"planMarketLevel"` // 1–5；0 表示默认 3
	BlockNewEntriesOnDefense bool    `json:"blockNewEntriesOnDefense"`
	MaxGrossExposurePct      float64 `json:"maxGrossExposurePct"`
	MaxSingleNamePct         float64 `json:"maxSingleNamePct"`
	MaxDailyLossPct          float64 `json:"maxDailyLossPct"`
	CurrentDailyPnlPct       float64 `json:"currentDailyPnlPct"` // 可由外部写入；默认 0
}

// PaperOpenBuyResult 单次执行汇总。
type PaperOpenBuyResult struct {
	Enabled bool                     `json:"enabled"`
	PlanID  uint                     `json:"planId,omitempty"`
	Codes   []string                 `json:"codes"`
	Items   []PaperOpenBuyItemResult `json:"items"`
	Message string                   `json:"message"`
}

// PaperOpenBuyItemResult 单票结果。
type PaperOpenBuyItemResult struct {
	StockCode string  `json:"stockCode"`
	StockName string  `json:"stockName"`
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	OK        bool    `json:"ok"`
	Error     string  `json:"error,omitempty"`
	OrderID   uint    `json:"orderId,omitempty"`
	Status    string  `json:"status,omitempty"`
}

var (
	paperOpenBuyMu     sync.RWMutex
	paperOpenBuyCached *PaperOpenBuyConfig
)

func defaultPaperOpenBuyConfig() PaperOpenBuyConfig {
	return PaperOpenBuyConfig{
		EnablePaperOpenBuy: false,
		PaperOpenBuyCodes: []string{
			"sz000001",
			"sh600519",
		},
		OpenBuyAmountPerStock:    100_000,
		AllowWhitelistFallback:   false,
		EnableRiskFilter:         true,
		PlanMarketLevel:          3,
		BlockNewEntriesOnDefense: true,
		MaxGrossExposurePct:      0.85,
		MaxSingleNamePct:         0.20,
		MaxDailyLossPct:          0,
		CurrentDailyPnlPct:       0,
	}
}

func paperOpenBuyConfigPath() string {
	return filepath.Join("data", paperOpenBuyConfigFile)
}

func jsonHasKey(raw []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// PaperOpenBuyConfigFileExists 仅供启动自检日志。
func PaperOpenBuyConfigFileExists() bool {
	_, err := os.Stat(paperOpenBuyConfigPath())
	return err == nil
}

// GetPaperOpenBuyConfig 读取配置（文件 → 默认）。
func GetPaperOpenBuyConfig() PaperOpenBuyConfig {
	paperOpenBuyMu.RLock()
	if paperOpenBuyCached != nil {
		cfg := *paperOpenBuyCached
		paperOpenBuyMu.RUnlock()
		return cfg
	}
	paperOpenBuyMu.RUnlock()

	cfg := defaultPaperOpenBuyConfig()
	path := paperOpenBuyConfigPath()
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}
	if cfg.OpenBuyAmountPerStock <= 0 {
		cfg.OpenBuyAmountPerStock = 100_000
	}
	if len(cfg.PaperOpenBuyCodes) == 0 {
		cfg.PaperOpenBuyCodes = defaultPaperOpenBuyConfig().PaperOpenBuyCodes
	}
	def := defaultPaperOpenBuyConfig()
	// 旧配置文件无 Phase1.2 字段时：保持 EnableRiskFilter 默认 true（零值 false 需显式写 false）
	if raw != nil && !jsonHasKey(raw, "enableRiskFilter") {
		cfg.EnableRiskFilter = def.EnableRiskFilter
	}
	if cfg.PlanMarketLevel <= 0 {
		cfg.PlanMarketLevel = def.PlanMarketLevel
	}
	if cfg.MaxGrossExposurePct <= 0 {
		cfg.MaxGrossExposurePct = def.MaxGrossExposurePct
	}
	if cfg.MaxSingleNamePct <= 0 {
		cfg.MaxSingleNamePct = def.MaxSingleNamePct
	}
	if raw != nil && !jsonHasKey(raw, "blockNewEntriesOnDefense") {
		cfg.BlockNewEntriesOnDefense = def.BlockNewEntriesOnDefense
	}
	paperOpenBuyMu.Lock()
	paperOpenBuyCached = &cfg
	paperOpenBuyMu.Unlock()
	return cfg
}

// SavePaperOpenBuyConfig 保存配置到 data/paper_open_buy.json。
func SavePaperOpenBuyConfig(cfg PaperOpenBuyConfig) error {
	if cfg.OpenBuyAmountPerStock <= 0 {
		cfg.OpenBuyAmountPerStock = 100_000
	}
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(paperOpenBuyConfigPath(), raw, 0o644); err != nil {
		return err
	}
	paperOpenBuyMu.Lock()
	paperOpenBuyCached = &cfg
	paperOpenBuyMu.Unlock()
	return nil
}

// IsAShareSinaCode 仅允许 sz/sh + 6 位数字。
func IsAShareSinaCode(code string) bool {
	c := strings.ToLower(strings.TrimSpace(code))
	if len(c) != 8 {
		return false
	}
	if !strings.HasPrefix(c, "sz") && !strings.HasPrefix(c, "sh") {
		return false
	}
	for _, r := range c[2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseStockInfoPrice(info StockInfo) (float64, error) {
	p, err := strconv.ParseFloat(strings.TrimSpace(info.Price), 64)
	if err != nil || p <= 0 {
		return 0, fmt.Errorf("invalid price %q", info.Price)
	}
	return p, nil
}

func calcOpenBuyVolume(amount, price float64) int64 {
	if amount <= 0 || price <= 0 {
		return 0
	}
	shares := math.Floor(amount / price / 100) * 100
	return int64(shares)
}

func todayTradeDateLocal() string {
	return time.Now().Format("2006-01-02")
}

// openBuyQuoteFetcher 开盘行情拉取（可测注入；默认走 StockDataApi）。
type openBuyQuoteFetcher func(codes ...string) (*[]StockInfo, error)

var (
	openBuyQuoteMu   sync.RWMutex
	openBuyQuoteFetch openBuyQuoteFetcher = defaultOpenBuyQuoteFetch
)

func defaultOpenBuyQuoteFetch(codes ...string) (*[]StockInfo, error) {
	return NewStockDataApi().GetStockCodeRealTimeData(codes...)
}

func fetchOpenBuyQuotes(codes ...string) (*[]StockInfo, error) {
	openBuyQuoteMu.RLock()
	fn := openBuyQuoteFetch
	openBuyQuoteMu.RUnlock()
	if fn == nil {
		fn = defaultOpenBuyQuoteFetch
	}
	return fn(codes...)
}

// SetOpenBuyQuoteFetcherForTest 仅测试注入行情，避免外网依赖。
func SetOpenBuyQuoteFetcherForTest(fn openBuyQuoteFetcher) {
	openBuyQuoteMu.Lock()
	if fn == nil {
		openBuyQuoteFetch = defaultOpenBuyQuoteFetch
	} else {
		openBuyQuoteFetch = fn
	}
	openBuyQuoteMu.Unlock()
}

func buildPaperOrderReason(item models.TradePlanItem) string {
	name := strings.TrimSpace(item.StrategyName)
	ver := strings.TrimSpace(item.StrategyVersion)
	reason := strings.TrimSpace(item.Reason)
	if name == "" {
		name = "unknown"
	}
	if ver == "" {
		ver = "-"
	}
	if reason == "" {
		reason = "trade_plan"
	}
	return fmt.Sprintf("%s:%s:%s", name, ver, reason)
}

func lookupQuote(byCode map[string]StockInfo, code string) (StockInfo, bool) {
	if info, ok := byCode[code]; ok {
		return info, true
	}
	for k, v := range byCode {
		if strings.EqualFold(k, code) || (len(code) >= 2 && strings.HasSuffix(k, code[2:])) {
			return v, true
		}
	}
	return StockInfo{}, false
}

func findFillIDByOrder(orderID uint) uint {
	if db.Dao == nil || orderID == 0 {
		return 0
	}
	var fill PaperFill
	if err := db.Dao.Where("order_id = ?", orderID).Order("id DESC").First(&fill).Error; err != nil {
		return 0
	}
	return fill.ID
}

// RunPaperOpenPrepare Execution Adapter：检查当日 ready TradePlan，不下单。
func RunPaperOpenPrepare() PaperOpenBuyResult {
	cfg := GetPaperOpenBuyConfig()
	tradeDate := todayTradeDateLocal()
	repo := NewTradePlanRepo()
	plan, err := repo.GetReadyByTradeDate(tradeDate)
	res := PaperOpenBuyResult{Enabled: cfg.EnablePaperOpenBuy}
	if err != nil {
		res.Message = fmt.Sprintf("prepare: no ready TradePlan for %s (%v)", tradeDate, err)
		logger.SugaredLogger.Warnf("paper open buy PREPARE %s", res.Message)
		return res
	}
	res.PlanID = plan.ID
	// Phase6-A6.2: only Frozen ready may be MarkChecked / treated as prepare-ok.
	if guard := models.RequireFrozenReadyTradePlan(plan); !guard.Allowed {
		res.Message = fmt.Sprintf("prepare blocked: reason=%s planId=%d %s",
			guard.Reason, plan.ID, guard.Message)
		logger.SugaredLogger.Warnf("paper open buy PREPARE blocked reason=%s planId=%d",
			guard.Reason, plan.ID)
		return res
	}
	codes := make([]string, 0, len(plan.Items))
	for _, it := range plan.Items {
		codes = append(codes, it.StockCode)
		res.Items = append(res.Items, PaperOpenBuyItemResult{
			StockCode: it.StockCode,
			StockName: it.StockName,
			OK:        true,
			Status:    it.Status,
		})
		logger.SugaredLogger.Infof("paper open buy candidate: %s %s strategy=%s/%s",
			it.StockCode, it.StockName, it.StrategyName, it.StrategyVersion)
	}
	res.Codes = codes
	_ = repo.MarkChecked(plan.ID)
	res.Message = fmt.Sprintf("prepare: planId=%d enabled=%v items=%d amount=%.0f",
		plan.ID, cfg.EnablePaperOpenBuy, len(codes), plan.AmountPerStock)
	logger.SugaredLogger.Infof("paper open buy PREPARE %s", res.Message)
	return res
}

// RunPaperOpenBuyOnce Execution Adapter：CAS ready→executing → 行情 → 手数 → PlanItemExecutor。
// 下单唯一入口：ExecutionService → ExecutionPort → PaperBroker → SubmitPaperOrder（由 execution 包注入）。
// 禁止选股/排序/TopN。requireEnabled=true（cron）需开关开启。
func RunPaperOpenBuyOnce(requireEnabled bool) PaperOpenBuyResult {
	cfg := GetPaperOpenBuyConfig()
	res := PaperOpenBuyResult{Enabled: cfg.EnablePaperOpenBuy}
	if requireEnabled && !cfg.EnablePaperOpenBuy {
		res.Message = "EnablePaperOpenBuy=false, skip buy"
		logger.SugaredLogger.Infof("paper open buy SKIP: %s", res.Message)
		return res
	}

	tradeDate := todayTradeDateLocal()
	repo := NewTradePlanRepo()
	plan, err := repo.GetReadyByTradeDate(tradeDate)
	if err != nil {
		res.Message = fmt.Sprintf("no ready TradePlan for %s", tradeDate)
		logger.SugaredLogger.Warnf("paper open buy: %s", res.Message)
		return res
	}
	res.PlanID = plan.ID
	// Phase6-A6.2: only Frozen ready may CAS into executing.
	if guard := models.RequireFrozenReadyTradePlan(plan); !guard.Allowed {
		res.Message = fmt.Sprintf("skip buy: reason=%s planId=%d %s",
			guard.Reason, plan.ID, guard.Message)
		logger.SugaredLogger.Warnf("paper open buy SKIP reason=%s planId=%d",
			guard.Reason, plan.ID)
		return res
	}

	okCAS, cerr := repo.TryBeginExecute(plan.ID)
	if cerr != nil {
		res.Message = "CAS begin execute failed: " + cerr.Error()
		logger.SugaredLogger.Errorf("paper open buy: %s", res.Message)
		return res
	}
	if !okCAS {
		res.Message = fmt.Sprintf("planId=%d not ready (already executing/done)", plan.ID)
		logger.SugaredLogger.Warnf("paper open buy SKIP: %s", res.Message)
		return res
	}

	return runPaperOpenBuyExecuting(repo, plan, cfg, res)
}

func finishExecutingPlan(repo *TradePlanRepo, planID uint, toStatus, message string) {
	ok, err := repo.FinishPlanCAS(planID, models.TradePlanStatusExecuting, toStatus, message)
	if err != nil {
		logger.SugaredLogger.Errorf("paper open buy finish CAS plan_id=%d: %v", planID, err)
		return
	}
	if !ok {
		logger.SugaredLogger.Warnf("paper open buy finish CAS miss plan_id=%d to=%s", planID, toStatus)
	}
}

func runPaperOpenBuyExecuting(repo *TradePlanRepo, plan *models.TradePlan, cfg PaperOpenBuyConfig, res PaperOpenBuyResult) (out PaperOpenBuyResult) {
	out = res
	tradeDate := plan.TradeDate
	if tradeDate == "" {
		tradeDate = todayTradeDateLocal()
	}

	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("paper open buy panic plan_id=%d: %v", plan.ID, r)
			rec, rerr := ReconcileTradePlan(plan.ID)
			if rerr != nil {
				logger.SugaredLogger.Errorf("paper open buy panic reconcile failed plan_id=%d: %v", plan.ID, rerr)
				out.Message = fmt.Sprintf("panic recovered: %v (reconcile err: %v)", r, rerr)
				return
			}
			if rec.Applied {
				logger.SugaredLogger.Infof("paper open buy panic reconciled plan_id=%d status=%s", plan.ID, rec.TerminalStatus)
			}
			out.Message = fmt.Sprintf("panic recovered: %v; reconcile=%s", r, rec.TerminalStatus)
		}
	}()

	logger.SugaredLogger.Infof("Paper Open Buy Execution Start plan_id=%d tradeDate=%s items=%d",
		plan.ID, tradeDate, len(plan.Items))

	var buyCodes []string
	var skipLines []string
	for _, it := range plan.Items {
		if it.Status == "" || it.Status == models.TradePlanItemPending {
			buyCodes = append(buyCodes, it.StockCode)
			continue
		}
		reason := it.RiskCode
		if reason == "" {
			reason = it.Status
		}
		skipLines = append(skipLines, fmt.Sprintf("%s reason=%s", it.StockCode, reason))
	}
	logger.SugaredLogger.Infof("trade execution: planId=%d", plan.ID)
	if len(buyCodes) > 0 {
		logger.SugaredLogger.Infof("buy: %s", strings.Join(buyCodes, " "))
	} else {
		logger.SugaredLogger.Infof("buy: (none)")
	}
	for _, line := range skipLines {
		logger.SugaredLogger.Infof("skip: %s", line)
	}

	codes := buyCodes
	out.Codes = codes
	if len(codes) == 0 {
		finishExecutingPlan(repo, plan.ID, models.TradePlanStatusSkipped, "no pending plan items after risk filter")
		out.Message = "no pending plan items"
		logger.SugaredLogger.Infof("paper open buy SKIP: planId=%d no pending items", plan.ID)
		return out
	}

	amount := plan.AmountPerStock
	if amount <= 0 {
		amount = cfg.OpenBuyAmountPerStock
	}

	quotes, qerr := fetchOpenBuyQuotes(codes...)
	if qerr != nil {
		finishExecutingPlan(repo, plan.ID, models.TradePlanStatusFailed, "quote failed: "+qerr.Error())
		out.Message = "GetStockCodeRealTimeData failed: " + qerr.Error()
		logger.SugaredLogger.Errorf("paper open buy: %s", out.Message)
		return out
	}
	byCode := map[string]StockInfo{}
	if quotes != nil {
		for _, q := range *quotes {
			byCode[strings.ToLower(strings.TrimSpace(q.Code))] = q
		}
	}

	executor := getPlanItemExecutor()
	if executor == nil {
		msg := "plan item executor not configured (import go-stock/backend/execution); refuse silent SubmitPaperOrder fallback"
		finishExecutingPlan(repo, plan.ID, models.TradePlanStatusFailed, msg)
		out.Message = msg
		logger.SugaredLogger.Errorf("paper open buy: %s", out.Message)
		return out
	}

	okCount := 0
	for i := range plan.Items {
		planItem := &plan.Items[i]
		if planItem.Status != "" && planItem.Status != models.TradePlanItemPending {
			continue
		}
		item := PaperOpenBuyItemResult{StockCode: planItem.StockCode, StockName: planItem.StockName}

		info, ok := lookupQuote(byCode, planItem.StockCode)
		if !ok {
			item.Error = "no realtime quote"
			planItem.Status = models.TradePlanItemError
			planItem.Error = item.Error
			_ = repo.UpdateItemExecution(planItem)
			out.Items = append(out.Items, item)
			logger.SugaredLogger.Warnf("Paper Order Failed stock=%s reason=%s", planItem.StockCode, item.Error)
			continue
		}
		item.StockName = info.Name
		planItem.StockName = info.Name
		price, perr := parseStockInfoPrice(info)
		if perr != nil {
			item.Error = perr.Error()
			planItem.Status = models.TradePlanItemError
			planItem.Error = item.Error
			_ = repo.UpdateItemExecution(planItem)
			out.Items = append(out.Items, item)
			logger.SugaredLogger.Warnf("Paper Order Failed stock=%s reason=%s", planItem.StockCode, item.Error)
			continue
		}
		item.Price = price
		targetAmount := planItem.TargetAmount
		if targetAmount <= 0 {
			targetAmount = amount
		}
		vol := calcOpenBuyVolume(targetAmount, price)
		item.Volume = vol
		planItem.TargetVolume = vol

		logger.SugaredLogger.Infof("stock=%s strategy=%s reason=%s price=%.4f volume=%d",
			planItem.StockCode, planItem.StrategyName, buildPaperOrderReason(*planItem), price, vol)

		if vol < 100 {
			item.Error = fmt.Sprintf("volume < 100 (amount=%.0f price=%.4f)", targetAmount, price)
			planItem.Status = models.TradePlanItemSkipped
			planItem.Error = item.Error
			_ = repo.UpdateItemExecution(planItem)
			out.Items = append(out.Items, item)
			logger.SugaredLogger.Warnf("Paper Order Failed stock=%s reason=%s", planItem.StockCode, item.Error)
			continue
		}

		order, serr := executor.ExecutePlanItem(*planItem, PlanItemExecOpts{
			StockName:   info.Name,
			Price:       price,
			Volume:      vol,
			Reason:      buildPaperOrderReason(*planItem),
			StrategyTag: models.PaperStrategyTagTradePlan,
			AutoFill:    true,
		})
		if serr != nil {
			item.Error = serr.Error()
			planItem.Status = models.TradePlanItemError
			planItem.Error = item.Error
			if order != nil {
				item.OrderID = order.ID
				item.Status = order.Status
				planItem.OrderID = order.ID
			}
			_ = repo.UpdateItemExecution(planItem)
			out.Items = append(out.Items, item)
			logger.SugaredLogger.Errorf("Paper Order Failed stock=%s reason=%s", planItem.StockCode, item.Error)
			continue
		}

		item.OK = true
		item.OrderID = order.ID
		item.Status = order.Status
		planItem.Status = models.TradePlanItemFilled
		planItem.OrderID = order.ID
		planItem.FillID = findFillIDByOrder(order.ID)
		planItem.FilledPrice = order.FilledPrice
		if planItem.FilledPrice <= 0 {
			planItem.FilledPrice = price
		}
		planItem.FilledVolume = order.FilledVol
		if planItem.FilledVolume <= 0 {
			planItem.FilledVolume = vol
		}
		planItem.FilledFee = order.Fee
		planItem.Error = ""
		_ = repo.UpdateItemExecution(planItem)
		okCount++
		logger.SugaredLogger.Infof("Paper Order Filled stock=%s order_id=%d status=%s",
			planItem.StockCode, order.ID, order.Status)
		out.Items = append(out.Items, item)
	}

	pendingTotal := len(codes)
	finalStatus := models.TradePlanStatusDone
	if okCount == 0 {
		finalStatus = models.TradePlanStatusFailed
	} else if okCount < pendingTotal {
		finalStatus = models.TradePlanStatusPartial
	}
	msg := fmt.Sprintf("done ok=%d/%d planId=%d", okCount, pendingTotal, plan.ID)
	finishExecutingPlan(repo, plan.ID, finalStatus, msg)
	out.Message = msg
	logger.SugaredLogger.Infof("Paper Open Buy Execution End plan_id=%d status=%s %s", plan.ID, finalStatus, msg)
	return out
}

// GetTodayTradePlan 调试/查询当日最新计划（含 items）。
func GetTodayTradePlan() (*models.TradePlan, error) {
	return NewTradePlanRepo().GetLatestByTradeDate(todayTradeDateLocal())
}

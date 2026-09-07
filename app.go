package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-stock/backend/agent"
	"go-stock/backend/agent/tools"
	"go-stock/backend/broker"
	"go-stock/backend/cache"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/marketdata"
	"go-stock/backend/models"
	"go-stock/backend/security"
	_ "go-stock/backend/tradingrule" // QuantityPolicy Submit hooks (legacy paper)
	"go-stock/backend/version"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/PuerkitoBio/goquery"
	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx                context.Context
	cache              *freecache.Cache
	cron               *cron.Cron
	cronEntrys         map[string]cron.EntryID
	cronEntrysMu       sync.Mutex
	AiTools            []data.Tool
	SponsorInfo        map[string]any
	VipLevel           int64
	summaryMu          sync.Mutex
	summaryCancel      context.CancelFunc
	agentMu            sync.Mutex
	agentCancel        context.CancelFunc
	stockAlertMu       sync.Mutex
	stockAlertLastSent map[string]time.Time
	priceAtAlertReset  map[string]float64
	tradingEventMu     sync.Mutex
	tradingEventCancel context.CancelFunc
	tradingUnsubscribe func()
	tradingEventWG     sync.WaitGroup
	// settingsReloadPrev：UpdateConfig 写入前的关键字段快照，供 updateSettings 决定是否 WindowReloadApp
	settingsReloadMu   sync.Mutex
	settingsReloadPrev *settingsReloadFingerprint
	// marketData：App 实例级行情门面（MD-001 M1）；禁止业务直连 EastMoney/Tencent Provider。
	marketData marketdata.MarketDataService
}

// settingsReloadFingerprint 仅包含「必须整页重载」的关键配置字段。
// RefreshInterval 已由 UpdateConfig 热更新 cron；DarkTheme 由原生主题 API 即时生效。
type settingsReloadFingerprint struct {
	HttpProxy        string
	HttpProxyEnabled bool
	BrowserPath      string
	EnableAgent      bool
	EnableNews       bool
	EnableFund       bool
	SponsorCode      string
}

func fingerprintSettings(cfg *data.SettingConfig) *settingsReloadFingerprint {
	if cfg == nil || cfg.Settings == nil {
		return &settingsReloadFingerprint{}
	}
	return &settingsReloadFingerprint{
		HttpProxy:        cfg.HttpProxy,
		HttpProxyEnabled: cfg.HttpProxyEnabled,
		BrowserPath:      cfg.BrowserPath,
		EnableAgent:      cfg.EnableAgent,
		EnableNews:       cfg.EnableNews,
		EnableFund:       cfg.EnableFund,
		SponsorCode:      cfg.SponsorCode,
	}
}

func settingsFingerprintChanged(prev, next *settingsReloadFingerprint) bool {
	if prev == nil || next == nil {
		return prev != next
	}
	return *prev != *next
}

// NewApp creates a new App application struct
func NewApp() *App {
	cacheSize := 512 * 1024
	cache := freecache.NewCache(cacheSize)
	c := cron.New(cron.WithSeconds())
	c.Start()
	var tools []data.Tool
	tools = data.Tools(tools)
	return &App{
		cache:              cache,
		cron:               c,
		cronEntrys:         make(map[string]cron.EntryID),
		AiTools:            tools,
		stockAlertLastSent: make(map[string]time.Time),
		priceAtAlertReset:  make(map[string]float64),
		marketData:         newAppMarketDataService(),
	}
}

func (a *App) setCronEntry(key string, id cron.EntryID) {
	a.cronEntrysMu.Lock()
	a.cronEntrys[key] = id
	a.cronEntrysMu.Unlock()
}

func (a *App) getCronEntry(key string) (cron.EntryID, bool) {
	a.cronEntrysMu.Lock()
	id, exists := a.cronEntrys[key]
	a.cronEntrysMu.Unlock()
	return id, exists
}

func (a *App) removeCronEntry(key string) {
	a.cronEntrysMu.Lock()
	delete(a.cronEntrys, key)
	a.cronEntrysMu.Unlock()
}

func (a *App) startTradingEventBridge(ctx context.Context) {
	a.stopTradingEventBridge()
	events, unsubscribe := broker.DefaultHub.Subscribe(256)
	eventCtx, cancel := context.WithCancel(ctx)

	a.tradingEventMu.Lock()
	a.tradingEventCancel = cancel
	a.tradingUnsubscribe = unsubscribe
	a.tradingEventWG.Add(1)
	a.tradingEventMu.Unlock()

	go func() {
		defer a.tradingEventWG.Done()
		for {
			select {
			case <-eventCtx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				runtime.EventsEmit(ctx, "tradingEvent", event)
			}
		}
	}()
}

func (a *App) stopTradingEventBridge() {
	a.tradingEventMu.Lock()
	cancel := a.tradingEventCancel
	unsubscribe := a.tradingUnsubscribe
	a.tradingEventCancel = nil
	a.tradingUnsubscribe = nil
	a.tradingEventMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if unsubscribe != nil {
		unsubscribe()
	}
	a.tradingEventWG.Wait()
}

func (a *App) GetSponsorInfo() map[string]any {
	if data.BypassSponsorVip {
		return data.BypassSponsorInfo()
	}
	return a.SponsorInfo
}

// GetEffectiveSponsorVip 从本地配置解密授权信息并判断当前是否在 Pro 有效期内（与 ai-assistant-web / data.EffectiveSponsorVipLevel 一致）。
func (a *App) GetEffectiveSponsorVip() map[string]any {
	level, active := data.EffectiveSponsorVipLevel()
	return map[string]any{
		"vipLevel": level,
		"active":   active,
	}
}
func (a *App) CheckSponsorCode(sponsorCode string) map[string]any {
	sponsorCode = strutil.Trim(sponsorCode)
	if sponsorCode != "" {
		encrypted, err := hex.DecodeString(sponsorCode)
		if err != nil {
			return map[string]any{
				"code": 0,
				"msg":  "授权码格式错误，请输入正确的授权码",
			}
		}
		key, err := hex.DecodeString(BuildKey)
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return map[string]any{
				"code": 0,
				"msg":  "当前版本不支持授权码校验",
			}
		}
		decrypt := cryptor.AesEcbDecrypt(encrypted, key)
		if decrypt == nil || len(decrypt) == 0 {
			return map[string]any{
				"code": 0,
				"msg":  "授权码无效，请检查后重试",
			}
		}

		config := data.GetSettingConfig()
		if config.SponsorCode != sponsorCode {
			config.SponsorCode = sponsorCode
			data.UpdateConfig(config)
		}

		return map[string]any{
			"code": 1,
			"msg":  "授权码校验成功",
		}
	}
	return map[string]any{"code": 0, "message": "授权码不能为空"}
}

func (a *App) syncNews() {
	defer PanicHandler()
	// Phase 1B: legacy remote news sync disabled; local crawlers remain unchanged.
	logger.SugaredLogger.Debug("syncNews: remote operator endpoint sync disabled")
}

func GetSource(tags []string) string {
	if slice.ContainAny(tags, []string{"外媒简讯", "外媒资讯", "外媒"}) {
		return "外媒"
	}
	if slices.Contains(tags, "财联社电报") {
		return "财联社电报"
	}
	if slices.Contains(tags, "新浪财经") {
		return "新浪财经"
	}
	return ""
}


func syncAllStockInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	db.Dao.Unscoped().Model(&models.AllStockInfo{}).Where("1=1").Delete(&models.AllStockInfo{})
	for page := 1; page <= 10; page++ {
		res := data.NewStockDataApi().GetAllStocks(page, 3000, "", "", "", "", models.TechnicalIndicators{})
		if res == nil || len(res.Result.Data) == 0 {
			break
		}
		var datas []models.AllStockInfo
		for _, row := range res.Result.Data {
			datas = append(datas, row.ToAllStockInfo())
		}
		err := db.Dao.CreateInBatches(&datas, 1000).Error
		if err != nil {
			logger.SugaredLogger.Errorf("db.Dao.CreateInBatches error:%s", err.Error())
		}
		total := res.Result.Count
		if total > 0 && page*3000 >= total {
			break
		}
		if len(res.Result.Data) < 3000 {
			break
		}
	}
}
func (a *App) CheckStockBaseInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	stockBasics := &[]data.StockBasic{}
	resty.New().R().
		SetHeader("user", "go-stock").
		SetResult(stockBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_basic.json")

	db.Dao.Unscoped().Model(&data.StockBasic{}).Where("1=1").Delete(&data.StockBasic{})
	err := db.Dao.CreateInBatches(stockBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockBasic股票基础信息失败:%s", err.Error())
	}

	//count := int64(0)
	//db.Dao.Model(&data.StockBasic{}).Count(&count)
	//if count == int64(len(*stockBasics)) {
	//	return
	//}
	//for _, stock := range *stockBasics {
	//	stockInfo := &data.StockBasic{
	//		TsCode: stock.TsCode,
	//		Name:   stock.Name,
	//		Symbol: stock.Symbol,
	//		BKCode: stock.BKCode,
	//		BKName: stock.BKName,
	//	}
	//	db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&data.StockBasic{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Updates(stockInfo)
	//	}
	//}

	stockHKBasics := &[]models.StockInfoHK{}
	resty.New().R().
		SetHeader("user", "go-stock").
		SetResult(stockHKBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_base_info_hk.json")

	db.Dao.Unscoped().Model(&models.StockInfoHK{}).Where("1=1").Delete(&models.StockInfoHK{})
	err = db.Dao.CreateInBatches(stockHKBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockInfoHK股票基础信息失败:%s", err.Error())
	}

	//for _, stock := range *stockHKBasics {
	//	stockInfo := &models.StockInfoHK{
	//		Code:   stock.Code,
	//		Name:   stock.Name,
	//		BKName: stock.BKName,
	//		BKCode: stock.BKCode,
	//	}
	//	db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stock.Code).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stock.Code).Updates(stockInfo)
	//	}
	//}
	stockUSBasics := &[]models.StockInfoUS{}
	resty.New().R().
		SetHeader("user", "go-stock").
		SetResult(stockUSBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_base_info_us.json")

	db.Dao.Unscoped().Model(&models.StockInfoUS{}).Where("1=1").Delete(&models.StockInfoUS{})
	err = db.Dao.CreateInBatches(stockUSBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockInfoUS股票基础信息失败:%s", err.Error())
	}
	//for _, stock := range *stockUSBasics {
	//	stockInfo := &models.StockInfoUS{
	//		Code:   stock.Code,
	//		Name:   stock.Name,
	//		BKName: stock.BKName,
	//		BKCode: stock.BKCode,
	//	}
	//	db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stock.Code).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stock.Code).Updates(stockInfo)
	//	}
	//}

}

var newsPushRecent sync.Map

func newsPushDedupKey(t models.Telegraph) string {
	c := strings.TrimSpace(t.Content)
	runes := []rune(c)
	if len(runes) > 100 {
		c = string(runes[:100])
	}
	tk := strings.TrimSpace(t.Time)
	if t.DataTime != nil {
		tk = t.DataTime.Format("15:04:05")
	}
	return strings.TrimSpace(t.Source) + "|" + tk + "|" + c
}

func shouldEmitNewsPush(t models.Telegraph) bool {
	key := newsPushDedupKey(t)
	now := time.Now().Unix()
	if v, ok := newsPushRecent.Load(key); ok {
		if now-v.(int64) < 30*60 {
			return false
		}
	}
	newsPushRecent.Store(key, now)
	return true
}

func (a *App) NewsPush(news *[]models.Telegraph) {

	follows := data.NewStockDataApi().GetFollowList(0)
	stockNames := slice.Map(*follows, func(index int, item data.FollowedStock) string {
		return item.Name
	})

	for _, telegraph := range *news {
		if !shouldEmitNewsPush(telegraph) {
			continue
		}
		if a.GetConfig().EnableOnlyPushRedNews {
			if telegraph.IsRed || strutil.ContainsAny(telegraph.Content, stockNames) {
				go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
			}
		} else {
			go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
		}
		//go data.NewAlertWindowsApi("go-stock", telegraph.Source+" "+telegraph.Time, telegraph.Content, string(icon)).SendNotification()
		//}
	}
}

func (a *App) AddCronTask(follow data.FollowedStock) func() {
	return func() {
		go runtime.EventsEmit(a.ctx, "warnMsg", "开始自动分析"+follow.Name+"_"+follow.StockCode)
		ai := data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId)
		msgs := ai.NewChatStream(follow.Name, follow.StockCode, "", nil, a.AiTools, true)
		var res strings.Builder

		chatId := ""
		question := ""
		for msg := range msgs {
			if msg["extraContent"] != nil {
				res.WriteString(msg["extraContent"].(string) + "\n")
			}
			if msg["content"] != nil {
				res.WriteString(msg["content"].(string))
			}
			if msg["chatId"] != nil {
				chatId = msg["chatId"].(string)
			}
			if msg["question"] != nil {
				question = msg["question"].(string)
			}
		}

		data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId).SaveAIResponseResult(follow.StockCode, follow.Name, res.String(), chatId, question)
		go runtime.EventsEmit(a.ctx, "warnMsg", "AI分析完成："+follow.Name+"_"+follow.StockCode)

	}
}

func refreshTelegraphList() *[]string {
	url := "https://www.cls.cn/telegraph"
	response, err := resty.New().R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	if err != nil {
		return &[]string{}
	}
	//logger.SugaredLogger.Info(string(response.Body()))
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body())))
	if err != nil {
		return &[]string{}
	}
	var telegraph []string
	document.Find("div.telegraph-content-box").Each(func(i int, selection *goquery.Selection) {
		//logger.SugaredLogger.Info(selection.Text())
		telegraph = append(telegraph, selection.Text())
	})
	return &telegraph
}

// isTradingDay 判断是否是交易日
func isTradingDay(date time.Time) bool {
	weekday := date.Weekday()
	// 判断是否是周末
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	// 这里可以添加具体的节假日判断逻辑
	// 例如：判断是否是春节、国庆节等
	return true
}

// isTradingTime 判断是否是交易时间
func isTradingTime(date time.Time) bool {
	if !isTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	// 判断是否在9:15到11:30之间
	if (hour == 9 && minute >= 15) || (hour == 10) || (hour == 11 && minute <= 30) {
		return true
	}

	// 判断是否在13:00到15:00之间
	if (hour == 13) || (hour == 14) || (hour == 15 && minute <= 0) {
		return true
	}

	return false
}

// IsHKTradingTime 判断当前时间是否在港股交易时间内
func IsHKTradingTime(date time.Time) bool {
	hour, minute, _ := date.Clock()

	// 开市前竞价时段：09:00 - 09:30
	if (hour == 9 && minute >= 0) || (hour == 9 && minute <= 30) {
		return true
	}

	// 上午持续交易时段：09:30 - 12:00
	if (hour == 9 && minute > 30) || (hour >= 10 && hour < 12) || (hour == 12 && minute == 0) {
		return true
	}

	// 下午持续交易时段：13:00 - 16:00
	if (hour == 13 && minute >= 0) || (hour >= 14 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}

	// 收市竞价交易时段：16:00 - 16:10
	if (hour == 16 && minute >= 0) || (hour == 16 && minute <= 10) {
		return true
	}
	return false
}

// IsUSTradingTime 判断当前时间是否在美股交易时间内
func IsUSTradingTime(date time.Time) bool {
	// 获取美国东部时区
	est, err := time.LoadLocation("America/New_York")
	var estTime time.Time
	if err != nil {
		estTime = date.Add(time.Hour * -12)
	} else {
		// 将当前时间转换为美国东部时间
		estTime = date.In(est)
	}

	// 判断是否是周末
	weekday := estTime.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// 获取小时和分钟
	hour, minute, _ := estTime.Clock()

	// 判断是否在4:00 AM到9:30 AM之间（盘前）
	if (hour == 4) || (hour == 5) || (hour == 6) || (hour == 7) || (hour == 8) || (hour == 9 && minute < 30) {
		return true
	}

	// 判断是否在9:30 AM到4:00 PM之间（盘中）
	if (hour == 9 && minute >= 30) || (hour >= 10 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}

	// 判断是否在4:00 PM到8:00 PM之间（盘后）
	if (hour == 16 && minute > 0) || (hour >= 17 && hour < 20) || (hour == 20 && minute == 0) {
		return true
	}

	return false
}
func MonitorFundPrices(a *App) {
	// 检查 A 股是否开市（基金交易时间与 A 股一致）
	if !isTradingTime(time.Now()) {
		logger.SugaredLogger.Debugf("当前 A 股未开市，跳过基金价格监控")
		return
	}

	logger.SugaredLogger.Debugf("A 股市场已开市，开始基金价格监控")

	dest := &[]data.FollowedFund{}
	db.Dao.Model(&data.FollowedFund{}).Find(dest)
	for _, follow := range *dest {
		_, err := data.NewFundApi().CrawlFundBasic(follow.Code)
		if err != nil {
			logger.SugaredLogger.Errorf("获取基金基本信息失败，基金代码：%s，错误信息：%s", follow.Code, err.Error())
			continue
		}
		data.NewFundApi().CrawlFundNetEstimatedUnit(follow.Code)
		data.NewFundApi().CrawlFundNetUnitValue(follow.Code)
	}
}

// MonitorAiRecommendStockPrices 监控 AI 推荐股票的价格，当股价达到预警线时发送通知
func MonitorAiRecommendStockPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过 AI 推荐股票价格监控")
		return
	}

	var aiRecommendStocks []models.AiRecommendStocks
	db.Dao.Model(&models.AiRecommendStocks{}).Where("enable_alert = ?", true).Find(&aiRecommendStocks)

	if len(aiRecommendStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockCodeMap := make(map[string]*models.AiRecommendStocks)
	for i := range aiRecommendStocks {
		stock := &aiRecommendStocks[i]
		stopLossPrice, _ := convertor.ToFloat(stock.RecommendStopLossPrice)
		if stock.RecommendBuyPriceMin <= 0 && stock.RecommendStopProfitPriceMin <= 0 && stopLossPrice <= 0 {
			continue
		}
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockCodeMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	if len(stockCodes) == 0 {
		logger.SugaredLogger.Debugf("没有设置预警价格的 AI 推荐股票，跳过价格监控")
		return
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取 AI 推荐股票实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		aiStock, ok := stockCodeMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		baseAlertKey := fmt.Sprintf("%s:%s", aiStock.StockCode, aiStock.DataTime.Format("20060102"))

		buyAlertKey := baseAlertKey + ":BUY"
		if aiStock.RecommendBuyPriceMin > 0 && currentPrice <= aiStock.RecommendBuyPriceMin {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if priceSinceLastBuyAlert == 0 || priceSinceLastBuyAlert > aiStock.RecommendBuyPriceMin {
				title := fmt.Sprintf("【买入预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议买入价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议买入价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax)
				if a.canSendAlert(buyAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(buyAlertKey)
					a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		} else {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if currentPrice > aiStock.RecommendBuyPriceMin && (priceSinceLastBuyAlert == 0 || currentPrice > priceSinceLastBuyAlert) {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		}

		profitAlertKey := baseAlertKey + ":PROFIT"
		if aiStock.RecommendStopProfitPriceMin > 0 && currentPrice >= aiStock.RecommendStopProfitPriceMin {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if priceSinceLastProfitAlert == 0 || priceSinceLastProfitAlert < aiStock.RecommendStopProfitPriceMin {
				title := fmt.Sprintf("【止盈预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止盈价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止盈价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax)
				if a.canSendAlert(profitAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(profitAlertKey)
					a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		} else {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if currentPrice < aiStock.RecommendStopProfitPriceMin && (priceSinceLastProfitAlert == 0 || currentPrice < priceSinceLastProfitAlert) {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		}

		stopLossAlertKey := baseAlertKey + ":LOSS"
		stopLossPrice, _ := convertor.ToFloat(aiStock.RecommendStopLossPrice)
		if stopLossPrice > 0 && currentPrice <= stopLossPrice {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if priceSinceLastLossAlert == 0 || priceSinceLastLossAlert > stopLossPrice {
				title := fmt.Sprintf("【止损预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止损价**: %s\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止损价: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice)
				if a.canSendAlert(stopLossAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(stopLossAlertKey)
					a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		} else {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if currentPrice > stopLossPrice && (priceSinceLastLossAlert == 0 || currentPrice > priceSinceLastLossAlert) {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		}
	}
}

// MonitorFollowedStockCostPrices 监控自选股的持仓成本价，当股价低于成本价时发送预警
func MonitorFollowedStockCostPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过自选股成本价监控")
		return
	}

	var followedStocks []data.FollowedStock
	db.Dao.Model(&data.FollowedStock{}).Where("cost_price > 0").Find(&followedStocks)

	if len(followedStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockMap := make(map[string]*data.FollowedStock)
	for i := range followedStocks {
		stock := &followedStocks[i]
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取自选股实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		followedStock, ok := stockMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		costPrice := followedStock.CostPrice
		if costPrice <= 0 {
			continue
		}

		alertKey := fmt.Sprintf("COST:%s:%s", followedStock.StockCode, followedStock.Time.Format("20060102"))

		if currentPrice < costPrice {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if priceSinceLastAlert == 0 || priceSinceLastAlert >= costPrice {
				dropPercent := ((costPrice - currentPrice) / costPrice) * 100
				title := fmt.Sprintf("【成本价预警】%s", followedStock.Name)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **持仓成本价**: %.2f\n- **亏损比例**: %.2f%%\n- **关注时间**: %s",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent,
					followedStock.Time.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n成本价: %.2f\n亏损: %.2f%%",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent)
				if a.canSendAlert(alertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(alertKey)
					a.updatePriceAtAlertReset(alertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		} else {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if currentPrice >= costPrice && (priceSinceLastAlert == 0 || currentPrice < priceSinceLastAlert) {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		}
	}
}

// canSendAlert 检查是否可以发送预警，避免重复发送
// alertKey: 预警的唯一标识
// interval: 发送间隔
// 返回 true 表示可以发送，false 表示需要在间隔后才能发送
func (a *App) canSendAlert(alertKey string, interval time.Duration) bool {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()

	lastSent, exists := a.stockAlertLastSent[alertKey]
	if !exists {
		return true
	}

	return time.Since(lastSent) >= interval
}

// updateAlertSentTime 更新预警发送时间
func (a *App) updateAlertSentTime(alertKey string) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.stockAlertLastSent[alertKey] = time.Now()
}

// getPriceAtAlertReset 获取预警重置后的价格（用于判断是否需要重新触发预警）
func (a *App) getPriceAtAlertReset(alertKey string) float64 {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	return a.priceAtAlertReset[alertKey]
}

// updatePriceAtAlertReset 更新预警重置后的价格
func (a *App) updatePriceAtAlertReset(alertKey string, price float64) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.priceAtAlertReset[alertKey] = price
}

func GetStockInfos(follows ...data.FollowedStock) *[]data.StockInfo {
	out, err := getStockInfosWithQuoteService(data.GetQuoteService(), true, follows...)
	if err != nil {
		logger.SugaredLogger.Errorf("GetStockInfos QuoteService: %v", err)
	}
	if out == nil {
		empty := make([]data.StockInfo, 0)
		return &empty
	}
	return out
}

// getStockInfosWithQuoteService Phase7-A2-3-1：Monitor 展示取数经 QuoteService（可注入 fake）。
// applyFollow=false 时跳过 addStockFollowData（单测避免 NewStockDataApi→DB）。
func getStockInfosWithQuoteService(svc quoteBatchService, applyFollow bool, follows ...data.FollowedStock) (*[]data.StockInfo, error) {
	stockInfos := make([]data.StockInfo, 0)
	stockCodes := make([]string, 0)
	for _, follow := range follows {
		if strutil.HasPrefixAny(follow.StockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}
		stockCodes = append(stockCodes, follow.StockCode)
	}
	if len(stockCodes) == 0 {
		return &stockInfos, nil
	}
	fetched, err := fetchRealtimeStockInfos(svc, stockCodes)
	if err != nil {
		return &stockInfos, quoteServiceFetchError(err)
	}
	if len(fetched) == 0 {
		return &stockInfos, nil
	}
	for _, info := range fetched {
		v, ok := slice.FindBy(follows, func(idx int, follow data.FollowedStock) bool {
			if strutil.HasPrefixAny(follow.StockCode, []string{"US", "us"}) {
				return strings.ToLower(strings.Replace(follow.StockCode, "us", "gb_", 1)) == info.Code
			}

			return follow.StockCode == info.Code
		})
		if ok {
			cp := info
			if applyFollow {
				addStockFollowData(v, &cp)
			}
			stockInfos = append(stockInfos, cp)
		}
	}
	return &stockInfos, nil
}

func GetStockInfosRealtimeBatch(follows ...data.FollowedStock) *[]data.StockInfo {
	return getStockInfosRealtimeBatchWithQuoteService(data.GetQuoteService(), true, follows...)
}

// getStockInfosRealtimeBatchWithQuoteService Phase7-A2-3-1：Dashboard batch miss 经 QuoteService；
// FollowRealtimePriceCache 保留；applyFollow=false 便于无 DB 单测。
func getStockInfosRealtimeBatchWithQuoteService(svc quoteBatchService, applyFollow bool, follows ...data.FollowedStock) *[]data.StockInfo {
	stockInfos := make([]data.StockInfo, 0)
	stockCodes := make([]string, 0, len(follows))
	for _, follow := range follows {
		stockCodes = append(stockCodes, follow.StockCode)
	}
	if len(stockCodes) == 0 {
		return &stockInfos
	}

	hits, misses := cache.FollowRealtimePriceCache.Partition(stockCodes)
	merged := make(map[string]data.StockInfo, len(hits)+len(misses))
	for code, p := range hits {
		if p != nil {
			merged[code] = *p
		}
	}
	if len(misses) > 0 {
		fetched, err := fetchRealtimeStockInfos(svc, misses)
		if err == nil && len(fetched) > 0 {
			cache.FollowRealtimePriceCache.SetBatchFromSlice(fetched)
			for _, info := range fetched {
				key := cache.NormalizeStockCode(info.Code)
				if key == "" {
					continue
				}
				merged[key] = info
			}
		}
	}

	for _, info := range merged {
		v, ok := slice.FindBy(follows, func(idx int, follow data.FollowedStock) bool {
			if strutil.HasPrefixAny(follow.StockCode, []string{"US", "us"}) {
				return strings.ToLower(strings.Replace(follow.StockCode, "us", "gb_", 1)) == info.Code
			}
			return cache.NormalizeStockCode(follow.StockCode) == cache.NormalizeStockCode(info.Code)
		})
		if ok {
			cp := info
			if applyFollow {
				addStockFollowData(v, &cp)
			}
			stockInfos = append(stockInfos, cp)
		}
	}
	return &stockInfos
}

func getStockInfo(follow data.FollowedStock) *data.StockInfo {
	stockCode := follow.StockCode
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || len(*stockDatas) == 0 {
		return &data.StockInfo{}
	}
	stockData := (*stockDatas)[0]
	addStockFollowData(follow, &stockData)
	return &stockData
}

func addStockFollowData(follow data.FollowedStock, stockData *data.StockInfo) {
	stockData.PrePrice = follow.Price //上次当前价格
	stockData.Sort = follow.Sort
	stockData.CostPrice = follow.CostPrice //成本价
	stockData.CostVolume = follow.Volume   //成本量
	stockData.AlarmChangePercent = follow.AlarmChangePercent
	stockData.AlarmPrice = follow.AlarmPrice
	stockData.Groups = follow.Groups
	if !follow.Time.IsZero() {
		stockData.FollowTime = follow.Time.Format("2006-01-02 15:04:05")
	}
	// P1 热路径：行业/板块走内存缓存，禁止每 tick HTTP
	api := data.NewStockDataApi()
	industry, bkName := api.LookupStockIndustrySectorCached(follow.StockCode)
	stockData.Industry = industry
	stockData.BKName = bkName

	//当前价格
	price, _ := convertor.ToFloat(stockData.Price)
	//当前价格为0 时 使用卖一价格作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.A1P)
	}
	//当前价格依然为0 时 使用买一报价作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.B1P)
	}

	//昨日收盘价
	preClosePrice, _ := convertor.ToFloat(stockData.PreClose)

	//当前价格依然为0 时 使用昨日收盘价为当前价格
	if price == 0 {
		price = preClosePrice
	}

	//今日最高价
	highPrice, _ := convertor.ToFloat(stockData.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(stockData.Open)
	}

	//今日最低价
	lowPrice, _ := convertor.ToFloat(stockData.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(stockData.Open)
	}
	//开盘价
	//openPrice, _ := convertor.ToFloat(stockData.Open)

	if price > 0 && preClosePrice > 0 {
		stockData.ChangePrice = mathutil.RoundToFloat(price-preClosePrice, 2)
		stockData.ChangePercent = mathutil.RoundToFloat(mathutil.Div(price-preClosePrice, preClosePrice)*100, 2)
	}
	if highPrice > 0 && preClosePrice > 0 {
		stockData.HighRate = mathutil.RoundToFloat(mathutil.Div(highPrice-preClosePrice, preClosePrice)*100, 2)
	}
	if lowPrice > 0 && preClosePrice > 0 {
		stockData.LowRate = mathutil.RoundToFloat(mathutil.Div(lowPrice-preClosePrice, preClosePrice)*100, 2)
	}
	if follow.CostPrice > 0 && follow.Volume > 0 {
		if price > 0 {
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(price-follow.CostPrice, follow.CostPrice)*100, 2)
			stockData.ProfitAmount = mathutil.RoundToFloat((price-follow.CostPrice)*float64(follow.Volume), 2)
			stockData.ProfitAmountToday = mathutil.RoundToFloat((price-preClosePrice)*float64(follow.Volume), 2)
		} else {
			//未开盘时当前价格为昨日收盘价
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-follow.CostPrice, follow.CostPrice)*100, 2)
			stockData.ProfitAmount = mathutil.RoundToFloat((preClosePrice-follow.CostPrice)*float64(follow.Volume), 2)
			// 未开盘时，今日盈亏为 0
			stockData.ProfitAmountToday = 0
		}

	}

	priceUnchanged := price > 0 && follow.Price == price
	followBaseline := follow.FollowPrice
	if followBaseline <= 0 || !priceUnchanged {
		followBaseline = api.ResolveFollowBaselinePrice(follow, price)
	}
	stockData.FollowPrice = followBaseline
	// 未变价：跳过 follow_price / price 持久化与异步补全（热路径减负）
	if !priceUnchanged {
		if followBaseline > 0 && followBaseline != follow.FollowPrice {
			go func(code string, baseline float64) {
				db.Dao.Model(&data.FollowedStock{}).Where("stock_code = ?", code).Update("follow_price", baseline)
			}(follow.StockCode, followBaseline)
		} else if follow.FollowPrice <= 0 && !follow.Time.IsZero() {
			go func(code string, t time.Time) {
				p := data.NewStockDataApi().LookupFollowBaselinePrice(code, t)
				if p > 0 {
					db.Dao.Model(&data.FollowedStock{}).Where("stock_code = ? AND (follow_price IS NULL OR follow_price = 0)", code).Update("follow_price", p)
				}
			}(follow.StockCode, follow.Time)
		}
		if price > 0 {
			go db.Dao.Model(follow).Where("stock_code = ?", follow.StockCode).Updates(map[string]interface{}{
				"price": price,
			})
		}
	}
	if followBaseline > 0 {
		refPrice := price
		if refPrice <= 0 {
			refPrice = preClosePrice
		}
		if refPrice > 0 {
			stockData.FollowProfit = mathutil.RoundToFloat(mathutil.Div(refPrice-followBaseline, followBaseline)*100, 2)
		}
	}
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	defer PanicHandler()
	a.stopTradingEventBridge()
	// 记录当前窗口大小，供下次启动时还原
	if a.ctx != nil {
		if w, h := runtime.WindowGetSize(a.ctx); w > 0 && h > 0 {
			cfg := data.GetSettingConfig()
			cfg.WindowWidth = w
			cfg.WindowHeight = h
			data.UpdateConfig(cfg)
			//logger.SugaredLogger.Infof("save window size: %dx%d", w, h)
		}
	}
	//logger.SugaredLogger.Infof("application shutdown Version:%s", Version)
}

// Greet returns a greeting for the given name
func (a *App) Greet(stockCode string) *data.StockInfo {
	//stockInfo, _ := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)

	follow := &data.FollowedStock{
		StockCode: stockCode,
	}
	db.Dao.Model(follow).Where("stock_code = ?", stockCode).Preload("Groups").Preload("Groups.GroupInfo").First(follow)
	stockInfo := getStockInfo(*follow)
	return stockInfo
}

func (a *App) Follow(stockCode string) string {
	return data.NewStockDataApi().Follow(stockCode)
}

func (a *App) UnFollow(stockCode string) string {
	return data.NewStockDataApi().UnFollow(stockCode)
}

func (a *App) GetFollowList(groupId int) *[]data.FollowedStock {
	return data.NewStockDataApi().GetFollowList(groupId)
}

func (a *App) GetFollowRealtimeList(groupId int) *[]data.StockInfo {
	follows := data.NewStockDataApi().GetFollowList(groupId)
	if follows == nil || len(*follows) == 0 {
		empty := []data.StockInfo{}
		return &empty
	}
	// MD-001 M1：经 App.marketData（MarketDataService.GetQuotes）；包级 Monitor 路径仍用 GetQuoteService。
	if a != nil && a.marketData != nil {
		return getStockInfosRealtimeBatchWithQuoteService(a.marketData, true, *follows...)
	}
	return GetStockInfosRealtimeBatch(*follows...)
}

func (a *App) GetStockList(key string) []data.StockBasic {
	return data.NewStockDataApi().GetStockList(key)
}

func (a *App) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return data.NewStockDataApi().SetCostPriceAndVolume(price, volume, stockCode)
}

func (a *App) SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	return data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
}

func (a *App) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return data.NewStockDataApi().SetAlarmChangePercent(val, alarmPrice, stockCode)
}
func (a *App) SetStockSort(sort int64, stockCode string) {
	data.NewStockDataApi().SetStockSort(sort, stockCode)
}
func (a *App) SendDingDingMessage(message string, stockCode string) string {
	ttl, _ := a.cache.TTL([]byte(stockCode))
	//logger.SugaredLogger.Infof("stockCode %s ttl:%d", stockCode, ttl)
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), 60*5)
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
func (a *App) SendDingDingMessageByType(message string, stockCode string, msgType int) string {

	if strutil.HasPrefixAny(stockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
		return "非A股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
		return "非港股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
		return "非美股交易时间"
	}

	ttl, _ := a.cache.TTL([]byte(stockCode))
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), getMsgTypeTTL(msgType))
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	stockInfo := &data.StockInfo{}
	db.Dao.Model(stockInfo).Where("code = ?", stockCode).First(stockInfo)
	go data.NewAlertWindowsApi("go-stock消息通知", getMsgTypeName(msgType), GenNotificationMsg(stockInfo), "").SendNotification()

	go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
		"time":    "📈 " + getMsgTypeName(msgType),
		"isRed":   true,
		"source":  "go-stock",
		"content": GenNotificationMsg(stockInfo),
	})

	return data.NewDingDingAPI().SendDingDingMessage(message)
}

func (a *App) NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool) {
	var msgs <-chan map[string]any
	if enableTools {
		msgs = data.NewDeepSeekOpenAi(a.ctx, aiConfigId).NewChatStream(stock, stockCode, question, sysPromptId, a.AiTools, think)
	} else {
		msgs = data.NewDeepSeekOpenAi(a.ctx, aiConfigId).NewChatStream(stock, stockCode, question, sysPromptId, []data.Tool{}, think)
	}
	for msg := range msgs {
		runtime.EventsEmit(a.ctx, "newChatStream", msg)
	}
	runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
}

func (a *App) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	data.NewDeepSeekOpenAi(a.ctx, aiConfigId).SaveAIResponseResult(stockCode, stockName, result, chatId, question)
}
func (a *App) GetAIResponseResult(stock string) *models.AIResponseResult {
	return data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stock)
}

func (a *App) GetVersionInfo() *models.VersionInfo {
	bridgeVersionIdentity()
	cur := version.Current()
	ver := Version
	if strings.TrimSpace(ver) == "" {
		ver = cur.Version
	}
	content := VersionCommit
	if strings.TrimSpace(content) == "" {
		content = cur.GitCommit
	}
	return &models.VersionInfo{
		Version:           ver,
		Icon:              GetImageBase(icon),
		Alipay:            "",
		Wxpay:             "",
		Wxgzh:             "",
		Content:           content,
		OfficialStatement: OFFICIAL_STATEMENT,
	}
}

//// checkChromeOnWindows 在 Windows 系统上检查谷歌浏览器是否安装
//func checkChromeOnWindows() bool {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	_, _, err = key.GetValue("Path", nil)
//	return err == nil
//}
//
//// checkEdgeOnWindows 在 Windows 系统上检查Edge浏览器是否安装，并返回安装路径
//func checkEdgeOnWindows() (string, bool) {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return "", false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	path, _, err := key.GetStringValue("Path")
//	if err != nil {
//		return "", false
//	}
//	return path, true
//}

func GetImageBase(bytes []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
}

func GenNotificationMsg(stockInfo *data.StockInfo) string {
	Price, err := convertor.ToFloat(stockInfo.Price)
	if err != nil {
		Price = 0
	}
	PreClose, err := convertor.ToFloat(stockInfo.PreClose)
	if err != nil {
		PreClose = 0
	}
	var RF float64
	if PreClose > 0 {
		RF = mathutil.RoundToFloat(((Price-PreClose)/PreClose)*100, 2)
	}

	return "[" + stockInfo.Name + "] " + stockInfo.Price + " " + convertor.ToString(RF) + "% " + stockInfo.Date + " " + stockInfo.Time
}

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟) 4 止盈报警(5分钟) 5 止损报警(5分钟)
func getMsgTypeTTL(msgType int) int {
	switch msgType {
	case 1:
		return 60 * 5
	case 2:
		return 60 * 30
	case 3:
		return 60 * 30
	case 4:
		return 60 * 5
	case 5:
		return 60 * 5
	default:
		return 60 * 5
	}
}

func getMsgTypeName(msgType int) string {
	switch msgType {
	case 1:
		return "涨跌报警"
	case 2:
		return "股价报警"
	case 3:
		return "成本价报警"
	case 4:
		return "止盈报警"
	case 5:
		return "止损报警"
	default:
		return "未知类型"
	}
}

func onExit(a *App) {
	// 清理操作
	//logger.SugaredLogger.Infof("systray onExit")
	//systray.Quit()
	//runtime.Quit(a.ctx)
}

func (a *App) UpdateConfig(settingConfig *data.SettingConfig) string {
	//s1, _ := json.Marshal(settingConfig)
	//logger.SugaredLogger.Infof("UpdateConfig:%s", s1)
	// 快照写入前配置：供 handleUpdateSettings 判断是否需要整页 reload
	prev := fingerprintSettings(data.GetSettingConfig())
	a.settingsReloadMu.Lock()
	a.settingsReloadPrev = prev
	a.settingsReloadMu.Unlock()

	// 热更新 P1 行情 cron 间隔，避免每次存配置都 WindowReloadApp
	if settingConfig.RefreshInterval > 0 {
		if entryID, exists := a.getCronEntry("MonitorStockPrices"); exists {
			a.cron.Remove(entryID)
		}
		id, _ := a.cron.AddFunc(fmt.Sprintf("@every %ds", settingConfig.RefreshInterval), func() {
			MonitorStockPrices(a)
		})
		a.setCronEntry("MonitorStockPrices", id)
	}

	return data.UpdateConfig(settingConfig)
}

// handleUpdateSettings 设置变更后：优先热更新主题；仅关键字段变化时才 WindowReloadApp。
func (a *App) handleUpdateSettings(ctx context.Context) {
	config := data.GetSettingConfig()
	if config != nil && config.Settings != nil {
		if config.DarkTheme {
			runtime.WindowSetBackgroundColour(ctx, 27, 38, 54, 1)
			runtime.WindowSetDarkTheme(ctx)
		} else {
			runtime.WindowSetBackgroundColour(ctx, 255, 255, 255, 1)
			runtime.WindowSetLightTheme(ctx)
		}
	}

	a.settingsReloadMu.Lock()
	prev := a.settingsReloadPrev
	a.settingsReloadPrev = nil
	a.settingsReloadMu.Unlock()

	next := fingerprintSettings(config)
	// 仅当代理/浏览器路径/Agent/资讯/基金/授权码等关键字段变化时整页重载；
	// RefreshInterval 已热更新 cron；DarkTheme 已通过原生 API 生效，前端 App.vue 监听 updateSettings。
	if settingsFingerprintChanged(prev, next) {
		logger.SugaredLogger.Infof("updateSettings: critical config changed, WindowReloadApp")
		runtime.WindowReloadApp(ctx)
	}
}

// emitMonitorPerf 向前端 performanceMetrics 上报 Monitor 耗时（P1 观测）。
func emitMonitorPerf(a *App, started time.Time) {
	if a == nil || a.ctx == nil {
		return
	}
	ms := time.Since(started).Milliseconds()
	go runtime.EventsEmit(a.ctx, "monitor_perf", map[string]interface{}{
		"name":       "monitor.stock_prices",
		"durationMs": ms,
	})
}

func (a *App) GetConfig() *data.SettingConfig {
	return data.GetSettingConfig()
}

func (a *App) ExportConfig() string {
	config := data.NewSettingsApi().Export()
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "导出配置文件（默认已脱敏）",
		CanCreateDirectories: true,
		DefaultFilename:      "config.json",
	})
	if err != nil {
		msg := security.SanitizeErrorMessage(err.Error())
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", msg)
		return msg
	}
	err = os.WriteFile(file, []byte(config), os.ModePerm)
	if err != nil {
		msg := security.SanitizeErrorMessage(err.Error())
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", msg)
		return msg
	}
	return "导出成功（已脱敏）:" + security.PathHint(file)
}

func (a *App) ShareAnalysis(stockCode, stockName string) string {
	_ = stockCode
	_ = stockName
	return "社区分享功能已停用。分析结果仍保存在本地，可使用导出功能。"
}

// ShareText 直接把文本分享到社区（用于 AI 助手等非 AIResponseResult 场景）
func (a *App) ShareText(text, title string) string {
	_ = strings.TrimSpace(text)
	_ = strings.TrimSpace(title)
	return "社区分享功能已停用。内容仍保存在本地会话中。"
}

func (a *App) GetfundList(key string) []data.FundBasic {
	return data.NewFundApi().GetFundList(key)
}
func (a *App) GetFollowedFund() []data.FollowedFund {
	return data.NewFundApi().GetFollowedFund()
}
func (a *App) FollowFund(fundCode string) string {
	return data.NewFundApi().FollowFund(fundCode)
}
func (a *App) UnFollowFund(fundCode string) string {
	return data.NewFundApi().UnFollowFund(fundCode)
}
func (a *App) SaveAsMarkdown(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006-01-02_15_04_05")
		file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "保存为Markdown",
			DefaultFilename: fmt.Sprintf("%s[%s]AI分析结果_%s.md", stockName, stockCode, analysisTime),
			Filters: []runtime.FileFilter{
				{
					DisplayName: "Markdown",
					Pattern:     "*.md;*.markdown",
				},
			},
		})
		if err != nil {
			return err.Error()
		}
		err = os.WriteFile(file, []byte(res.Content), 0644)
		return "已保存至：" + file
	}
	return "分析结果异常,无法保存。"
}

func (a *App) GetPromptTemplates(name, promptType string) *[]models.PromptTemplate {
	return data.NewPromptTemplateApi().GetPromptTemplates(name, promptType)
}
func (a *App) AddPrompt(prompt models.Prompt) string {
	promptTemplate := models.PromptTemplate{
		ID:      prompt.ID,
		Content: prompt.Content,
		Name:    prompt.Name,
		Type:    prompt.Type,
	}
	return data.NewPromptTemplateApi().AddPrompt(promptTemplate)
}
func (a *App) DelPrompt(id uint) string {
	return data.NewPromptTemplateApi().DelPrompt(id)
}
func (a *App) SetStockAICron(cronText, stockCode string) {
	data.NewStockDataApi().SetStockAICron(cronText, stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	if entryID, exists := a.getCronEntry(stockCode); exists {
		a.cron.Remove(entryID)
	}
	follow := data.NewStockDataApi().GetFollowedStockByStockCode(stockCode)
	id, _ := a.cron.AddFunc(cronText, a.AddCronTask(follow))
	a.setCronEntry(stockCode, id)

}
func (a *App) AddGroup(group data.Group) string {
	ok := data.NewStockGroupApi(db.Dao).AddGroup(group)
	if ok {
		return "添加成功"
	} else {
		return "添加失败"
	}
}
func (a *App) GetGroupList() []data.Group {
	return data.NewStockGroupApi(db.Dao).GetGroupList()
}

func (a *App) UpdateGroupSort(id int, newSort int) bool {
	return data.NewStockGroupApi(db.Dao).UpdateGroupSort(id, newSort)
}

func (a *App) InitializeGroupSort() bool {
	return data.NewStockGroupApi(db.Dao).InitializeGroupSort()
}

func (a *App) GetGroupStockList(groupId int) []data.GroupStock {
	return data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
}

func (a *App) AddStockGroup(groupId int, stockCode string) string {
	ok := data.NewStockGroupApi(db.Dao).AddStockGroup(groupId, stockCode)
	if ok {
		return "添加成功"
	} else {
		return "添加失败"
	}
}

func (a *App) RemoveStockGroup(code, name string, groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveStockGroup(code, name, groupId)
	if ok {
		return "移除成功"
	} else {
		return "移除失败"
	}
}

func (a *App) RemoveGroup(groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveGroup(groupId)
	if ok {
		return "移除成功"
	} else {
		return "移除失败"
	}
}

func (a *App) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	_ = stockName
	if a == nil || a.marketData == nil {
		return emptyKLineData()
	}
	bars, err := a.marketData.GetBars(stockCode, marketdata.PeriodDailyHK, "", int(days), time.Time{})
	if err != nil || len(bars) == 0 {
		return emptyKLineData()
	}
	return barsToKLineData(bars)
}

func (a *App) GetStockMinutePriceLineData(stockCode, stockName string) map[string]any {
	res := make(map[string]any, 4)
	res["stockName"] = stockName
	res["stockCode"] = stockCode
	if a == nil || a.marketData == nil {
		empty := []data.MinuteData{}
		res["priceData"] = &empty
		res["date"] = ""
		return res
	}
	pts, date, err := a.marketData.GetMinute(stockCode)
	if err != nil {
		empty := []data.MinuteData{}
		res["priceData"] = &empty
		res["date"] = date
		return res
	}
	res["priceData"] = minutesToData(pts)
	res["date"] = date
	return res
}

func (a *App) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	_ = stockName
	if a == nil || a.marketData == nil {
		return emptyKLineData()
	}
	bars, err := a.marketData.GetBars(stockCode, marketdata.PeriodDailyFQ, "", int(days), time.Time{})
	if err != nil || len(bars) == 0 {
		return emptyKLineData()
	}
	return barsToKLineData(bars)
}

// GetStockEastMoneyKLine 东方财富多周期 K 线（分钟：1/5/10/60/120；日 101、周 102、半年 105、年 106）。
// klt 与东方财富接口一致；10 分钟由 1 分钟数据聚合。limit 为根数上限（最大 5000）。
func (a *App) GetStockEastMoneyKLine(stockCode, stockName string, klt string, limit int) *[]data.KLineData {
	return a.GetStockEastMoneyKLinePage(stockCode, stockName, klt, limit, "")
}

// GetStockEastMoneyKLinePage 分页拉取 K 线：end 为东财 end 参数（YYYYMMDD 或 YYYYMMDDHHmmss），空字符串表示取最新一段（同 GetStockEastMoneyKLine）。
// MD-001 M1：经 MarketDataService.GetBars → EastMoney Adapter（底层仍走 kline_cache，无第二层缓存）。
func (a *App) GetStockEastMoneyKLinePage(stockCode, stockName string, klt string, limit int, end string) *[]data.KLineData {
	_ = stockName
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "1"
	}
	if a == nil || a.marketData == nil {
		return emptyKLineData()
	}
	endTime := marketdata.ParseEndTime(end)
	bars, err := a.marketData.GetBars(stockCode, klt, "", limit, endTime)
	if err != nil || len(bars) == 0 {
		return emptyKLineData()
	}
	return barsToKLineData(bars)
}

func (a *App) GetTelegraphList(source string) *[]*models.Telegraph {
	telegraphs := data.NewMarketNewsApi().GetTelegraphList(source)
	return telegraphs
}

func (a *App) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	//data.NewMarketNewsApi().GetNewTelegraph(30)
	go data.NewMarketNewsApi().TelegraphList(30)
	go data.NewMarketNewsApi().GetSinaNews(30)
	go data.NewMarketNewsApi().TradingViewNews()
	telegraphs := data.NewMarketNewsApi().GetTelegraphList(source)
	return telegraphs
}

func (a *App) GlobalStockIndexes() map[string]any {
	return data.NewMarketNewsApi().GlobalStockIndexes(30)
}

// GlobalStockIndexesReadable 将全球指数 JSON 转为 AI 易读 Markdown 文本。
func (a *App) GlobalStockIndexesReadable() string {
	return data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
}

func (a *App) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	ctx, cancel := context.WithCancel(a.ctx)

	// 保存当前会话的 cancel，用于前端中断
	a.summaryMu.Lock()
	if a.summaryCancel != nil {
		a.summaryCancel()
	}
	a.summaryCancel = cancel
	a.summaryMu.Unlock()

	// 允许前端自定义事件名，避免不同页面之间的事件冲突
	if strings.TrimSpace(eventName) == "" {
		eventName = "summaryStockNews"
	}

	// 解析对话历史（AI 助手记忆）：空字符串或解析失败则无历史
	var history []map[string]interface{}
	if strings.TrimSpace(historyJSON) != "" {
		var list []models.AiAssistantMessage
		if err := json.Unmarshal([]byte(historyJSON), &list); err == nil && len(list) > 0 {
			history = make([]map[string]interface{}, 0, len(list))
			for _, m := range list {
				item := map[string]interface{}{"role": m.Role, "content": m.Content}
				if m.Role == "assistant" && m.Reasoning != "" {
					item["reasoning_content"] = m.Reasoning
				}
				history = append(history, item)
			}
		}
	}

	var msgs <-chan map[string]any
	if enableTools {
		msgs = data.NewDeepSeekOpenAi(ctx, aiConfigId).NewSummaryStockNewsStreamWithTools(question, sysPromptId, a.AiTools, think, history)
	} else {
		msgs = data.NewDeepSeekOpenAi(ctx, aiConfigId).NewSummaryStockNewsStream(question, sysPromptId, think, history)
	}

	for msg := range msgs {
		runtime.EventsEmit(a.ctx, eventName, msg)
	}

	a.summaryMu.Lock()
	a.summaryCancel = nil
	a.summaryMu.Unlock()

	runtime.EventsEmit(a.ctx, eventName, "DONE")
}
func (a *App) GetIndustryRank(sort string, cnt int) []any {
	res := data.NewMarketNewsApi().GetIndustryRank(sort, cnt)
	if res == nil {
		return []any{}
	}
	data, ok := res["data"].([]any)
	if !ok || data == nil {
		return []any{}
	}
	return data
}
func (a *App) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	res := data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
	return res
}
func (a *App) GetMoneyRankSina(sort string) []map[string]any {
	res := data.NewMarketNewsApi().GetMoneyRankSina(sort)
	api := data.NewStockDataApi()
	for i := range res {
		symbol := convertor.ToString(res[i]["symbol"])
		if symbol == "" {
			continue
		}
		industry, _ := api.LookupStockIndustrySector(symbol)
		res[i]["industry"] = industry
	}
	return res
}

func (a *App) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	res := data.NewMarketNewsApi().GetStockMoneyTrendByDay(stockCode, days)
	slice.Reverse(res)
	return res
}

// OpenURL
//
//	@Description:  跨平台打开默认浏览器
//	@receiver a
//	@param url
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// SaveImage
//
//	@Description: 跨平台保存图片
//	@receiver a
//	@param name
//	@param base64Data
//	@return error
func (a *App) SaveImage(name, base64Data string) string {
	// 打开保存文件对话框
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: name + "AI分析.png",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PNG 图片",
				Pattern:     "*.png",
			},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	// 解码并保存
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "文件内容异常,无法保存。"
	}

	err = os.WriteFile(filepath.Clean(filePath), decodeString, os.ModePerm)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// SaveWordFile
//
//	@Description: // 跨平台保存word
//	@receiver a
//	@param filename
//	@param base64Data
//	@return error
func (a *App) SaveWordFile(filename string, base64Data string) string {
	// 弹出保存文件对话框
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 Word 文件",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "Word 文件", Pattern: "*.docx"},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	// 解码 base64 内容
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "文件内容异常,无法保存。"
	}
	// 保存为文件
	err = os.WriteFile(filepath.Clean(filePath), decodeString, 0777)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

// GetAiConfigs
//
//	@Description: // 获取 AiConfig 列表
//	@receiver a
//	@return error
func (a *App) GetAiConfigs() []*data.AIConfig {
	return data.GetSettingConfig().AiConfigs
}

// GetAiAssistantSession 获取 AI 助手会话消息列表，sessionId 为空时获取最新的
func (a *App) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	return data.GetAiAssistantSession(sessionId)
}

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库
func (a *App) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return data.SaveAiAssistantSession(sessionId, messages)
}

// FetchAiModels
//
//	@Description: 根据接口地址与 apiKey 自动获取支持的模型列表（OpenAI/DeepSeek 兼容 /models 接口）
//	@receiver a
//	@param baseUrl 接口地址（如 https://api.deepseek.com）
//	@param apiKey  鉴权令牌
//	@return []string 模型 ID 列表
func (a *App) FetchAiModels(baseUrl, apiKey string) []string {
	baseUrl = strutil.Trim(baseUrl)
	apiKey = strutil.Trim(apiKey)
	if baseUrl == "" || apiKey == "" {
		return []string{}
	}

	type modelItem struct {
		ID string `json:"id"`
	}
	var respData struct {
		Data []modelItem `json:"data"`
	}

	client := resty.New()
	client.SetBaseURL(baseUrl)
	client.SetHeader("Authorization", "Bearer "+apiKey)
	client.SetHeader("Content-Type", "application/json")

	resp, err := client.R().
		SetResult(&respData).
		Get("/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels error: %v", err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels http error: %s", resp.Status())
		return []string{}
	}

	modelsList := make([]string, 0, len(respData.Data))
	for _, m := range respData.Data {
		if strings.TrimSpace(m.ID) != "" {
			modelsList = append(modelsList, m.ID)
		}
	}
	return modelsList
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度
func (a *App) InitCronTasks() {
	tasks := agent.NewCronTaskApi().GetAll()
	if len(tasks) == 0 {
		return
	}
	for _, t := range tasks {
		// 避免闭包捕获循环变量
		taskCopy := t
		entryID, err := a.cron.AddFunc(taskCopy.CronExpr, func() {
			err := agent.NewCronTaskApi().ExecuteTask(a.ctx, &taskCopy)
			if err != nil {
				logger.SugaredLogger.Errorf("启动任务失败：%v %s", err, taskCopy.Name)
				return
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建定时任务失败：%v %s", err, taskCopy.Name)
			continue
		}
		a.setCronEntry(convertor.ToString(taskCopy.ID)+"_"+taskCopy.Name, entryID)
		//logger.SugaredLogger.Infof("自动创建定时任务成功：%s (ID:%d) entryID:%v", taskCopy.Name, taskCopy.ID, entryID)
	}
}

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答
func (a *App) AbortSummaryStockNews() {
	a.summaryMu.Lock()
	defer a.summaryMu.Unlock()
	if a.summaryCancel != nil {
		a.summaryCancel()
		a.summaryCancel = nil
	}
}

// CreateCronTask
//
//	@Description: 创建定时任务
//	@receiver a
//	@param task 定时任务信息
//	@return string 操作结果
func (a *App) CreateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Create(task)
	if err != nil {
		return fmt.Sprintf("创建失败：%v", err)
	}
	entryID, err := a.cron.AddFunc(task.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return "任务创建成功,但定时失败"
	}
	return "创建成功"
}

func (a *App) UpdateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Update(task)
	if entryID, exists := a.getCronEntry(convertor.ToString(task.ID) + "_" + task.Name); exists {
		a.cron.Remove(entryID)
	}
	entryID, err := a.cron.AddFunc(task.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
			return
		}
	})
	a.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	return "更新成功"
}

// DeleteCronTask
//
//	@Description: 删除定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) DeleteCronTask(id uint) string {
	err := agent.NewCronTaskApi().Delete(id)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
	}
	if err != nil {
		return fmt.Sprintf("删除失败：%v", err)
	}
	return "删除成功"
}

// GetCronTaskByID
//
//	@Description: 根据 ID 获取定时任务
//	@receiver a
//	@param id 任务 ID
//	@return *models.CronTask 任务信息
func (a *App) GetCronTaskByID(id uint) *models.CronTask {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return nil
	}
	return task
}

// GetCronTaskList
//
//	@Description: 获取定时任务列表
//	@receiver a
//	@param query 查询条件
//	@return *models.CronTaskPageResp 分页结果
func (a *App) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	return agent.NewCronTaskApi().List(query)
}

// EnableCronTask
//
//	@Description: 启用/禁用定时任务
//	@receiver a
func (a *App) EnableCronTask(id uint, enable bool) string {
	err := agent.NewCronTaskApi().EnableTask(id, enable)
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err == nil {
		if entryID, exists := a.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			a.cron.Remove(entryID)
		}
		if enable {
			entryID, err := a.cron.AddFunc(task.CronExpr, func() {
				err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
				if err != nil {
					logger.SugaredLogger.Errorf("%s 执行任务失败：%v", task.Name, err)
					return
				}
			})
			a.setCronEntry(convertor.ToString(id)+"_"+task.Name, entryID)
			if err != nil {
				return "操作成功,但定时失败"
			}
		}

	}
	if err != nil {
		return fmt.Sprintf("操作失败：%v", err)
	}
	return "操作成功"
}

// ExecuteCronTaskNow
//
//	@Description: 立即执行定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) ExecuteCronTaskNow(id uint) string {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return fmt.Sprintf("任务不存在：%v", err)
	}

	go func() {
		err := agent.NewCronTaskApi().ExecuteTask(a.ctx, task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
		}
	}()

	return "任务执行中"
}

// GetCronTaskTypes
//
//	@Description: 获取所有任务类型
//	@receiver a
//	@return []lo.Tuple2[string, string] 任务类型列表
func (a *App) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return agent.NewCronTaskApi().GetTaskTypes()
}

// ValidateCronExpr
//
//	@Description: 验证 Cron 表达式
//	@receiver a
//	@param expr Cron 表达式
//	@return string 验证结果
func (a *App) ValidateCronExpr(expr string) string {
	err := agent.NewCronTaskApi().ValidateCronExpr(expr)
	if err != nil {
		return fmt.Sprintf("无效表达式：%v", err)
	}
	return "有效表达式"
}

// SearchCronTasks
//
//	@Description: 搜索定时任务
//	@receiver a
//	@param keyword 搜索关键词
//	@return []models.CronTask 搜索结果
func (a *App) SearchCronTasks(keyword string) []models.CronTask {
	return agent.NewCronTaskApi().SearchTasks(keyword)
}

// CalculateNextRunTime 根据 Cron 表达式计算下一次运行时间
// 参数:
//   - cron: Cron 表达式，用于定义任务调度的时间规则
//
// 返回值:
//   - string: 格式化为 "2006-01-02 15:04:05" 的下一次运行时间字符串
func (a *App) CalculateNextRunTime(cron string) string {
	nextRunTime := agent.NewCronTaskApi().CalculateNextRunTime(cron)
	return nextRunTime.Format("2006-01-02 15:04:05")
}

// CalculateNextRunTimes 根据 Cron 表达式计算未来多次运行时间
// 参数:
//   - cron: Cron 表达式
//   - count: 需要计算的次数
//
// 返回值:
//   - []string: 按时间顺序排序的运行时间列表，格式为 "2006-01-02 15:04:05"
func (a *App) CalculateNextRunTimes(cron string, count int) []string {
	times := agent.NewCronTaskApi().CalculateNextRunTimes(cron, count)
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, t.Format("2006-01-02 15:04:05"))
	}
	return result
}

// AddTradingRecord 添加交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - uint: 新添加的交易记录ID
//   - error: 错误信息
func (a *App) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return data.NewStockDataApi().AddTradingRecord(record)
}

// UpsertSellSignalDraft 止/减信号自动生成卖出交易草稿
func (a *App) UpsertSellSignalDraft(record data.TradingRecord) (uint, error) {
	return data.NewStockDataApi().UpsertSellSignalDraft(record)
}

// UpsertBuySignalDraft 买点信号自动生成买入交易草稿
func (a *App) UpsertBuySignalDraft(record data.TradingRecord) (uint, error) {
	return data.NewStockDataApi().UpsertBuySignalDraft(record)
}

// ConfirmTradingRecordDraft 确认交易草稿为正式记录
func (a *App) ConfirmTradingRecordDraft(id uint) error {
	return data.NewStockDataApi().ConfirmTradingRecordDraft(id)
}

// GetPaperAccountSnapshot 模拟账户快照
func (a *App) GetPaperAccountSnapshot(accountId uint) *data.PaperAccountSnapshot {
	snap, err := data.NewPaperTradingApi().GetSnapshot(accountId)
	if err != nil {
		return &data.PaperAccountSnapshot{}
	}
	return snap
}

// ResetPaperAccount 重置模拟账户
func (a *App) ResetPaperAccount(initialCash float64) *data.PaperAccount {
	acc, err := data.NewPaperTradingApi().ResetAccount(initialCash)
	if err != nil {
		return nil
	}
	return acc
}

// SubmitPaperOrder 提交模拟委托（可立即撮合）
func (a *App) SubmitPaperOrder(req data.PaperSubmitOrderReq) (*data.PaperOrder, error) {
	return data.NewPaperTradingApi().SubmitPaperOrder(req)
}

// FillPaperOrder 撮合模拟委托
func (a *App) FillPaperOrder(orderId uint, fillPrice float64) error {
	return data.NewPaperTradingApi().FillPaperOrder(orderId, fillPrice)
}

// SetPaperMarkPrice 更新模拟持仓市价并重新计算账户权益。
func (a *App) SetPaperMarkPrice(accountId uint, stockCode string, markPrice float64) error {
	return data.NewPaperTradingApi().SetMarkPrice(accountId, stockCode, markPrice)
}

// SettlePaperSellable 日终：持仓全部变为可卖（T+1）
func (a *App) SettlePaperSellable(accountId uint) error {
	return data.NewPaperTradingApi().SettleAllSellable(accountId)
}

// ConfirmBrokerOrderPlan 人工确认后的券商送单（默认 manual 适配器，不真实下单）
func (a *App) ConfirmBrokerOrderPlan(stockCode, stockName, side string, price float64, volume int64, reason string) *broker.OrderResult {
	res, err := broker.Default().PlaceOrder(context.Background(), broker.OrderRequest{
		StockCode: stockCode,
		StockName: stockName,
		Side:      side,
		Price:     price,
		Volume:    volume,
		Reason:    reason,
	})
	if err != nil {
		return &broker.OrderResult{OK: false, Message: err.Error()}
	}
	return res
}

// GetTradingRecordList 获取交易记录列表（分页与筛选，返回结构与 AI 推荐列表一致）
func (a *App) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	page, err := data.NewStockDataApi().GetTradingRecordList(query)
	if err != nil {
		return &data.TradingRecordPageData{}
	}
	return page
}

// GetTradingRecordById 根据ID获取单个交易记录
// 参数:
//   - id: 交易记录ID
//
// 返回值:
//   - *data.TradingRecord: 交易记录指针
//   - error: 错误信息
func (a *App) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	return data.NewStockDataApi().GetTradingRecordById(id)
}

// GetTradingRecordStatistics 获取交易记录统计数据
//
// 返回值:
//   - *data.TradingRecordStatistics: 统计数据指针
func (a *App) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	stats, err := data.NewStockDataApi().GetTradingRecordStatistics()
	if err != nil {
		return &data.TradingRecordStatistics{}
	}
	return stats
}

// UpdateTradingRecord 更新交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - error: 错误信息
func (a *App) UpdateTradingRecord(record data.TradingRecord) error {
	return data.NewStockDataApi().UpdateTradingRecord(record)
}

// DeleteTradingRecord 删除交易记录
// 参数:
//   - id: 交易记录ID
//
// 返回值:
//   - error: 错误信息
func (a *App) DeleteTradingRecord(id uint) error {
	return data.NewStockDataApi().DeleteTradingRecord(id)
}

// CheckFrequentTrading 检查是否频繁交易
// 参数:
//   - stockCode: 股票代码
//
// 返回值:
//   - map[string]any: 包含 canTrade (bool) 和 msg (string)
func (a *App) CheckFrequentTrading(stockCode string) map[string]any {
	canTrade, msg := data.NewStockDataApi().CheckFrequentTrading(stockCode)
	return map[string]any{
		"canTrade": canTrade,
		"msg":      msg,
	}
}

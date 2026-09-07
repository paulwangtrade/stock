package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	assistantweb "go-stock/ai-assistant-web"
	"go-stock/backend/data"
	"go-stock/backend/database"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"go-stock/backend/healthcheck"
	"go-stock/backend/models"
	"go-stock/backend/version"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/api"
	goruntime "go-stock/backend/runtime"
	"go-stock/backend/strategysnapshot"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/app.ico
var icon2 []byte

var mainAppCtx context.Context

//go:embed build/stock_basic.json
var stocksBin []byte

//go:embed build/stock_base_info_hk.json
var stocksBinHK []byte

//go:embed build/stock_base_info_us.json
var stocksBinUS []byte

// 勿 go generate 覆盖 build/bin/data：exe 运行时数据在 build/bin/data，编译请用 scripts/build-windows.ps1

var Version string
var VersionCommit string
var OFFICIAL_STATEMENT string
var BuildKey string

const defaultRuntimeDBDSN = "data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"

type runtimeDataBindingInfo struct {
	WorkingDirectory string
	DatabasePath     string
	DatabaseExists   bool
	DatabaseSize     int64
	DatabaseHash8    string
}

func runtimeDataBindingSnapshot(dsn string) runtimeDataBindingInfo {
	wd, err := os.Getwd()
	if err != nil {
		wd = "<unknown>"
	}
	rawPath := strings.TrimSpace(dsn)
	if rawPath == "" {
		rawPath = defaultRuntimeDBDSN
	}
	dbPath := rawPath
	if i := strings.Index(dbPath, "?"); i >= 0 {
		dbPath = dbPath[:i]
	}
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(wd, dbPath)
	}
	absPath, err := filepath.Abs(dbPath)
	if err == nil {
		dbPath = absPath
	}

	info := runtimeDataBindingInfo{
		WorkingDirectory: wd,
		DatabasePath:     dbPath,
	}
	st, err := os.Stat(dbPath)
	if err != nil || st.IsDir() {
		return info
	}
	info.DatabaseExists = true
	info.DatabaseSize = st.Size()

	content, err := os.ReadFile(dbPath)
	if err != nil {
		return info
	}
	sum := sha256.Sum256(content)
	info.DatabaseHash8 = strings.ToUpper(hex.EncodeToString(sum[:])[:8])
	return info
}

func logRuntimeDataBinding(info runtimeDataBindingInfo) {
	log.SugaredLogger.Info("Runtime Data Binding:")
	log.SugaredLogger.Info("Working Directory: ", info.WorkingDirectory)
	log.SugaredLogger.Info("Database: ", info.DatabasePath)
	log.SugaredLogger.Infof("Database File Exists: %v", info.DatabaseExists)
	log.SugaredLogger.Infof("Database File Size: %d", info.DatabaseSize)
	log.SugaredLogger.Info("Database Fingerprint: ", info.DatabaseHash8)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			log.SugaredLogger.Error("panic: ", r)
			log.SugaredLogger.Error("stack: ", stack)
			log.SugaredLogger.Infof("release identity: %s", version.LogLine())
			if path, err := version.WriteCrashReport(r, stack); err != nil {
				log.SugaredLogger.Errorf("crash report write failed: %v", err)
			} else {
				log.SugaredLogger.Infof("crash report written: %s", path)
			}
		}
	}()

	// Phase16.16-B: Runtime Storage Layer — anchors all paths to exe directory.
	rtProfile, rtErr := goruntime.Init("", Version)
	if rtErr != nil {
		// Non-fatal: fall back to legacy cwd-relative paths.
		log.SugaredLogger.Warnf("runtime.Init failed (falling back to cwd paths): %v", rtErr)
	} else {
		// Reinitialise logger to use the runtime-resolved log directory.
		log.InitWithDir(rtProfile.LogDir)
		for _, line := range rtProfile.LogLines() {
			log.SugaredLogger.Info(line)
		}
	}

	// checkDir kept for backward-compat (tests / legacy); runtime.Init already
	// called MkdirAll for data/, logs/, runtime/.
	checkDir("data")
	data.SponsorDecryptKeyHex = BuildKey

	// Pass absolute database path from RuntimeProfile when available.
	var dbPath string
	if rtProfile != nil {
		dbPath = rtProfile.DatabasePath + "?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"
	}
	db.Init(dbPath)
	binding := runtimeDataBindingSnapshot(dbPath)
	logRuntimeDataBinding(binding)
	// Phase10-H.3: backup → integrity_check before migrate / trading init.
	safety := database.Default().RunAtStartup(binding.DatabasePath, db.Dao)
	database.SetLastResult(safety)
	data.InitAnalyzeSentiment()
	if !safety.OK {
		log.SugaredLogger.Errorf("DATABASE_SAFETY_FAILED ok=false trading_blocked=true err=%s", safety.Error)
		log.SugaredLogger.Errorf("DATABASE_SAFETY_RECOVERY: %s", safety.RecoveryHint)
		log.SugaredLogger.Errorf("DATABASE_SAFETY: skipping AutoMigrate; UI may start but trading cron/init stays blocked")
	} else {
		log.SugaredLogger.Infof("DATABASE_SAFETY_OK integrity=%s backup=%s", safety.Integrity, safety.BackupPath)
		// Sync migrate before Wails startup/cron to avoid 9:20 schema races.
		AutoMigrate()
		// Phase16.18-B3: persist StrategySnapshot for historical Explain recovery.
		strategysnapshot.InitDefaultStore(db.Dao)
	}
	//db.Dao.Model(&data.Group{}).Where("id = ?", 0).FirstOrCreate(&data.Group{
	//	Name: "默认分组",
	//	Sort: 0,
	//})

	log.SugaredLogger.Info("starting...")
	_ = version.Bootstrap(Version, VersionCommit)
	wireDiagnosticProviders()
	log.SugaredLogger.Infof("release identity: %s", version.LogLine())
	//log.SugaredLogger.Infof("build key: %s", BuildKey)

	// 程序启动时预缓存东财 Cookie
	//go func() {
	//	cacheCookies("https://push2his.eastmoney.com/api/qt/stock/kline/get")
	//}()

	// Create an instance of the app structure
	app := NewApp()
	AppMenu := menu.NewMenu()
	if IsMacOS() {
		AppMenu.Append(menu.EditMenu())
	}
	//FileMenu := AppMenu.AddSubmenu("设置")
	//FileMenu.AddText("窗口全屏", keys.CmdOrCtrl("f"), func(callback *menu.CallbackData) {
	//	runtime.WindowFullscreen(app.ctx)
	//})
	//FileMenu.AddText("窗口还原", keys.Key("Esc"), func(callback *menu.CallbackData) {
	//	runtime.WindowUnfullscreen(app.ctx)
	//})
	//FileMenu.AddText("显示搜索框", keys.CmdOrCtrl("s"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 1)
	//})
	//FileMenu.AddText("隐藏搜索框", keys.CmdOrCtrl("d"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 0)
	//})
	//FileMenu.AddText("刷新数据", keys.CmdOrCtrl("r"), func(callbackData *menu.CallbackData) {
	//	//runtime.EventsEmit(app.ctx, "refresh", "setting-"+time.Now().Format("2006-01-02 15:04:05"))
	//	runtime.EventsEmit(app.ctx, "refreshFollowList", "refresh-"+time.Now().Format("2006-01-02 15:04:05"))
	//})
	//FileMenu.AddSeparator()

	//if goruntime.GOOS == "windows" {
	//	FileMenu.AddText("隐藏到托盘区", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) {
	//		runtime.WindowHide(app.ctx)
	//	})
	//}

	//FileMenu.AddText("退出", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
	//	runtime.Quit(app.ctx)
	//})
	log.SugaredLogger.Infof("release identity: %s", version.LogLine())
	// 根据屏幕分辨率自适应窗口尺寸
	width, height, _, _, err := getScreenResolution()
	if err != nil {
		log.SugaredLogger.Error("get screen resolution error")
		// 获取失败时给一个合理的默认值
		width = 1412
		height = 834
	}

	darkTheme := data.GetSettingConfig().DarkTheme
	backgroundColour := &options.RGBA{R: 255, G: 255, B: 255, A: 1}
	if darkTheme {
		backgroundColour = &options.RGBA{R: 27, G: 38, B: 54, A: 1}
	}

	//frameless := getFrameless()

	// 计算默认窗口大小：优先使用上次保存的用户尺寸，否则自适应
	config := data.GetSettingConfig()

	appWidth := config.WindowWidth
	appHeight := config.WindowHeight

	// 若用户尚未调整过窗口或记录为 0，则按屏幕比例给一个合适默认值
	if appWidth <= 0 || appHeight <= 0 {
		appWidth = width * 5 / 10
		appHeight = height * 5 / 10
	}
	log.SugaredLogger.Info("screen resolution: " + convertor.ToString(width) + "x" + convertor.ToString(height))
	log.SugaredLogger.Info("window size: " + convertor.ToString(appWidth) + "x" + convertor.ToString(appHeight))

	// 作为 go-stock 子组件启动独立 Web 服务
	// 端口默认由 AI_ASSISTANT_WEB_ADDR 决定。
	go func() {
		if err := assistantweb.Start(); err != nil {
			log.SugaredLogger.Errorf("ai-assistant-web start error: %v", err)
		}
	}()

	// Create application with options
	err = wails.Run(&options.App{
		Title: "股票分析",
		// 默认窗口大小：自适应但保留明显边距
		Width:  appWidth,
		Height: appHeight,
		//MinWidth:  minWidth,
		//MinHeight: minHeight,
		// 限制最大尺寸不超过屏幕
		//MaxWidth:                 width,
		//MaxHeight:                height,
		DisableResize:            false,
		Fullscreen:               false,
		Frameless:                false,
		StartHidden:              false,
		HideWindowOnClose:        false,
		EnableDefaultContextMenu: true,
		BackgroundColour:         backgroundColour,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: api.ChainAssetMiddleware(
				api.StrategyIntentsAssetMiddleware,  // Phase13-B1-A: Strategy Intent read-only
				api.StrategySchemasAssetMiddleware, // Phase13-B3-A: Strategy Schema read-only
				api.ResearchCandidatesAssetMiddleware, // Phase13-A5 MVP-1: Research Candidate Pool
				api.CandidatePoolAssetMiddleware,
				api.RealOrdersAssetMiddleware,
				api.TradePlansAssetMiddleware,
				api.PaperTradingAssetMiddleware, // Phase10-C.2-A: observation + POST /run
				api.ExitReviewAssetMiddleware,   // Phase14-D3: POST /api/exit-review/outcome
				api.OpportunitiesAssetMiddleware, // Phase14-G1.1: opportunity user actions
				api.WatchlistAssetMiddleware,     // Phase16.26-C2.1: GET /api/watchlist
				api.PortfolioDashboardAssetMiddleware, // Phase11-B.2: GET /api/portfolio/dashboard
				api.TradingDayMonitorAssetMiddleware,  // Phase11-C: GET /api/trading/day-monitor
				api.DailyInvestmentSummaryAssetMiddleware, // Phase11-E: GET /api/investment/daily-summary
				api.OpsTradingDayAssetMiddleware,
				api.RecoveryReadinessAssetMiddleware,
				api.BrokerReconcileAssetMiddleware,
				api.ProductCapabilitiesAssetMiddleware, // Phase13-D: FeatureGate / Explain / Risk / Usage
			),
		},
		Menu:               AppMenu,
		Logger:             logger.NewFileLogger("./logs/wails.log"),
		LogLevel:           logger.DEBUG,
		LogLevelProduction: logger.INFO,
		OnStartup:          app.startup,
		OnDomReady:         app.domReady,
		OnBeforeClose:      app.beforeClose,
		OnShutdown:         app.shutdown,
		WindowStartState:   options.Normal,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "go-stock",
			OnSecondInstanceLaunch: OnSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath: "",
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "go-stock",
				Message: "go-stock：股票分析✨ ",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.SugaredLogger.Fatal(err)
	}

}

func cacheCookies(url string) {
	log.SugaredLogger.Info("预缓存东财 Cookie...")
	_, err := data.FetchEastMoneyCookiesViaChromedp("", 3*time.Minute, url)
	if err != nil {
		log.SugaredLogger.Warnf("预缓存东财 Cookie 失败：%v", err)
	} else {
		log.SugaredLogger.Info("东财 Cookie 预缓存完成")
	}
}

func updateMultipleModel() {
	oldSettings := &models.OldSettings{}
	db.Dao.Model(oldSettings).First(oldSettings)
	aiConfig := &data.AIConfig{}
	db.Dao.Model(aiConfig).First(aiConfig)
	if oldSettings.OpenAiEnable && oldSettings.OpenAiApiKey != "" && aiConfig.ID == 0 {
		aiConfig.Name = oldSettings.OpenAiModelName
		aiConfig.ApiKey = oldSettings.OpenAiApiKey
		aiConfig.BaseUrl = oldSettings.OpenAiBaseUrl
		aiConfig.ModelName = oldSettings.OpenAiModelName
		aiConfig.Temperature = oldSettings.OpenAiTemperature
		aiConfig.MaxTokens = oldSettings.OpenAiMaxTokens
		aiConfig.TimeOut = oldSettings.OpenAiApiTimeOut
		err := db.Dao.Model(aiConfig).Create(aiConfig).Error
		if err != nil {
			log.SugaredLogger.Error(err.Error())
		}
	}
}

func AutoMigrate() {
	if _, err := applyApplicationMigrations(); err != nil {
		log.SugaredLogger.Errorf("schema migration registry failed; app continues with trading blocked: %v", err)
	}
	validation := validateApplicationSchema()
	if !validation.Ready() {
		log.SugaredLogger.Errorf("startup schema validation %s: version=%d/%d missingTables=%v missingColumns=%v missingIndexes=%v errors=%v",
			validation.Status, validation.CurrentVersion, validation.RequiredVersion,
			validation.MissingTables, validation.MissingColumns, validation.MissingIndexes, validation.Errors)
	} else {
		log.SugaredLogger.Infof("startup schema validation READY version=%d", validation.CurrentVersion)
	}
	logDatabaseHealthCheck()
	// Wire readonly TradingDayStatus schema provider (re-validates on each query; no trading side effects).
	data.SetTradingDaySchemaStatusProvider(func() data.TradingDaySchemaView {
		v := validateApplicationSchema()
		return data.TradingDaySchemaView{
			RegistryVersion:  v.CurrentVersion,
			ValidationStatus: v.Status,
		}
	})

	go data.NewStockDataApi().BackfillMissingFollowPrices()

	//updateMultipleModel()

	// Defer CronTask seed slightly to avoid lock contention with extended migrations.
	go func() {
		time.Sleep(200 * time.Millisecond)
		initGlobalStockIndexCacheTask()
	}()
}

// runSchemaMigrations runs the full application migration registry (tests / explicit callers).
func runSchemaMigrations() error {
	_, err := applyApplicationMigrations()
	return err
}

// logDatabaseHealthCheck runs read-only DB probes at startup (no repair).
func logDatabaseHealthCheck() {
	if db.Dao == nil {
		return
	}
	res := healthcheck.Run(db.Dao)
	if res.Failed() {
		log.SugaredLogger.Errorf("DB_HEALTH_CHECK %s: %s", res.Status, res.Summary())
		for _, c := range res.Checks {
			if !c.Skipped && !c.OK {
				log.SugaredLogger.Errorf("DB_HEALTH_CHECK check=%s detail=%s", c.Name, c.Detail)
			}
		}
		return
	}
	log.SugaredLogger.Infof("DB_HEALTH_CHECK %s: %s", res.Status, res.Summary())
}

// initGlobalStockIndexCacheTask 检查并创建 global_stock_index_cache 定时任务
func initGlobalStockIndexCacheTask() {
	var count int64
	db.Dao.Model(&models.CronTask{}).Where("task_type = ?", "global_stock_index_cache").Count(&count)
	if count == 0 {
		task := &models.CronTask{
			Name:        "全球指数缓存",
			CronExpr:    "0 0/5 * * * *", // 每分钟执行一次
			TaskType:    "global_stock_index_cache",
			Target:      "",
			Params:      `{"crawlTimeOut": 30}`,
			Enable:      true,
			Status:      "active",
			Description: "自动缓存全球股票指数数据",
		}
		err := db.Dao.Create(task).Error
		if err != nil {
			log.SugaredLogger.Errorf("创建 global_stock_index_cache 定时任务失败：%v", err)
		} else {
			log.SugaredLogger.Info("创建 global_stock_index_cache 定时任务成功")
		}
	}

}

func initStockDataUS(ctx context.Context) {
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	var v []models.StockInfoUS
	err := json.Unmarshal(stocksBinUS, &v)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}
	log.SugaredLogger.Infof("init stock data us %d", len(v))
	var total int64
	db.Dao.Model(&models.StockInfoUS{}).Count(&total)
	if total != int64(len(v)) {
		for _, item := range v {
			var count int64
			db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", item.Code).Count(&count)
			if count > 0 {
				//log.SugaredLogger.Infof("stock data us %s exist", item.Code)
				continue
			}
			db.Dao.Model(&models.StockInfoUS{}).Create(&item)
		}
	}
}

func initStockDataHK(ctx context.Context) {
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	var v []models.StockInfoHK
	err := json.Unmarshal(stocksBinHK, &v)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}
	log.SugaredLogger.Infof("init stock data hk %d", len(v))
	var total int64
	db.Dao.Model(&models.StockInfoHK{}).Count(&total)
	if total != int64(len(v)) {
		for _, item := range v {
			var count int64
			db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", item.Code).Count(&count)
			if count > 0 {
				//log.SugaredLogger.Infof("stock data hk %s exist", item.Code)
				continue
			}
			db.Dao.Model(&models.StockInfoHK{}).Create(&item)
		}
	}

}

func updateBasicInfo() {
	config := data.GetSettingConfig()
	if config.UpdateBasicInfoOnStart {
		//更新基本信息
		go data.NewStockDataApi().GetStockBaseInfo()
		go data.NewStockDataApi().GetIndexBasic()
	}
}

func initStockData(ctx context.Context) {
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	fields := "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,exchange,list_status,curr_type,enname,delist_date,is_hs"
	log.SugaredLogger.Info("init stock data")
	res := &data.TushareStockBasicResponse{}
	err := json.Unmarshal(stocksBin, res)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}

	for _, item := range res.Data.Items {
		stock := &data.StockBasic{}
		stockData := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			//logger.SugaredLogger.Infof("field: %s", field)
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			stockData[field] = item[idx]
		}
		jsonData, _ := json.Marshal(stockData)
		err := json.Unmarshal(jsonData, stock)
		if err != nil {
			continue
		}
		stock.ID = 0
		var count int64
		db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Count(&count)
		if count > 0 {
			continue
		} else {
			db.Dao.Create(stock)
		}

		//db.Dao.Model(&data.StockBasic{}).FirstOrCreate(stock, &data.StockBasic{TsCode: stock.TsCode}).Where("ts_code = ?", stock.TsCode).Updates(stock)
	}

	//for _, item := range res.Data.Items {
	//	stock := &data.StockBasic{}
	//	stock.Exchange = convertor.ToString(item[0])
	//	stock.IsHs = convertor.ToString(item[1])
	//	stock.Name = convertor.ToString(item[2])
	//	stock.Industry = convertor.ToString(item[3])
	//	stock.ListStatus = convertor.ToString(item[4])
	//	stock.ActName = convertor.ToString(item[5])
	//	stock.ID = uint(item[6].(float64))
	//	stock.CurrType = convertor.ToString(item[7])
	//	stock.Area = convertor.ToString(item[8])
	//	stock.ListDate = convertor.ToString(item[9])
	//	stock.DelistDate = convertor.ToString(item[10])
	//	stock.ActEntType = convertor.ToString(item[11])
	//	stock.TsCode = convertor.ToString(item[12])
	//	stock.Symbol = convertor.ToString(item[13])
	//	stock.Cnspell = convertor.ToString(item[14])
	//	stock.Fullname = convertor.ToString(item[20])
	//	stock.Ename = convertor.ToString(item[21])
	//
	//	var count int64
	//	db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Count(&count)
	//	if count > 0 {
	//		continue
	//	} else {
	//		db.Dao.Create(stock)
	//	}
	//}
}

func checkDir(dir string) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		os.Mkdir(dir, os.ModePerm)
		log.SugaredLogger.Info("create dir: " + dir)
	}
	if BuildKey == "" {
		BuildKey = "cc1e0d684e32f176c56ff1fcf384dcd9"
	}
}

// PanicHandler 捕获 panic 的包装函数（写入本地 crash report，含统一 VersionInfo）。
func PanicHandler() {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		fmt.Printf("Recovered from panic: %v\n", r)
		fmt.Printf("release identity: %s\n", version.LogLine())
		debug.PrintStack()
		if path, err := version.WriteCrashReport(r, stack); err != nil {
			fmt.Printf("crash report write failed: %v\n", err)
		} else {
			fmt.Printf("crash report written: %s\n", path)
		}
	}
}

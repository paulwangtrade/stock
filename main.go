package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	assistantweb "go-stock/ai-assistant-web"
	"go-stock/backend/data"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"go-stock/backend/models"
	"os"
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
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/app.ico
var icon2 []byte

var mainAppCtx context.Context

//go:embed build/screenshot/alipay.jpg
var alipay []byte

//go:embed build/screenshot/wxpay.jpg
var wxpay []byte

//go:embed build/screenshot/鎵爜_鎼滅储鑱斿悎浼犳挱鏍峰紡-鐧借壊鐗?png
var wxgzh []byte

//go:embed build/stock_basic.json
var stocksBin []byte

//go:embed build/stock_base_info_hk.json
var stocksBinHK []byte

//go:embed build/stock_base_info_us.json
var stocksBinUS []byte

// 鍕?go generate 瑕嗙洊 build/bin/data锛歟xe 杩愯鏃舵暟鎹湪 build/bin/data锛岀紪璇戣鐢?scripts/build-windows.ps1

var Version string
var VersionCommit string
var OFFICIAL_STATEMENT string
var BuildKey string

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.SugaredLogger.Error("panic: ", r)
			log.SugaredLogger.Error("stack: ", string(debug.Stack()))
		}
	}()

	checkDir("data")
	data.SponsorDecryptKeyHex = BuildKey
	db.Init("")
	data.InitAnalyzeSentiment()
	// Sync migrate before Wails startup/cron to avoid 9:20 schema races.
	AutoMigrate()

	//db.Dao.Model(&data.Group{}).Where("id = ?", 0).FirstOrCreate(&data.Group{
	//	Name: "榛樿鍒嗙粍",
	//	Sort: 0,
	//})

	log.SugaredLogger.Info("starting...")
	log.SugaredLogger.Infof("version: %s  commit: %s", Version, VersionCommit)
	//log.SugaredLogger.Infof("build key: %s", BuildKey)

	// 绋嬪簭鍚姩鏃堕缂撳瓨涓滆储 Cookie
	//go func() {
	//	cacheCookies("https://push2his.eastmoney.com/api/qt/stock/kline/get")
	//}()

	// Create an instance of the app structure
	app := NewApp()
	AppMenu := menu.NewMenu()
	if IsMacOS() {
		AppMenu.Append(menu.EditMenu())
	}
	//FileMenu := AppMenu.AddSubmenu("璁剧疆")
	//FileMenu.AddText("绐楀彛鍏ㄥ睆", keys.CmdOrCtrl("f"), func(callback *menu.CallbackData) {
	//	runtime.WindowFullscreen(app.ctx)
	//})
	//FileMenu.AddText("绐楀彛杩樺師", keys.Key("Esc"), func(callback *menu.CallbackData) {
	//	runtime.WindowUnfullscreen(app.ctx)
	//})
	//FileMenu.AddText("鏄剧ず鎼滅储妗?, keys.CmdOrCtrl("s"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 1)
	//})
	//FileMenu.AddText("闅愯棌鎼滅储妗?, keys.CmdOrCtrl("d"), func(callbackData *menu.CallbackData) {
	//	runtime.EventsEmit(app.ctx, "showSearch", 0)
	//})
	//FileMenu.AddText("鍒锋柊鏁版嵁", keys.CmdOrCtrl("r"), func(callbackData *menu.CallbackData) {
	//	//runtime.EventsEmit(app.ctx, "refresh", "setting-"+time.Now().Format("2006-01-02 15:04:05"))
	//	runtime.EventsEmit(app.ctx, "refreshFollowList", "refresh-"+time.Now().Format("2006-01-02 15:04:05"))
	//})
	//FileMenu.AddSeparator()

	//if goruntime.GOOS == "windows" {
	//	FileMenu.AddText("闅愯棌鍒版墭鐩樺尯", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) {
	//		runtime.WindowHide(app.ctx)
	//	})
	//}

	//FileMenu.AddText("閫€鍑?, keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
	//	runtime.Quit(app.ctx)
	//})
	log.SugaredLogger.Info("version: " + Version)
	log.SugaredLogger.Info("commit: " + VersionCommit)
	// 鏍规嵁灞忓箷鍒嗚鲸鐜囪嚜閫傚簲绐楀彛灏哄
	width, height, _, _, err := getScreenResolution()
	if err != nil {
		log.SugaredLogger.Error("get screen resolution error")
		// 鑾峰彇澶辫触鏃剁粰涓€涓悎鐞嗙殑榛樿鍊?
		width = 1412
		height = 834
	}

	darkTheme := data.GetSettingConfig().DarkTheme
	backgroundColour := &options.RGBA{R: 255, G: 255, B: 255, A: 1}
	if darkTheme {
		backgroundColour = &options.RGBA{R: 27, G: 38, B: 54, A: 1}
	}

	//frameless := getFrameless()

	// 璁＄畻榛樿绐楀彛澶у皬锛氫紭鍏堜娇鐢ㄤ笂娆′繚瀛樼殑鐢ㄦ埛灏哄锛屽惁鍒欒嚜閫傚簲
	config := data.GetSettingConfig()

	appWidth := config.WindowWidth
	appHeight := config.WindowHeight

	// 鑻ョ敤鎴峰皻鏈皟鏁磋繃绐楀彛鎴栬褰曚负 0锛屽垯鎸夊睆骞曟瘮渚嬬粰涓€涓悎閫傞粯璁ゅ€?
	if appWidth <= 0 || appHeight <= 0 {
		appWidth = width * 5 / 10
		appHeight = height * 5 / 10
	}
	log.SugaredLogger.Info("screen resolution: " + convertor.ToString(width) + "x" + convertor.ToString(height))
	log.SugaredLogger.Info("window size: " + convertor.ToString(appWidth) + "x" + convertor.ToString(appHeight))

	// 浣滀负 go-stock 瀛愮粍浠跺惎鍔ㄧ嫭绔?Web 鏈嶅姟
	// 绔彛榛樿鐢?AI_ASSISTANT_WEB_ADDR 鍐冲畾銆?
	go func() {
		if err := assistantweb.Start(); err != nil {
			log.SugaredLogger.Errorf("ai-assistant-web start error: %v", err)
		}
	}()

	// Create application with options
	err = wails.Run(&options.App{
		Title: "鑲＄エ鍒嗘瀽",
		// 榛樿绐楀彛澶у皬锛氳嚜閫傚簲浣嗕繚鐣欐槑鏄捐竟璺?
		Width:  appWidth,
		Height: appHeight,
		//MinWidth:  minWidth,
		//MinHeight: minHeight,
		// 闄愬埗鏈€澶у昂瀵镐笉瓒呰繃灞忓箷
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
				api.CandidatePoolAssetMiddleware,
				api.RealOrdersAssetMiddleware,
				api.TradePlansAssetMiddleware,
				api.PaperTradingAssetMiddleware, // Phase10-C.2-A: observation + POST /run
				api.OpsTradingDayAssetMiddleware,
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
				Message: "go-stock锛氳偂绁ㄥ垎鏋愨湪 ",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.SugaredLogger.Fatal(err)
	}

}

func cacheCookies(url string) {
	log.SugaredLogger.Info("棰勭紦瀛樹笢璐?Cookie...")
	_, err := data.FetchEastMoneyCookiesViaChromedp("", 3*time.Minute, url)
	if err != nil {
		log.SugaredLogger.Warnf("棰勭紦瀛樹笢璐?Cookie 澶辫触锛?v", err)
	} else {
		log.SugaredLogger.Info("涓滆储 Cookie 棰勭紦瀛樺畬鎴?)
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

// initGlobalStockIndexCacheTask 妫€鏌ュ苟鍒涘缓 global_stock_index_cache 瀹氭椂浠诲姟
func initGlobalStockIndexCacheTask() {
	var count int64
	db.Dao.Model(&models.CronTask{}).Where("task_type = ?", "global_stock_index_cache").Count(&count)
	if count == 0 {
		task := &models.CronTask{
			Name:        "鍏ㄧ悆鎸囨暟缂撳瓨",
			CronExpr:    "0 0/5 * * * *", // 姣忓垎閽熸墽琛屼竴娆?
			TaskType:    "global_stock_index_cache",
			Target:      "",
			Params:      `{"crawlTimeOut": 30}`,
			Enable:      true,
			Status:      "active",
			Description: "鑷姩缂撳瓨鍏ㄧ悆鑲＄エ鎸囨暟鏁版嵁",
		}
		err := db.Dao.Create(task).Error
		if err != nil {
			log.SugaredLogger.Errorf("鍒涘缓 global_stock_index_cache 瀹氭椂浠诲姟澶辫触锛?v", err)
		} else {
			log.SugaredLogger.Info("鍒涘缓 global_stock_index_cache 瀹氭椂浠诲姟鎴愬姛")
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
		//鏇存柊鍩烘湰淇℃伅
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

// PanicHandler 鎹曡幏 panic 鐨勫寘瑁呭嚱鏁?
func PanicHandler() {
	if r := recover(); r != nil {
		fmt.Printf("Recovered from panic: %v\n", r)
		debug.PrintStack()
	}
}

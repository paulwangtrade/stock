//go:build darwin
// +build darwin

package main

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"log"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/gen2brain/beeep"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startup 在应用程序启动时调用
func (a *App) startup(ctx context.Context) {
	defer PanicHandler()
	runtime.EventsOn(ctx, "frontendError", handleFrontendError)
	logger.SugaredLogger.Infof("Version:%s", Version)
	// Perform your setup here
	a.ctx = ctx
	a.bindLoadingProgressEvents()
	a.reportStartupProgress()
	a.startTradingEventBridge(ctx)
	a.startTaskScheduler()

	// 应用启动时自动创建已启用的定时任务
	a.InitCronTasks()
	a.InitPaperMarginDayJobs()
	a.InitPaperOpenBuyJobs()
	a.InitAfterClosePlanJobs()
	a.InitPaperTradingJobs()
	a.InitMorningPreparationJobs()
	a.InitTradingAutomationJobs()
	a.InitJobRuntimeReliability()
	a.InitStockStrategies()
	a.emitLoadingProgress(55, "正在初始化组件...")

	// 监听设置更新事件
	runtime.EventsOn(ctx, "updateSettings", func(optionalData ...interface{}) {
		a.handleUpdateSettings(ctx)
	})

	// 创建 macOS 托盘
	go func() {
		// 使用 Beeep 库替代 Windows 的托盘库
		err := beeep.Notify("go-stock", "应用程序已启动", "")
		if err != nil {
			log.Fatalf("系统通知失败: %v", err)
		}
	}()
	go setUpScreen(a)
	logger.SugaredLogger.Infof(" application startup Version:%s", Version)
}

func setUpScreen(a *App) {
	screens, _ := runtime.ScreenGetAll(a.ctx)
	if len(screens) == 0 {
		return
	}
	screen := screens[0]
	sw, sh := screen.Width, screen.Height

	// macOS 菜单栏 + Dock 留出空间
	topBarHeight := 22
	dockHeight := 56
	verticalMargin := topBarHeight + dockHeight

	// 设置窗口为屏幕 80% 宽 × 可用高度 90%
	w := int(float64(sw) * 0.8)
	h := int(float64(sh-verticalMargin) * 0.9)

	runtime.WindowSetSize(a.ctx, w, h)
	runtime.WindowCenter(a.ctx)
}

// OnSecondInstanceLaunch 处理第二实例启动时的通知
func OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	err := beeep.Notify("go-stock", "程序已经在运行了", "")
	if err != nil {
		logger.SugaredLogger.Error(err)
	}
	time.Sleep(time.Second * 3)
}

func MonitorStockPrices(a *App) {
	started := time.Now()
	defer emitMonitorPerf(a, started)

	if !watchlistPricePoll.Allow(false) {
		return
	}

	// 检查是否至少有一个市场开市
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	// 如果所有市场都不在交易时间，则提前返回
	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过价格监控")
		watchlistPricePoll.MarkSuccess()
		return
	}

	logger.SugaredLogger.Debugf("市场状态 - A股: %v, 港股: %v, 美股: %v", isAStockOpen, isHKStockOpen, isUSStockOpen)

	dest := &[]data.FollowedStock{}
	db.Dao.Model(&data.FollowedStock{}).Find(dest)
	total := float64(0)

	// P1：异步预热行业/板块，后续 tick 走缓存
	warmCodes := make([]string, 0, len(*dest))
	for _, f := range *dest {
		warmCodes = append(warmCodes, f.StockCode)
	}
	data.NewStockDataApi().WarmIndustrySectorCacheAsync(warmCodes...)

	// 股票信息处理逻辑
	stockInfos, err := GetStockInfos(*dest...)
	if err != nil {
		watchlistPricePoll.MarkFailure()
		logger.SugaredLogger.Errorf("MonitorStockPrices fetch error: %v", err)
		return
	}
	watchlistPricePoll.MarkSuccess()
	for _, stockInfo := range *stockInfos {
		if strutil.HasPrefixAny(stockInfo.Code, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}

		total += stockInfo.ProfitAmountToday
		price, _ := convertor.ToFloat(stockInfo.Price)

		if stockInfo.PrePrice != price {
			go runtime.EventsEmit(a.ctx, "stock_price", stockInfo)
		}
	}

	// 计算总收益并更新状态
	if total != 0 {
		// 使用通知替代 systray 更新 Tooltip
		title := "go-stock " + time.Now().Format(time.DateTime) + fmt.Sprintf("  %.2f¥", total)

		// 发送通知显示实时数据
		err := beeep.Notify("go-stock", title, "")
		if err != nil {
			logger.SugaredLogger.Errorf("发送通知失败: %v", err)
		}
	}

	// 触发实时利润事件
	go runtime.EventsEmit(a.ctx, "realtime_profit", fmt.Sprintf("  %.2f", total))
}

// onReady 在应用程序准备好时调用
func onReady(a *App) {
	// 初始化操作
	logger.SugaredLogger.Infof("onReady")

	// 使用 Beeep 发送通知
	err := beeep.Notify("go-stock", "应用程序已准备就绪", "")
	if err != nil {
		log.Fatalf("系统通知失败: %v", err)
	}

	// 显示应用窗口
	runtime.WindowShow(a.ctx)

	// 在 macOS 上没有系统托盘图标菜单，通常我们通过通知或其他方式提供与用户交互的界面
}

// beforeClose 在应用程序关闭前调用，显示确认对话框
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	defer PanicHandler()

	// 记录当前窗口大小，供下次启动时还原
	if a.ctx != nil {
		w, h := runtime.WindowGetSize(ctx)
		logger.SugaredLogger.Infof(" window size: %dx%d", w, h)
		if w > 0 && h > 0 {
			cfg := data.GetSettingConfig()
			cfg.WindowWidth = w
			cfg.WindowHeight = h
			data.UpdateConfig(cfg)
			logger.SugaredLogger.Infof("save window size: %dx%d", w, h)
		}
	}

	// 在 macOS 上使用 MessageDialog 显示确认窗口
	dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:         runtime.QuestionDialog,
		Title:        "股票分析",
		Message:      "确定关闭吗？",
		Buttons:      []string{"确定", "取消"},
		Icon:         icon2,
		CancelButton: "取消",
	})

	if err != nil {
		logger.SugaredLogger.Errorf("dialog error:%s", err.Error())
		return false
	}

	logger.SugaredLogger.Debugf("dialog:%s", dialog)
	if dialog == "取消" {
		return true // 如果选择了取消，不关闭应用
	} else {
		// 在 macOS 上应用退出时执行清理工作
		a.cron.Stop() // 停止定时任务
		a.stopTaskScheduler()
		return false // 如果选择了确定，继续关闭应用
	}
}

func getFrameless() bool {
	return false
}

func getScreenResolution() (int, int, int, int, error) {
	//user32 := syscall.NewLazyDLL("user32.dll")
	//getSystemMetrics := user32.NewProc("GetSystemMetrics")
	//
	//width, _, _ := getSystemMetrics.Call(0)
	//height, _, _ := getSystemMetrics.Call(1)

	return int(1200), int(800), 0, 0, nil
}

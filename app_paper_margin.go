package main

import (
	"context"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution"
	"go-stock/backend/logger"
	"go-stock/backend/risk"
	"go-stock/backend/scheduler"
)

// paperMarginService 是两融模拟的单一集成点；首期仅使用本地数据库，不连接真实券商。
func paperMarginService() *execution.PaperMarginService {
	return execution.NewPaperMarginService(db.Dao)
}

func (a *App) paperMarginContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// GetPaperMarginSnapshot 返回账户、负债、账本和统一风控指标快照。
func (a *App) GetPaperMarginSnapshot(accountID uint, marks []execution.PositionMark) (*execution.Snapshot, error) {
	return paperMarginService().Snapshot(a.paperMarginContext(), accountID, marks)
}

// SubmitPaperMarginOrder 提交并立即成交本地模拟订单；拒单也会保留原因码。
func (a *App) SubmitPaperMarginOrder(req execution.SubmitRequest) (*data.PaperMarginOrder, risk.RiskDecision, error) {
	return paperMarginService().Submit(a.paperMarginContext(), req)
}

// AccruePaperMarginInterest 按 ACT/365 计提模拟息费；asOf 为空时使用当前时间。
func (a *App) AccruePaperMarginInterest(accountID uint, asOf string) (*execution.AccrualResult, error) {
	at := time.Now()
	if asOf != "" {
		parsed, err := time.Parse(time.RFC3339, asOf)
		if err != nil {
			return nil, err
		}
		at = parsed
	}
	return paperMarginService().AccrueInterest(a.paperMarginContext(), accountID, at)
}

// ScanPaperMarginRisk 执行模拟警戒/平仓线巡检并返回本次新生成的事件。
func (a *App) ScanPaperMarginRisk(accountID uint, marks []execution.PositionMark) ([]data.PaperMarginRiskEvent, error) {
	return paperMarginService().ScanRisk(a.paperMarginContext(), accountID, marks, time.Now())
}

// RunPaperMarginRiskScan 保留前端统一命名，行为与 ScanPaperMarginRisk 一致。
func (a *App) RunPaperMarginRiskScan(accountID uint, marks []execution.PositionMark) ([]data.PaperMarginRiskEvent, error) {
	return a.ScanPaperMarginRisk(accountID, marks)
}

// ConfigurePaperMarginAccount 配置模拟授信、息率和风险阈值；阈值不代表监管标准。
func (a *App) ConfigurePaperMarginAccount(config execution.AccountConfig) (*data.PaperMarginAccount, error) {
	return paperMarginService().ConfigureAccount(a.paperMarginContext(), config)
}

// UpsertPaperBorrowPool 配置本地模拟券源、折算率和保证金比例。
func (a *App) UpsertPaperBorrowPool(pool data.PaperBorrowPool) (*data.PaperBorrowPool, error) {
	return paperMarginService().UpsertBorrowPool(a.paperMarginContext(), pool)
}

// startTaskScheduler 启动统一 P0–P4 队列，并登记已知任务优先级约定。
func (a *App) startTaskScheduler() {
	scheduler.Default.Start(a.paperMarginContext())
	scheduler.Default.Register(scheduler.TaskMeta{Name: "paper_margin_accrue", Priority: scheduler.P0ExecSafety})
	scheduler.Default.Register(scheduler.TaskMeta{Name: "paper_margin_scan", Priority: scheduler.P0ExecSafety})
	scheduler.Default.Register(scheduler.TaskMeta{Name: "MonitorStockPrices", Priority: scheduler.P1PositionQuote})
	scheduler.Default.Register(scheduler.TaskMeta{Name: "watchlistScan", Priority: scheduler.P3WatchlistScan, Cancelable: true})
}

func (a *App) stopTaskScheduler() {
	scheduler.Default.Stop()
}

func (a *App) submitPaperMarginDayJob(name string, run func(context.Context) error) {
	accepted, err := scheduler.Default.Submit(scheduler.Task{
		Name:     name,
		Priority: scheduler.P0ExecSafety,
		Fn: func(ctx context.Context) error {
			return run(ctx)
		},
	})
	if err != nil {
		logger.SugaredLogger.Errorf("submit %s: %s", name, err.Error())
		return
	}
	if !accepted {
		logger.SugaredLogger.Infof("%s skipped: already queued or running", name)
	}
}

// InitPaperMarginDayJobs 注册模拟两融日终任务：收盘后计息 + 警戒/平仓巡检。
// 默认工作日 15:10 / 15:20（含秒）；仅本地模拟，不连接真实券商。
// cron 只负责触发时刻，实际执行经 scheduler P0 队列，并传入可取消的 ctx。
func (a *App) InitPaperMarginDayJobs() {
	if a.cron == nil {
		return
	}
	const accrueKey = "paper_margin_accrue"
	const scanKey = "paper_margin_scan"
	if _, exists := a.getCronEntry(accrueKey); !exists {
		id, err := a.cron.AddFunc("0 10 15 * * 1-5", func() {
			defer PanicHandler()
			a.submitPaperMarginDayJob(accrueKey, a.runPaperMarginDayAccrue)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperMarginDayJobs accrue: %s", err.Error())
		} else {
			a.setCronEntry(accrueKey, id)
			logger.SugaredLogger.Infof("paper margin accrue cron registered: %v", id)
		}
	}
	if _, exists := a.getCronEntry(scanKey); !exists {
		id, err := a.cron.AddFunc("0 20 15 * * 1-5", func() {
			defer PanicHandler()
			a.submitPaperMarginDayJob(scanKey, a.runPaperMarginDayScan)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperMarginDayJobs scan: %s", err.Error())
		} else {
			a.setCronEntry(scanKey, id)
			logger.SugaredLogger.Infof("paper margin risk scan cron registered: %v", id)
		}
	}
}

func (a *App) runPaperMarginDayAccrue(ctx context.Context) error {
	if ctx == nil {
		ctx = a.paperMarginContext()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	svc := paperMarginService()
	var accounts []data.PaperMarginAccount
	if err := db.Dao.WithContext(ctx).Find(&accounts).Error; err != nil {
		logger.SugaredLogger.Errorf("paper margin day accrue list: %s", err.Error())
		return err
	}
	now := time.Now()
	for _, account := range accounts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if result, err := svc.AccrueInterest(ctx, account.AccountID, now); err != nil {
			logger.SugaredLogger.Errorf("paper margin accrue account=%d: %s", account.AccountID, err.Error())
		} else if result != nil && (result.FinanceInterest > 0 || result.SecuritiesFee > 0) {
			logger.SugaredLogger.Infof("paper margin accrued account=%d finance=%.4f securities=%.4f days=%.4f",
				result.AccountID, result.FinanceInterest, result.SecuritiesFee, result.Days)
		}
	}
	return ctx.Err()
}

func (a *App) runPaperMarginDayScan(ctx context.Context) error {
	if ctx == nil {
		ctx = a.paperMarginContext()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	svc := paperMarginService()
	var accounts []data.PaperMarginAccount
	if err := db.Dao.WithContext(ctx).Find(&accounts).Error; err != nil {
		logger.SugaredLogger.Errorf("paper margin day scan list: %s", err.Error())
		return err
	}
	for _, account := range accounts {
		if err := ctx.Err(); err != nil {
			return err
		}
		events, err := svc.ScanRisk(ctx, account.AccountID, nil, time.Now())
		if err != nil {
			logger.SugaredLogger.Errorf("paper margin scan account=%d: %s", account.AccountID, err.Error())
			continue
		}
		for _, event := range events {
			logger.SugaredLogger.Warnf("paper margin risk event account=%d level=%s code=%s ratio=%.4f",
				event.AccountID, event.Level, event.ReasonCode, event.MaintenanceRatio)
		}
	}
	return ctx.Err()
}

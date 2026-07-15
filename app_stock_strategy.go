package main

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
)

func stockStrategyCronKey(id uint) string {
	return "stock_strategy_" + convertor.ToString(id)
}

// InitStockStrategies 为已启用的选股策略注册定时任务
func (a *App) InitStockStrategies() {
	list := data.NewStockStrategyApi().GetAllEnabled()
	for _, s := range list {
		strategy := s
		if strategy.CronExpr == "" {
			continue
		}
		entryID, err := a.cron.AddFunc(strategy.CronExpr, func() {
			_, err := a.RunStockStrategy(strategy.ID)
			if err != nil {
				logger.SugaredLogger.Errorf("策略定时执行失败 %s: %v", strategy.Name, err)
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("注册策略定时失败 %s: %v", strategy.Name, err)
			continue
		}
		a.setCronEntry(stockStrategyCronKey(strategy.ID), entryID)
	}
}

func (a *App) registerStockStrategyCron(s *models.StockStrategy) {
	if entryID, ok := a.getCronEntry(stockStrategyCronKey(s.ID)); ok {
		a.cron.Remove(entryID)
	}
	if !s.Enable || s.CronExpr == "" {
		return
	}
	strategyID := s.ID
	cronExpr := s.CronExpr
	entryID, err := a.cron.AddFunc(cronExpr, func() {
		_, err := a.RunStockStrategy(strategyID)
		if err != nil {
			logger.SugaredLogger.Errorf("策略定时执行失败 id=%d: %v", strategyID, err)
		}
	})
	if err != nil {
		logger.SugaredLogger.Errorf("注册策略定时失败: %v", err)
		return
	}
	a.setCronEntry(stockStrategyCronKey(s.ID), entryID)
}

func (a *App) unregisterStockStrategyCron(id uint) {
	if entryID, ok := a.getCronEntry(stockStrategyCronKey(id)); ok {
		a.cron.Remove(entryID)
	}
}

func (a *App) CreateStockStrategy(s *models.StockStrategy) string {
	if err := data.NewStockStrategyApi().Create(s); err != nil {
		return "创建失败: " + err.Error()
	}
	if s.Enable {
		a.registerStockStrategyCron(s)
	}
	return "创建成功"
}

func (a *App) UpdateStockStrategy(s *models.StockStrategy) string {
	if err := data.NewStockStrategyApi().Update(s); err != nil {
		return "更新失败: " + err.Error()
	}
	a.registerStockStrategyCron(s)
	return "更新成功"
}

func (a *App) DeleteStockStrategy(id uint) string {
	a.unregisterStockStrategyCron(id)
	if err := data.NewStockStrategyApi().Delete(id); err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (a *App) GetStockStrategyList(query *models.StockStrategyQuery) *models.StockStrategyPageResp {
	return data.NewStockStrategyApi().List(query)
}

func (a *App) GetStockStrategyByID(id uint) *models.StockStrategy {
	s, err := data.NewStockStrategyApi().GetByID(id)
	if err != nil {
		return nil
	}
	return s
}

func (a *App) GetStockStrategySummary(id uint) string {
	s, err := data.NewStockStrategyApi().GetByID(id)
	if err != nil {
		return ""
	}
	return data.NewStockStrategyApi().SummaryText(s)
}

func (a *App) RunStockStrategy(id uint) (*models.StockStrategyRunView, error) {
	s, err := data.NewStockStrategyApi().GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("策略不存在")
	}
	view := data.NewStockStrategyApi().RunStrategy(s)
	return view, nil
}

func (a *App) GetStockStrategyRunList(query *models.StockStrategyRunQuery) *models.StockStrategyRunPageResp {
	return data.NewStockStrategyApi().ListRuns(query)
}

func (a *App) GetStockStrategyRunDetail(runID uint) *models.StockStrategyRunView {
	run, err := data.NewStockStrategyApi().GetRunByID(runID)
	if err != nil {
		return nil
	}
	var view models.StockStrategyRunView
	if err := json.Unmarshal([]byte(run.ResultJSON), &view); err != nil {
		return &models.StockStrategyRunView{
			RunID:   run.ID,
			Code:    -1,
			Message: "解析结果失败",
		}
	}
	view.RunID = run.ID
	if !run.CreatedAt.IsZero() {
		view.RunAt = run.CreatedAt.Format("2006-01-02 15:04:05")
	}
	return &view
}

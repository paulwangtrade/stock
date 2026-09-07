package tradeplanorigin

import (
	"fmt"
	"strings"

	"go-stock/backend/models"
)

type sourceReasonInput struct {
	SignalTag    string
	StrategyName string
	SignalTime   string
	PoolReason   string
	PoolSource   string
	StatusText   string
}

type selectionReasonInput struct {
	PoolRank    int
	PoolScore   float64
	PlanScore   float64
	MaxNames    int
	ItemStatus  string
	RiskCode    string
	RiskMessage string
	HasPoolItem bool
	InPlan      bool
}

func buildSourceReason(in sourceReasonInput) string {
	tag := strings.TrimSpace(in.SignalTag)
	strategy := strings.TrimSpace(in.StrategyName)
	signalTime := strings.TrimSpace(in.SignalTime)
	poolReason := strings.TrimSpace(in.PoolReason)
	poolSource := strings.TrimSpace(in.PoolSource)
	statusText := strings.TrimSpace(in.StatusText)

	if tag != "" && strategy != "" {
		if date := signalDateLabel(signalTime); date != "" {
			return fmt.Sprintf("%s 扫描命中「%s」信号（%s）", strategy, tag, date)
		}
		return fmt.Sprintf("%s 扫描命中「%s」信号", strategy, tag)
	}
	if tag != "" {
		return fmt.Sprintf("收盘扫描命中「%s」信号", tag)
	}
	if statusText != "" && strategy != "" {
		return fmt.Sprintf("%s：%s", strategy, statusText)
	}
	if poolSource == models.CandidatePoolSourceFollow || poolReason == models.CandidatePoolSourceFollow {
		return "自选股关注列表兜底入选（无当日信号增强）"
	}
	if poolSource == models.CandidatePoolSourceStrategyRun || poolReason == models.CandidatePoolSourceStrategyRun {
		return "策略选股结果入选（当日无匹配信号快照）"
	}
	if poolReason == models.CandidatePoolSourceStrategyRun {
		return "策略流水线入选"
	}
	if poolReason == models.CandidatePoolSourceFollow {
		return "关注列表兜底入选"
	}
	if strategy != "" {
		return fmt.Sprintf("%s 策略入选", strategy)
	}
	return Missing
}

func buildSelectionReason(in selectionReasonInput) string {
	if !in.HasPoolItem && in.PlanScore <= 0 && strings.TrimSpace(in.ItemStatus) == "" {
		return Missing
	}

	rank := in.PoolRank
	score := in.PoolScore
	if score <= 0 && in.PlanScore > 0 {
		score = in.PlanScore
	}
	scoreLabel := formatScore(score)

	status := strings.TrimSpace(in.ItemStatus)
	riskCode := strings.TrimSpace(in.RiskCode)
	maxNames := in.MaxNames

	if status == models.TradePlanItemSkipped {
		if riskCode != "" {
			if rank > 0 {
				return fmt.Sprintf("候选池排名第 %d，因风控 %s 未纳入执行", rank, riskCode)
			}
			return fmt.Sprintf("因风控 %s 未纳入执行", riskCode)
		}
		if strings.TrimSpace(in.RiskMessage) != "" {
			return fmt.Sprintf("未纳入执行：%s", strings.TrimSpace(in.RiskMessage))
		}
	}

	if rank > 0 && maxNames > 0 && rank > maxNames {
		return fmt.Sprintf("候选池排名第 %d，超出计划名额上限未选入", rank)
	}

	if rank > 0 && in.InPlan {
		if maxNames > 0 {
			if scoreLabel != Missing {
				return fmt.Sprintf("候选池排名第 %d，综合分 %s，纳入计划（上限 %d 只）", rank, scoreLabel, maxNames)
			}
			return fmt.Sprintf("候选池排名第 %d，纳入计划（上限 %d 只）", rank, maxNames)
		}
		if scoreLabel != Missing {
			return fmt.Sprintf("候选池排名第 %d，综合分 %s，纳入计划", rank, scoreLabel)
		}
		return fmt.Sprintf("候选池排名第 %d，纳入计划", rank)
	}

	if scoreLabel != Missing && in.InPlan {
		return fmt.Sprintf("综合分 %s，纳入计划", scoreLabel)
	}
	return Missing
}

func signalDateLabel(signalTime string) string {
	signalTime = strings.TrimSpace(signalTime)
	if signalTime == "" {
		return ""
	}
	if len(signalTime) >= 10 {
		return strings.ReplaceAll(signalTime[:10], "-", "-")
	}
	return signalTime
}

package qualitygate

import (
	"time"

	"go-stock/backend/models"
)

// Evaluate runs MVP rules E1/I1/G1/M1/P1 and aggregates overall severity.
// It never mutates the plan, positions, or market data.
func Evaluate(in Input) Result {
	cfg := mergeConfig(in.Config)

	findings := make([]Finding, 0, 16)
	findings = append(findings, evalE1(in.Plan, cfg)...)
	findings = append(findings, evalI1(in.Plan, in.Positions)...)
	findings = append(findings, evalG1(in.Plan, in.MarketData, cfg)...)
	findings = append(findings, evalM1(in.Plan, in.MarketData)...)
	findings = append(findings, evalP1(in.Plan, in.MarketData, cfg)...)

	return aggregate(in.Plan, findings)
}

func mergeConfig(c Config) Config {
	out := DefaultConfig()
	if c.SectorWarnAmountShare > 0 {
		out.SectorWarnAmountShare = c.SectorWarnAmountShare
	}
	if c.SectorWarnNameCount > 0 {
		out.SectorWarnNameCount = c.SectorWarnNameCount
	}
	if c.MinCompletenessRatio > 0 {
		out.MinCompletenessRatio = c.MinCompletenessRatio
	}
	// bool zero-value cannot mean "unset"; MVP always requires entry price.
	out.RequireEntryPrice = true
	return out
}

// EvaluateTradePlan wraps models.TradePlan for callers.
func EvaluateTradePlan(plan *models.TradePlan, positions []AccountPosition, market MarketDataSnapshot, cfg Config) Result {
	if plan == nil {
		return Result{
			Passed:    false,
			Severity:  SeverityFAIL,
			CheckedAt: time.Now(),
			Findings: []Finding{{
				Passed:   false,
				Severity: SeverityFAIL,
				RuleCode: "QG-META",
				Code:     "PLAN_NIL",
				Message:  "trade plan is nil",
			}},
		}
	}
	return Evaluate(Input{
		Plan:       PlanFromModel(plan),
		Positions:  positions,
		MarketData: market,
		Config:     cfg,
	})
}

// PlanFromModel maps models.TradePlan → PlanView.
func PlanFromModel(plan *models.TradePlan) PlanView {
	pv := PlanView{
		ID:             plan.ID,
		TradeDate:      plan.TradeDate,
		PlanVersion:    plan.PlanVersion,
		Status:         plan.Status,
		AmountPerStock: plan.AmountPerStock,
	}
	for _, it := range plan.Items {
		pv.Items = append(pv.Items, ItemView{
			StockCode:    it.StockCode,
			StockName:    it.StockName,
			Side:         it.Side,
			Priority:     it.Priority,
			TargetAmount: it.TargetAmount,
			LimitPrice:   it.LimitPrice,
			TargetVolume: it.TargetVolume,
		})
	}
	return pv
}

func aggregate(plan PlanView, findings []Finding) Result {
	sev := SeverityPASS
	blockers := make([]Finding, 0)
	warnings := make([]Finding, 0)
	for _, f := range findings {
		if f.Passed {
			continue
		}
		switch f.Severity {
		case SeverityFAIL:
			sev = SeverityFAIL
			blockers = append(blockers, f)
		case SeverityWARN:
			if sev != SeverityFAIL {
				sev = SeverityWARN
			}
			warnings = append(warnings, f)
		}
	}
	return Result{
		Passed:    sev == SeverityPASS,
		Severity:  sev,
		PlanID:    plan.ID,
		TradeDate: plan.TradeDate,
		CheckedAt: time.Now(),
		Findings:  findings,
		Blockers:  blockers,
		Warnings:  warnings,
	}
}

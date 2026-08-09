package main

import (
	"context"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/riskreport"
	"go-stock/backend/usagemetrics"
)

// AdvancedRiskReportDTO is the Wails-facing RiskReport for Observation UI.
type AdvancedRiskReportDTO = riskreport.RiskReport

// BuildAdvancedRiskReport builds a read-only Advanced Risk Report (Feature AdvancedRisk).
// Does not affect trading execution.
func (a *App) BuildAdvancedRiskReport(tier string, tradeDate string) *AdvancedRiskReportDTO {
	user := shellUserForTier(tier)
	_ = entitlement.Default().EnsureTierDefaults(user)
	_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedRisk, usagemetrics.EventOpened, map[string]string{
		"scene":  "advanced_risk_report",
		"source": "wails",
	})
	out, err := riskreport.Default().Build(context.Background(), riskreport.BuildRequest{
		UserID:    user.ID,
		Tier:      string(user.Tier),
		TradeDate: tradeDate,
	})
	if err != nil || out == nil {
		return &riskreport.RiskReport{
			Status:      riskreport.StatusFailed,
			SchemaVersion: riskreport.SchemaVersion,
			Disclaimers: riskreportDisclaimers(),
			Score:       riskreport.RiskScore{Band: riskreport.BandUnknown, ByDimension: map[string]int{}},
		}
	}
	return out
}

func riskreportDisclaimers() []string {
	return []string{
		"本报告为只读风险观察，不构成投资建议或交易指令，不会修改或触发任何成交。",
	}
}

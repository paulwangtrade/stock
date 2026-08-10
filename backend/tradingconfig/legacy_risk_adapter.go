package tradingconfig

import "go-stock/backend/data"

// LegacyRiskConfigAdapter loads Plan Risk thresholds from the existing
// paper_open_buy.json store without changing its on-disk format.
//
// Phase6.5-B: authoritative Risk source remains GetPaperOpenBuyConfig();
// this adapter only re-exposes the same values for Provider consumers.
type LegacyRiskConfigAdapter struct{}

// Load returns RiskView identical to fields used by loadPlanFilterContext.
func (a *LegacyRiskConfigAdapter) Load() RiskView {
	if a == nil {
		a = &LegacyRiskConfigAdapter{}
	}
	cfg := data.GetPaperOpenBuyConfig()
	return RiskView{
		Enabled:                  cfg.EnableRiskFilter,
		MarketLevel:              cfg.PlanMarketLevel,
		BlockNewEntriesOnDefense: cfg.BlockNewEntriesOnDefense,
		MaxGrossExposurePct:      cfg.MaxGrossExposurePct,
		MaxSingleNamePct:         cfg.MaxSingleNamePct,
		MaxDailyLossPct:          cfg.MaxDailyLossPct,
		CurrentDailyPnlPct:       cfg.CurrentDailyPnlPct,
		Source:                   SourceLegacyPaperConfig,
	}
}

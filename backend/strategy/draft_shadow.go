package strategy

import (
	"go-stock/backend/logger"
	"go-stock/backend/providershadow"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"
)

// afterCloseDraftShadow is the G.12 sidecar. Production Default() has Enabled=false.
var afterCloseDraftShadow = providershadow.Default()

func observeAfterCloseDraftShadow(sel *selection.CandidateSelectionResult, amount float64, tradeDate string) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.SugaredLogger.Warnf("provider shadow panic ignored: %v", rec)
		}
	}()
	rt := afterCloseDraftShadow
	if rt == nil {
		return
	}
	// Production: ProviderShadow.Enabled=false and Default() Runtime.Enabled=false.
	// Tests may inject a Runtime with Enabled=true without flipping compile config.
	if !tradingconfig.ProviderShadowEnabled() && !rt.Enabled() {
		return
	}
	_ = rt.Observe(providershadow.ObserveInput{
		Trigger:       providershadow.TriggerAfterCloseDraft,
		Selection:     sel,
		UniformAmount: amount,
		TradeDate:     tradeDate,
	})
}

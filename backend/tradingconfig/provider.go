package tradingconfig

import (
	"sync"

	"go-stock/backend/logger"
)

// Provider is the unified TradingConfig read entry for business modules.
// Phase6.5 / H.4 always resolves via LegacyAdapter (no trading_config store yet).
type Provider interface {
	Resolve() Resolved
	OpenBuyAmountPerStock() float64
	EnablePaperOpenBuy() bool
	AfterCloseEnabled() bool
	PaperTradingEnabled() bool
	PaperInitialCash() float64
	FillMode() string
	Risk() RiskView
	LogInitialized()
	LogRiskInitialized()
}

type provider struct {
	adapter  *LegacyAdapter
	once     sync.Once
	riskOnce sync.Once
}

var (
	defaultProvider Provider = &provider{adapter: &LegacyAdapter{}}
)

// Default returns the process-wide TradingConfig Provider.
func Default() Provider {
	return defaultProvider
}

// SetDefaultProvider replaces the process provider (tests only).
func SetDefaultProvider(p Provider) {
	if p == nil {
		defaultProvider = &provider{adapter: &LegacyAdapter{}}
		return
	}
	defaultProvider = p
}

// ResetDefaultProvider restores the legacy-backed default provider (tests).
func ResetDefaultProvider() {
	defaultProvider = &provider{adapter: &LegacyAdapter{}}
}

func (p *provider) Resolve() Resolved {
	if p == nil {
		p = &provider{adapter: &LegacyAdapter{}}
	}
	if p.adapter == nil {
		p.adapter = &LegacyAdapter{}
	}
	return p.adapter.Load()
}

// OpenBuyAmountPerStock returns the fixed plan amount used by Draft generation.
// Behavior matches legacy GetPaperOpenBuyConfig().OpenBuyAmountPerStock with <=0 → 100000.
func (p *provider) OpenBuyAmountPerStock() float64 {
	return p.Resolve().Position.MaxPositionAmount
}

// EnablePaperOpenBuy reports Track-A open-buy / EnableExecute switch.
func (p *provider) EnablePaperOpenBuy() bool {
	return p.Resolve().Execution.EnablePaperOpenBuy
}

// AfterCloseEnabled reports whether the after-close cron workflow should run.
func (p *provider) AfterCloseEnabled() bool {
	return p.Resolve().Workflow.AfterCloseEnabled
}

// PaperTradingEnabled reports Track-B MVP enablePaperTrading.
func (p *provider) PaperTradingEnabled() bool {
	return p.Resolve().PaperMVP.EnablePaperTrading
}

// PaperInitialCash returns Track-B initial cash (default 1e6).
func (p *provider) PaperInitialCash() float64 {
	return p.Resolve().PaperMVP.InitialCash
}

// FillMode returns Track-B fill cron mode ("A" or "B").
func (p *provider) FillMode() string {
	return p.Resolve().PaperMVP.FillMode
}

// Risk returns Plan Risk thresholds (Phase6.5-B: legacy_paper_config).
func (p *provider) Risk() RiskView {
	return p.Resolve().Risk
}

// LogInitialized emits a one-shot observability log of resolved sources.
func (p *provider) LogInitialized() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		r := p.Resolve()
		logger.SugaredLogger.Infof(
			"TradingConfig Provider initialized source_position=%s sizing=%s fixed_amount=%.0f enable_paper_open_buy=%v source_after_close=%s after_close_enabled=%v source_mvp=%s enable_paper_trading=%v fill_mode=%s initial_cash=%.0f source_risk=%s max_single_name_pct=%.2f max_gross_exposure_pct=%.2f source_automation=%s automation_present=%v",
			r.Position.Source,
			r.Position.SizingMethod,
			r.Position.MaxPositionAmount,
			r.Execution.EnablePaperOpenBuy,
			r.Workflow.Source,
			r.Workflow.AfterCloseEnabled,
			r.PaperMVP.Source,
			r.PaperMVP.EnablePaperTrading,
			r.PaperMVP.FillMode,
			r.PaperMVP.InitialCash,
			r.Risk.Source,
			r.Risk.MaxSingleNamePct,
			r.Risk.MaxGrossExposurePct,
			r.Automation.Source,
			r.Automation.Present,
		)
	})
}

// LogRiskInitialized emits a one-shot Risk-domain source log.
func (p *provider) LogRiskInitialized() {
	if p == nil {
		return
	}
	p.riskOnce.Do(func() {
		r := p.Risk()
		logger.SugaredLogger.Infof(
			"RiskConfig initialized source=%s enable=%v market_level=%d block_new_entries=%v max_single_name_pct=%.0f%% max_gross_exposure_pct=%.0f%% max_daily_loss_pct=%.2f current_daily_pnl_pct=%.4f",
			r.Source,
			r.Enabled,
			r.MarketLevel,
			r.BlockNewEntriesOnDefense,
			r.MaxSingleNamePct*100,
			r.MaxGrossExposurePct*100,
			r.MaxDailyLossPct,
			r.CurrentDailyPnlPct,
		)
	})
}

// LogInitialized ensures the default provider emits its one-shot init log.
func LogInitialized() {
	Default().LogInitialized()
}

// LogRiskInitialized ensures the default provider emits its Risk init log.
func LogRiskInitialized() {
	Default().LogRiskInitialized()
}

package papertrading

import "go-stock/backend/tradingconfig"

func init() {
	tradingconfig.RegisterPaperMVPLoader(func() tradingconfig.PaperMVPView {
		c := GetConfig()
		return tradingconfig.PaperMVPView{
			EnablePaperTrading: c.EnablePaperTrading,
			InitialCash:        c.InitialCash,
			FillMode:           c.FillMode,
			PositionSizerMode:  NormalizePositionSizerMode(c.PositionSizerMode),
			PortfolioAware:     portfolioAwareViewFromConfig(c.PortfolioAware),
			Source:             tradingconfig.SourceLegacyPaperMVP,
		}
	})
}

// IsEnabled reports whether Paper Trading MVP is switched on (via TradingConfig Provider).
func IsEnabled() bool {
	return tradingconfig.Default().PaperTradingEnabled()
}

// EffectiveFillMode returns the exclusive fill cron mode from TradingConfig Provider.
func EffectiveFillMode() string {
	return NormalizeFillMode(tradingconfig.Default().FillMode())
}

func portfolioAwareViewFromConfig(in *PortfolioAwareConfig) tradingconfig.PortfolioAwareView {
	if in == nil {
		return tradingconfig.PortfolioAwareView{}
	}
	return tradingconfig.PortfolioAwareView{
		MaxExposure:             in.MaxExposure,
		MaxSinglePositionWeight: in.MaxSinglePositionWeight,
		ReserveCashRatio:        in.ReserveCashRatio,
		MinOrderAmount:          in.MinOrderAmount,
	}
}
